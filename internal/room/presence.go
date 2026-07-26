package room

import (
	"encoding/json"
	"sync"

	"twintube/internal/db"
)

// PresenceHub tracks online authenticated users for cross-room notifications.
type PresenceHub struct {
	mu     sync.RWMutex
	byUser map[string]map[string]*Client
}

var GlobalPresence = &PresenceHub{
	byUser: make(map[string]map[string]*Client),
}

func (h *PresenceHub) Register(c *Client) {
	if c == nil || c.UserID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byUser[c.UserID] == nil {
		h.byUser[c.UserID] = make(map[string]*Client)
	}
	h.byUser[c.UserID][c.ID] = c
}

func (h *PresenceHub) Unregister(c *Client) {
	if c == nil || c.UserID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if bucket, ok := h.byUser[c.UserID]; ok {
		delete(bucket, c.ID)
		if len(bucket) == 0 {
			delete(h.byUser, c.UserID)
		}
	}
}

type CoWatcherRoomPayload struct {
	RoomID         string `json:"roomId"`
	RoomName       string `json:"roomName"`
	HostNickname   string `json:"hostNickname"`
	HostUserID     string `json:"hostUserId"`
	URL            string `json:"url"`
}

// NotifyCoWatchersRoomStarted alerts online peers when someone they watched with opens a room.
func (h *PresenceHub) NotifyCoWatchersRoomStarted(hostUserID, hostNickname, roomID, roomName string) {
	if hostUserID == "" || db.Database == nil {
		return
	}
	peers, err := db.Database.GetCoWatcherUserIDs(hostUserID)
	if err != nil || len(peers) == 0 {
		return
	}

	payload := CoWatcherRoomPayload{
		RoomID:       roomID,
		RoomName:     roomName,
		HostNickname: hostNickname,
		HostUserID:   hostUserID,
		URL:          "/room/" + roomID,
	}
	raw, _ := json.Marshal(payload)

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, peerID := range peers {
		if peerID == hostUserID {
			continue
		}
		for _, client := range h.byUser[peerID] {
			select {
			case client.Send <- WSMessage{Action: "COWATCHER_ROOM", Payload: raw}:
			default:
			}
		}
	}
}

type MentionNotifyPayload struct {
	RoomID   string `json:"roomId"`
	MessageID string `json:"messageId"`
	From     string `json:"from"`
	Content  string `json:"content"`
}

// SendMentionNotify delivers a mention alert to one client.
func SendMentionNotify(client *Client, roomID string, msg db.ChatMessage) {
	if client == nil {
		return
	}
	snippet := msg.Content
	if len(snippet) > 120 {
		snippet = snippet[:120] + "…"
	}
	payload := MentionNotifyPayload{
		RoomID:    roomID,
		MessageID: msg.ID,
		From:      msg.Nickname,
		Content:   snippet,
	}
	raw, _ := json.Marshal(payload)
	select {
	case client.Send <- WSMessage{Action: "MENTION_NOTIFY", Payload: raw}:
	default:
	}
}
