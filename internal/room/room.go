package room

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"twintube/internal/db"
)

type VideoState struct {
	VideoID         string  `json:"videoId"`
	Title           string  `json:"title"`
	Status          string  `json:"status"`
	CurrentTime     float64 `json:"currentTime"`
	ServerTimestamp int64   `json:"serverTimestamp"`
}

type UserSummary struct {
	ID        string `json:"id"`
	UserID    string `json:"userId,omitempty"`
	Nickname  string `json:"nickname"`
	IsHost    bool   `json:"isHost"`
	IsGuest   bool   `json:"isGuest"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type WSMessage struct {
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
}

type Client struct {
	ID        string
	UserID    string
	RoomID    string
	Nickname  string
	IsHost    bool
	IsGuest   bool
	AvatarURL string
	Conn      *websocket.Conn
	Send      chan WSMessage
	Room      *Room
}

type Room struct {
	ID           string
	Name         string
	IsPrivate    bool
	OwnerID      string
	PasswordHash string
	ExpiresAt    *time.Time
	HostID       string
	State        VideoState
	Clients      map[string]*Client
	Playlist     []db.PlaylistItem
	Register     chan *Client
	Unregister   chan *Client
	Broadcast    chan WSMessage
	mu           sync.RWMutex
}

func (rm *RoomManager) RemoveRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.rooms, roomID)
}

func (r *Room) RequiresPassword() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.PasswordHash != ""
}

func (r *Room) PasswordHashValue() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.PasswordHash
}

func (r *Room) OwnerIDValue() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.OwnerID
}

func (r *Room) IsExpired() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.ExpiresAt == nil {
		return false
	}
	return !r.ExpiresAt.After(time.Now())
}

type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

var Manager = &RoomManager{
	rooms: make(map[string]*Room),
}

func (rm *RoomManager) GetOrCreateRoom(roomID string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		room = &Room{
			ID:     roomID,
			Name:   "Room " + roomID,
			HostID: "",
			State: VideoState{
				VideoID:         "dQw4w9WgXcQ",
				Title:           "Rick Astley - Never Gonna Give You Up",
				Status:          "PAUSED",
				CurrentTime:     0.0,
				ServerTimestamp: time.Now().UnixNano() / 1e6,
			},
			Clients:    make(map[string]*Client),
			Playlist:   make([]db.PlaylistItem, 0),
			Register:   make(chan *Client),
			Unregister: make(chan *Client),
			Broadcast:  make(chan WSMessage, 64),
		}

		if db.Database != nil {
			if rec, err := db.Database.GetRoomByID(roomID); err == nil && rec != nil {
				if rec.Name != "" {
					room.Name = rec.Name
				}
				room.OwnerID = rec.OwnerID
				room.PasswordHash = rec.PasswordHash
				room.IsPrivate = rec.IsPrivate || rec.PasswordHash != ""
				room.ExpiresAt = rec.ExpiresAt
				if rec.CurrentVideoID != "" {
					room.State.VideoID = rec.CurrentVideoID
				}
				if rec.CurrentStatus != "" {
					room.State.Status = rec.CurrentStatus
				}
				room.State.CurrentTime = rec.CurrentTime
			}
			if items, err := db.Database.LoadPlaylist(roomID); err == nil && len(items) > 0 {
				room.Playlist = items
			}
		}

		rm.rooms[roomID] = room
		go room.Run()
		log.Printf("[ROOM] Created new room %s", roomID)
	}

	return room
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.Register:
			r.mu.Lock()
			r.Clients[client.ID] = client

			if r.HostID == "" {
				r.HostID = client.ID
				client.IsHost = true
			}
			r.mu.Unlock()

			log.Printf("[ROOM %s] Client joined: %s (Guest: %v, Host: %v)", r.ID, client.Nickname, client.IsGuest, client.IsHost)

			userType := ""
			if client.IsGuest {
				userType = " (Guest)"
			}
			// deliver() directly — never send on Broadcast from inside Run (deadlock)
			r.BroadcastSystemAlert(client.Nickname + userType + " joined the room.")
			r.BroadcastUserList()
			r.SendInitState(client)

		case client := <-r.Unregister:
			leftNickname := ""
			newHostAlert := ""

			r.mu.Lock()
			if _, ok := r.Clients[client.ID]; ok {
				delete(r.Clients, client.ID)
				close(client.Send)
				leftNickname = client.Nickname

				log.Printf("[ROOM %s] Client left: %s", r.ID, client.Nickname)

				if client.ID == r.HostID {
					r.HostID = ""
					for _, c := range r.Clients {
						r.HostID = c.ID
						c.IsHost = true
						newHostAlert = c.Nickname + " is now the Host."
						break
					}
				}
			}
			r.mu.Unlock()

			if leftNickname != "" {
				if newHostAlert != "" {
					r.BroadcastSystemAlert(newHostAlert)
				}
				r.BroadcastSystemAlert(leftNickname + " left the room.")
				r.BroadcastUserList()
			}

		case message := <-r.Broadcast:
			r.deliver(message)
		}
	}
}

// deliver fans out a message to all clients without using the Broadcast channel,
// so it is safe to call from inside Run (Register/Unregister) as well as from
// BroadcastSystemAlert / BroadcastUserList.
func (r *Room) deliver(message WSMessage) {
	r.mu.RLock()
	clients := make([]*Client, 0, len(r.Clients))
	for _, client := range r.Clients {
		clients = append(clients, client)
	}
	r.mu.RUnlock()

	var stale []*Client
	for _, client := range clients {
		select {
		case client.Send <- message:
		default:
			stale = append(stale, client)
		}
	}

	if len(stale) == 0 {
		return
	}

	r.mu.Lock()
	for _, client := range stale {
		if _, ok := r.Clients[client.ID]; ok {
			delete(r.Clients, client.ID)
			close(client.Send)
		}
	}
	r.mu.Unlock()
}

func (r *Room) GetCalculatedTime() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.State.Status == "PLAYING" {
		nowMs := time.Now().UnixNano() / 1e6
		elapsedSec := float64(nowMs-r.State.ServerTimestamp) / 1000.0
		return r.State.CurrentTime + elapsedSec
	}
	return r.State.CurrentTime
}

func (r *Room) SnapshotState() VideoState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.State
}

func (r *Room) SendInitState(client *Client) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	calculatedTime := r.State.CurrentTime
	nowMs := time.Now().UnixNano() / 1e6
	if r.State.Status == "PLAYING" {
		calculatedTime += float64(nowMs-r.State.ServerTimestamp) / 1000.0
	}

	playlist := make([]db.PlaylistItem, len(r.Playlist))
	copy(playlist, r.Playlist)

	statePayload := map[string]interface{}{
		"roomId":   r.ID,
		"clientId": client.ID,
		"isHost":   client.IsHost,
		"isGuest":  client.IsGuest,
		"hostId":   r.HostID,
		"video": map[string]interface{}{
			"videoId":         r.State.VideoID,
			"title":           r.State.Title,
			"status":          r.State.Status,
			"currentTime":     calculatedTime,
			"serverTimestamp": nowMs,
		},
		"playlist": playlist,
		"users":    r.getUserListUnsafe(),
	}

	raw, _ := json.Marshal(statePayload)
	select {
	case client.Send <- WSMessage{
		Action:    "INIT_STATE",
		Payload:   raw,
		Timestamp: nowMs,
	}:
	default:
		log.Printf("[ROOM %s] Failed to send INIT_STATE to %s (send buffer full)", r.ID, client.Nickname)
	}

	if db.Database != nil {
		if history, err := db.Database.LoadChatHistory(r.ID, 50); err == nil && len(history) > 0 {
			rawHistory, _ := json.Marshal(history)
			select {
			case client.Send <- WSMessage{
				Action:  "CHAT_HISTORY",
				Payload: rawHistory,
			}:
			default:
			}
		}
	}
}

func (r *Room) BroadcastUserList() {
	r.mu.RLock()
	users := r.getUserListUnsafe()
	r.mu.RUnlock()

	raw, _ := json.Marshal(users)
	r.deliver(WSMessage{
		Action:  "USER_LIST",
		Payload: raw,
	})
}

func (r *Room) getUserListUnsafe() []UserSummary {
	list := make([]UserSummary, 0, len(r.Clients))
	for _, c := range r.Clients {
		list = append(list, UserSummary{
			ID:        c.ID,
			UserID:    c.UserID,
			Nickname:  c.Nickname,
			IsHost:    c.IsHost,
			IsGuest:   c.IsGuest,
			AvatarURL: c.AvatarURL,
		})
	}
	return list
}

func (r *Room) BroadcastSystemAlert(content string) {
	msg := db.ChatMessage{
		Type:      "chat",
		Nickname:  "System",
		Content:   content,
		IsSystem:  true,
		Timestamp: time.Now().Format("15:04"),
	}

	if db.Database != nil {
		_ = db.Database.SaveChatMessage(r.ID, "", "System", content, true)
	}

	raw, _ := json.Marshal(msg)
	r.deliver(WSMessage{
		Action:  "CHAT_MESSAGE",
		Payload: raw,
	})
}

func (r *Room) UpdateVideoState(videoId string, status string, currentTime float64, title string) {
	r.mu.Lock()
	nowMs := time.Now().UnixNano() / 1e6
	if videoId != "" {
		r.State.VideoID = videoId
	}
	r.State.Status = status
	r.State.CurrentTime = currentTime
	r.State.ServerTimestamp = nowMs
	if title != "" {
		r.State.Title = title
	}

	stateCopy := r.State

	if db.Database != nil {
		_ = db.Database.SaveRoom(r.ID, r.HostID, r.State.VideoID, r.State.Status, r.State.CurrentTime)
	}
	r.mu.Unlock()

	statePayload := map[string]interface{}{
		"videoId":         stateCopy.VideoID,
		"title":           stateCopy.Title,
		"status":          stateCopy.Status,
		"currentTime":     stateCopy.CurrentTime,
		"serverTimestamp": nowMs,
	}

	raw, _ := json.Marshal(statePayload)
	r.Broadcast <- WSMessage{
		Action:    "STATE_UPDATE",
		Payload:   raw,
		Timestamp: nowMs,
	}
}

func (r *Room) AppendPlaylistItem(item db.PlaylistItem) (db.PlaylistItem, []db.PlaylistItem) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item.Position = len(r.Playlist)
	r.Playlist = append(r.Playlist, item)

	out := make([]db.PlaylistItem, len(r.Playlist))
	copy(out, r.Playlist)
	return item, out
}

func (r *Room) PlayPlaylistItem(itemID string) (db.PlaylistItem, []db.PlaylistItem, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var target db.PlaylistItem
	found := false
	newPlaylist := make([]db.PlaylistItem, 0, len(r.Playlist))

	for _, item := range r.Playlist {
		if item.ID == itemID {
			target = item
			found = true
			continue
		}
		newPlaylist = append(newPlaylist, item)
	}

	if !found {
		out := make([]db.PlaylistItem, len(r.Playlist))
		copy(out, r.Playlist)
		return db.PlaylistItem{}, out, false
	}

	r.Playlist = newPlaylist
	out := make([]db.PlaylistItem, len(r.Playlist))
	copy(out, r.Playlist)
	return target, out, true
}

func (r *Room) RemovePlaylistItem(itemID string) (removed bool, playlist []db.PlaylistItem) {
	r.mu.Lock()
	defer r.mu.Unlock()

	newPlaylist := make([]db.PlaylistItem, 0, len(r.Playlist))
	for _, item := range r.Playlist {
		if item.ID == itemID {
			removed = true
			continue
		}
		newPlaylist = append(newPlaylist, item)
	}
	r.Playlist = newPlaylist

	playlist = make([]db.PlaylistItem, len(r.Playlist))
	copy(playlist, r.Playlist)
	return removed, playlist
}

func (r *Room) TransferHost(from *Client, targetClientID string) (targetNickname string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !from.IsHost {
		return "", false
	}

	target, exists := r.Clients[targetClientID]
	if !exists {
		return "", false
	}

	from.IsHost = false
	r.HostID = target.ID
	target.IsHost = true
	return target.Nickname, true
}
