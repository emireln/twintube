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
	ID         string
	Name       string
	IsPrivate  bool
	HostID     string
	State      VideoState
	Clients    map[string]*Client
	Playlist   []db.PlaylistItem
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan WSMessage
	mu         sync.RWMutex
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
			Broadcast:  make(chan WSMessage),
		}

		if db.Database != nil {
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
			r.BroadcastSystemAlert(client.Nickname + userType + " joined the room.")
			r.BroadcastUserList()
			r.SendInitState(client)

		case client := <-r.Unregister:
			r.mu.Lock()
			if _, ok := r.Clients[client.ID]; ok {
				delete(r.Clients, client.ID)
				close(client.Send)

				log.Printf("[ROOM %s] Client left: %s", r.ID, client.Nickname)

				if client.ID == r.HostID {
					r.HostID = ""
					for _, c := range r.Clients {
						r.HostID = c.ID
						c.IsHost = true
						r.BroadcastSystemAlert(c.Nickname + " is now the Host.")
						break
					}
				}
			}
			r.mu.Unlock()

			r.BroadcastSystemAlert(client.Nickname + " left the room.")
			r.BroadcastUserList()

		case message := <-r.Broadcast:
			r.mu.RLock()
			for _, client := range r.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(r.Clients, client.ID)
				}
			}
			r.mu.RUnlock()
		}
	}
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

func (r *Room) SendInitState(client *Client) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	calculatedTime := r.State.CurrentTime
	if r.State.Status == "PLAYING" {
		nowMs := time.Now().UnixNano() / 1e6
		calculatedTime += float64(nowMs-r.State.ServerTimestamp) / 1000.0
	}

	statePayload := map[string]interface{}{
		"roomId":   r.ID,
		"isHost":   client.IsHost,
		"isGuest":  client.IsGuest,
		"hostId":   r.HostID,
		"video": map[string]interface{}{
			"videoId":         r.State.VideoID,
			"title":           r.State.Title,
			"status":          r.State.Status,
			"currentTime":     calculatedTime,
			"serverTimestamp": time.Now().UnixNano() / 1e6,
		},
		"playlist": r.Playlist,
		"users":    r.getUserListUnsafe(),
	}

	raw, _ := json.Marshal(statePayload)
	client.Send <- WSMessage{
		Action:    "INIT_STATE",
		Payload:   raw,
		Timestamp: time.Now().UnixNano() / 1e6,
	}

	if db.Database != nil {
		if history, err := db.Database.LoadChatHistory(r.ID, 50); err == nil && len(history) > 0 {
			rawHistory, _ := json.Marshal(history)
			client.Send <- WSMessage{
				Action:  "CHAT_HISTORY",
				Payload: rawHistory,
			}
		}
	}
}

func (r *Room) BroadcastUserList() {
	r.mu.RLock()
	users := r.getUserListUnsafe()
	r.mu.RUnlock()

	raw, _ := json.Marshal(users)
	r.Broadcast <- WSMessage{
		Action:  "USER_LIST",
		Payload: raw,
	}
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
	r.Broadcast <- WSMessage{
		Action:  "CHAT_MESSAGE",
		Payload: raw,
	}
}

func (r *Room) UpdateVideoState(videoId string, status string, currentTime float64, title string) {
	r.mu.Lock()
	nowMs := time.Now().UnixNano() / 1e6
	r.State.VideoID = videoId
	r.State.Status = status
	r.State.CurrentTime = currentTime
	r.State.ServerTimestamp = nowMs
	if title != "" {
		r.State.Title = title
	}

	if db.Database != nil {
		_ = db.Database.SaveRoom(r.ID, r.HostID, r.State.VideoID, r.State.Status, r.State.CurrentTime)
	}
	r.mu.Unlock()

	statePayload := map[string]interface{}{
		"videoId":         r.State.VideoID,
		"title":           r.State.Title,
		"status":          r.State.Status,
		"currentTime":     r.State.CurrentTime,
		"serverTimestamp": nowMs,
	}

	raw, _ := json.Marshal(statePayload)
	r.Broadcast <- WSMessage{
		Action:    "STATE_UPDATE",
		Payload:   raw,
		Timestamp: nowMs,
	}
}
