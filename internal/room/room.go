package room

import (
	"encoding/json"
	"log"
	"strings"
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
	Platform        string  `json:"platform,omitempty"`
	MediaKind       string  `json:"mediaKind,omitempty"`
	SourceURL       string  `json:"sourceUrl,omitempty"`
	Seekable        bool    `json:"seekable"`
}

type UserSummary struct {
	ID             string `json:"id"`
	UserID         string `json:"userId,omitempty"`
	Nickname       string `json:"nickname"`
	IsHost         bool   `json:"isHost"`
	IsCohost       bool   `json:"isCohost"`
	IsGuest        bool   `json:"isGuest"`
	AvatarURL      string `json:"avatarUrl,omitempty"`
	LocalFileReady bool   `json:"localFileReady"`
	Role           string `json:"role"`
	VoiceJoined    bool   `json:"voiceJoined"`
	VoiceMuted     bool   `json:"voiceMuted"`
	VoiceSpeaking  bool   `json:"voiceSpeaking"`
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
	IsCohost  bool
	IsGuest   bool
	AvatarURL string
	RemoteIP  string
	JoinedAt  time.Time
	VoiceJoined   bool
	VoiceMuted    bool
	VoiceSpeaking bool
	Conn      *websocket.Conn
	Send      chan WSMessage
	Room      *Room
	// LocalReadyID is the local:<hash> video ID this client has loaded on
	// their device. Compared against Room.State.VideoID when building the
	// user list so readiness resets automatically when the video changes.
	LocalReadyID string
}

type Room struct {
	ID            string
	Name          string
	IsPrivate     bool
	OwnerID       string
	PasswordHash  string
	ExpiresAt     *time.Time
	HostID        string
	State         VideoState
	Clients       map[string]*Client
	Playlist      []db.PlaylistItem
	Moments       []Moment
	Permissions   RoomPermissions
	SkipVotes     map[string]bool
	SkipVideoID   string
	CohostUserIDs map[string]bool
	Register      chan *Client
	Unregister    chan *Client
	Broadcast     chan WSMessage
	mu            sync.RWMutex
}

func (rm *RoomManager) RemoveRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.rooms, roomID)
}

func (rm *RoomManager) GetRoom(roomID string) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.rooms[roomID]
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

func (r *Room) IsPersistent() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.OwnerID != ""
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

func (rm *RoomManager) newRoomShell(roomID, name string) *Room {
	starter := PickRandomStarter()
	return &Room{
		ID:     roomID,
		Name:   name,
		HostID: "",
		State: VideoState{
			VideoID:         starter.VideoID,
			Title:           starter.Title,
			Status:          "PAUSED",
			CurrentTime:     0.0,
			ServerTimestamp: time.Now().UnixNano() / 1e6,
			Platform:        "youtube",
			MediaKind:       "vod",
			Seekable:        true,
		},
		Clients:       make(map[string]*Client),
		Playlist:      make([]db.PlaylistItem, 0),
		Moments:       make([]Moment, 0),
		Permissions:   DefaultRoomPermissions(),
		SkipVotes:     make(map[string]bool),
		CohostUserIDs: make(map[string]bool),
		Register:      make(chan *Client),
		Unregister:    make(chan *Client),
		Broadcast:     make(chan WSMessage, 64),
	}
}

// CreateGuestRoom starts an in-memory-only room that is never persisted to the database.
func (rm *RoomManager) CreateGuestRoom(roomID, name, passwordHash string, isPrivate bool) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, exists := rm.rooms[roomID]; exists {
		room.mu.Lock()
		room.Name = name
		room.PasswordHash = passwordHash
		room.IsPrivate = isPrivate
		room.OwnerID = ""
		room.mu.Unlock()
		return room
	}

	if db.Database != nil {
		_ = db.Database.DeleteEphemeralRoomData(roomID)
	}

	if name == "" {
		name = "Room " + roomID
	}

	room := rm.newRoomShell(roomID, name)
	room.OwnerID = ""
	room.PasswordHash = passwordHash
	room.IsPrivate = isPrivate
	room.ExpiresAt = nil

	rm.rooms[roomID] = room
	go room.Run()
	log.Printf("[ROOM] Created ephemeral guest room %s", roomID)
	return room
}

func (rm *RoomManager) GetOrCreateRoom(roomID string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		room = rm.newRoomShell(roomID, "Room "+roomID)

		if db.Database != nil {
			if rec, err := db.Database.GetRoomByID(roomID); err == nil && rec != nil {
				if rec.OwnerID != "" {
					if rec.Name != "" {
						room.Name = rec.Name
					}
					room.OwnerID = rec.OwnerID
					room.PasswordHash = rec.PasswordHash
					room.IsPrivate = rec.IsPrivate || rec.PasswordHash != ""
					room.ExpiresAt = rec.ExpiresAt
					prevShellID := room.State.VideoID
					room.State.VideoID = rec.CurrentVideoID
					if rec.CurrentVideoID == "" {
						room.State.Title = ""
						room.State.Platform = ""
						room.State.SourceURL = ""
						room.State.MediaKind = "vod"
						room.State.Seekable = true
					} else if title, ok := StarterTitle(rec.CurrentVideoID); ok {
						room.State.Title = title
						room.State.Platform = "youtube"
						room.State.MediaKind = "vod"
						room.State.Seekable = true
					} else if rec.CurrentVideoID != prevShellID {
						// DB has no title column — avoid showing the wrong starter name
						room.State.Title = "Now playing"
					}
					if rec.CurrentStatus != "" {
						room.State.Status = rec.CurrentStatus
					} else if rec.CurrentVideoID == "" {
						room.State.Status = "PAUSED"
					}
					room.State.CurrentTime = rec.CurrentTime
					if items, err := db.Database.LoadPlaylist(roomID); err == nil && len(items) > 0 {
						room.Playlist = items
					}
					if moments, err := db.Database.LoadMoments(roomID); err == nil && len(moments) > 0 {
						room.Moments = make([]Moment, 0, len(moments))
						for _, row := range moments {
							room.Moments = append(room.Moments, Moment{
								ID:              row.ID,
								RoomID:          row.RoomID,
								CreatorUserID:   row.CreatorUserID,
								CreatorNickname: row.CreatorNickname,
								VideoID:         row.VideoID,
								Platform:        row.Platform,
								AtSeconds:       row.AtSeconds,
								Label:           row.Label,
								Status:          row.Status,
								CreatedAt:       row.CreatedAt,
							})
						}
					}
				} else {
					_ = db.Database.DeleteEphemeralRoomData(roomID)
				}
			}
		}

		rm.rooms[roomID] = room
		go room.Run()
		log.Printf("[ROOM] Created/Loaded Room %s", roomID)
	}

	return room
}

// OpenExistingRoom returns a room only if it already lives in memory or as a
// persisted owned room. It never creates phantom rooms from typed URL codes.
func (rm *RoomManager) OpenExistingRoom(roomID string) *Room {
	if room := rm.GetRoom(roomID); room != nil {
		return room
	}

	meta := rm.LookupJoinMeta(roomID)
	if !meta.Found || meta.Expired {
		return nil
	}

	return rm.GetOrCreateRoom(roomID)
}

// SendTo delivers a message to one client without evicting them on buffer pressure.
// Returns false if the client is missing or their send buffer is full.
func (r *Room) SendTo(clientID string, message WSMessage) bool {
	r.mu.RLock()
	client := r.Clients[clientID]
	r.mu.RUnlock()
	if client == nil {
		return false
	}
	select {
	case client.Send <- message:
		return true
	default:
		return false
	}
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.Register:
			r.mu.Lock()
			if client.JoinedAt.IsZero() {
				client.JoinedAt = time.Now()
			}
			client.IsHost = false
			r.Clients[client.ID] = client
			r.restoreCohostOnJoinLocked(client)

			hostAlert := ""
			ownerReclaim := r.OwnerID != "" && client.UserID != "" && client.UserID == r.OwnerID
			if ownerReclaim {
				prevHostID := r.HostID
				r.assignHostLocked(client)
				if prevHostID != "" && prevHostID != client.ID {
					hostAlert = client.Nickname + " reclaimed Host as the room owner."
				}
			} else {
				r.reconcileHostLocked()
				if r.HostID == "" {
					r.assignHostLocked(client)
				}
			}
			r.mu.Unlock()

			log.Printf("[ROOM %s] Client joined: %s (Guest: %v, Host: %v, Cohost: %v)", r.ID, client.Nickname, client.IsGuest, client.IsHost, client.IsCohost)

			userType := ""
			if client.IsGuest {
				userType = " (Guest)"
			}
			// deliver() directly — never send on Broadcast from inside Run (deadlock)
			r.BroadcastSystemAlert(client.Nickname + userType + " joined the room.")
			if hostAlert != "" {
				r.BroadcastSystemAlert(hostAlert)
			}
			r.BroadcastUserList()
			r.BroadcastRoomMeta()
			r.SendInitState(client)

		case client := <-r.Unregister:
			leftNickname := ""
			newHostAlert := ""
			destroyEphemeral := false

			r.mu.Lock()
			if _, ok := r.Clients[client.ID]; ok {
				delete(r.Clients, client.ID)
				close(client.Send)
				leftNickname = client.Nickname
				if r.SkipVotes != nil {
					delete(r.SkipVotes, client.ID)
				}
				client.VoiceJoined = false
				client.VoiceSpeaking = false
				client.VoiceMuted = true

				log.Printf("[ROOM %s] Client left: %s", r.ID, client.Nickname)

				wasHost := client.ID == r.HostID || client.IsHost
				client.IsHost = false
				if wasHost {
					r.HostID = ""
					if next := r.reconcileHostLocked(); next != nil {
						newHostAlert = next.Nickname + " is now the Host."
					}
				} else {
					r.reconcileHostLocked()
				}

				if len(r.Clients) == 0 && r.OwnerID == "" {
					destroyEphemeral = true
				}
			}
			r.mu.Unlock()

			if leftNickname != "" {
				if newHostAlert != "" {
					r.BroadcastSystemAlert(newHostAlert)
				}
				r.BroadcastSystemAlert(leftNickname + " left the room.")
				r.BroadcastUserList()
				r.BroadcastRoomMeta()
			}

			if destroyEphemeral {
				if db.Database != nil {
					_ = db.Database.DeleteEphemeralRoomData(r.ID)
				}
				Manager.RemoveRoom(r.ID)
				log.Printf("[ROOM] Destroyed ephemeral room %s", r.ID)
				return
			}

		case message := <-r.Broadcast:
			r.deliver(message)
		}
	}
}

// deliver fans out a message to all clients without using the Broadcast channel,
// so it is safe to call from inside Run (Register/Unregister) as well as from
// BroadcastSystemAlert / BroadcastUserList. Full buffers drop the message only —
// they do not kick the client (signaling/voice bursts must not evict viewers).
func (r *Room) deliver(message WSMessage) {
	r.mu.RLock()
	clients := make([]*Client, 0, len(r.Clients))
	for _, client := range r.Clients {
		clients = append(clients, client)
	}
	r.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.Send <- message:
		default:
			// Drop for this client; keep them connected.
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

func (r *Room) SnapshotState() VideoState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.State
}

func (r *Room) SendInitState(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	calculatedTime := r.State.CurrentTime
	nowMs := time.Now().UnixNano() / 1e6
	if r.State.Status == "PLAYING" {
		calculatedTime += float64(nowMs-r.State.ServerTimestamp) / 1000.0
	}

	playlist := make([]db.PlaylistItem, len(r.Playlist))
	copy(playlist, r.Playlist)

	meta := r.roomMetaForClient(client)
	approved := make([]Moment, 0)
	pending := make([]Moment, 0)
	for _, m := range r.Moments {
		if m.Status == "approved" {
			approved = append(approved, m)
		} else if m.Status == "pending" && client.isController() {
			pending = append(pending, m)
		}
	}

	statePayload := map[string]interface{}{
		"roomId":   r.ID,
		"clientId": client.ID,
		"isHost":   client.IsHost,
		"isCohost": client.IsCohost,
		"isGuest":  client.IsGuest,
		"hostId":   r.HostID,
		"role":     client.RoleName(),
		"canControlPlayback": meta["canControlPlayback"],
		"canModerateQueue":   meta["canModerateQueue"],
		"canAddQueue":        meta["canAddQueue"],
		"canSubmitMoment":    meta["canSubmitMoment"],
		"canJumpMoment":      meta["canJumpMoment"],
		"permissions":        meta["permissions"],
		"skipVotes":          meta["skipVotes"],
		"skipNeeded":         meta["skipNeeded"],
		"hasSkipVoted":       meta["hasSkipVoted"],
		"video": map[string]interface{}{
			"videoId":         r.State.VideoID,
			"title":           r.State.Title,
			"status":          r.State.Status,
			"currentTime":     calculatedTime,
			"serverTimestamp": nowMs,
			"platform":        r.State.Platform,
			"mediaKind":       r.State.MediaKind,
			"sourceUrl":       r.State.SourceURL,
			"seekable":        r.State.Seekable,
		},
		"playlist": playlist,
		"users":    r.getUserListUnsafe(),
		"moments":  approved,
		"pendingMoments": pending,
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

	if db.Database != nil && r.OwnerID != "" {
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
			ID:             c.ID,
			UserID:         c.UserID,
			Nickname:       c.Nickname,
			IsHost:         c.IsHost,
			IsCohost:       c.IsCohost,
			IsGuest:        c.IsGuest,
			AvatarURL:      c.AvatarURL,
			LocalFileReady: c.LocalReadyID != "" && c.LocalReadyID == r.State.VideoID,
			Role:           c.RoleName(),
			VoiceJoined:    c.VoiceJoined,
			VoiceMuted:     c.VoiceMuted,
			VoiceSpeaking:  c.VoiceSpeaking,
		})
	}
	return list
}

// SetLocalFileReady records whether a client has the given local video file
// loaded on their device, then broadcasts the updated user list.
func (r *Room) SetLocalFileReady(client *Client, videoID string, ready bool) {
	r.mu.Lock()
	if ready {
		client.LocalReadyID = videoID
	} else if videoID == "" || client.LocalReadyID == videoID {
		client.LocalReadyID = ""
	}
	r.mu.Unlock()
	r.BroadcastUserList()
}

func (r *Room) BroadcastSystemAlert(content string) {
	msg := db.ChatMessage{
		Type:      "chat",
		Nickname:  "System",
		Content:   content,
		IsSystem:  true,
		Timestamp: time.Now().Format("15:04"),
	}

	if db.Database != nil && r.IsPersistent() {
		_ = db.Database.SaveChatMessage(r.ID, "", "System", content, true)
	}

	raw, _ := json.Marshal(msg)
	r.deliver(WSMessage{
		Action:  "CHAT_MESSAGE",
		Payload: raw,
	})
}

type MediaMeta struct {
	Platform  string
	MediaKind string
	SourceURL string
	Seekable  bool
}

func (r *Room) UpdateVideoState(videoId string, status string, currentTime float64, title string, meta MediaMeta) {
	r.mu.Lock()
	nowMs := time.Now().UnixNano() / 1e6
	prevID := r.State.VideoID
	if videoId != "" {
		r.State.VideoID = videoId
	}
	r.State.Status = status
	r.State.CurrentTime = currentTime
	r.State.ServerTimestamp = nowMs
	if title != "" {
		r.State.Title = title
	}
	if meta.Platform != "" {
		r.State.Platform = meta.Platform
		r.State.MediaKind = meta.MediaKind
		if r.State.MediaKind == "" {
			r.State.MediaKind = "vod"
		}
		r.State.SourceURL = meta.SourceURL
		r.State.Seekable = meta.Seekable || r.State.MediaKind != "live"
		if meta.MediaKind == "live" {
			r.State.Seekable = false
		}
	} else if videoId != "" && videoId != prevID {
		if strings.HasPrefix(videoId, "local:") {
			r.State.Platform = "local"
		} else if strings.HasPrefix(videoId, "vod:") || strings.HasPrefix(videoId, "channel:") {
			r.State.Platform = "twitch"
			if strings.HasPrefix(videoId, "channel:") {
				r.State.MediaKind = "live"
				r.State.Seekable = false
			}
		} else if r.State.Platform == "" {
			r.State.Platform = "youtube"
		}
		if r.State.MediaKind == "" {
			r.State.MediaKind = "vod"
		}
		if r.State.MediaKind != "live" {
			r.State.Seekable = true
		}
	}

	stateCopy := r.State
	r.resetSkipVotesLocked()

	if db.Database != nil && r.OwnerID != "" {
		_ = db.Database.SaveRoom(r.ID, r.HostID, r.State.VideoID, r.State.Status, r.State.CurrentTime)
	}
	r.mu.Unlock()

	statePayload := map[string]interface{}{
		"videoId":         stateCopy.VideoID,
		"title":           stateCopy.Title,
		"status":          stateCopy.Status,
		"currentTime":     stateCopy.CurrentTime,
		"serverTimestamp": nowMs,
		"platform":        stateCopy.Platform,
		"mediaKind":       stateCopy.MediaKind,
		"sourceUrl":       stateCopy.SourceURL,
		"seekable":        stateCopy.Seekable,
	}

	raw, _ := json.Marshal(statePayload)
	r.Broadcast <- WSMessage{
		Action:    "STATE_UPDATE",
		Payload:   raw,
		Timestamp: nowMs,
	}
	r.BroadcastRoomMeta()
}

// ClearCurrentVideo empties the player for everyone (host-only). Returns false if denied.
func (r *Room) ClearCurrentVideo(from *Client) bool {
	if from == nil || !from.IsHost {
		return false
	}

	r.mu.Lock()
	nowMs := time.Now().UnixNano() / 1e6
	r.State.VideoID = ""
	r.State.Title = ""
	r.State.Status = "PAUSED"
	r.State.CurrentTime = 0
	r.State.ServerTimestamp = nowMs
	r.State.Platform = ""
	r.State.MediaKind = "vod"
	r.State.SourceURL = ""
	r.State.Seekable = true
	stateCopy := r.State
	r.resetSkipVotesLocked()
	if db.Database != nil && r.OwnerID != "" {
		_ = db.Database.SaveRoom(r.ID, r.HostID, "", "PAUSED", 0)
	}
	r.mu.Unlock()

	statePayload := map[string]interface{}{
		"videoId":         stateCopy.VideoID,
		"title":           stateCopy.Title,
		"status":          stateCopy.Status,
		"currentTime":     stateCopy.CurrentTime,
		"serverTimestamp": nowMs,
		"platform":        stateCopy.Platform,
		"mediaKind":       stateCopy.MediaKind,
		"sourceUrl":       stateCopy.SourceURL,
		"seekable":        stateCopy.Seekable,
		"forceReload":     true,
	}
	raw, _ := json.Marshal(statePayload)
	r.Broadcast <- WSMessage{
		Action:    "STATE_UPDATE",
		Payload:   raw,
		Timestamp: nowMs,
	}
	r.BroadcastRoomMeta()
	return true
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

func (r *Room) PlayNextPlaylistItem() (db.PlaylistItem, []db.PlaylistItem, bool) {
	r.mu.RLock()
	if len(r.Playlist) == 0 {
		r.mu.RUnlock()
		out := make([]db.PlaylistItem, 0)
		return db.PlaylistItem{}, out, false
	}
	nextID := r.Playlist[0].ID
	r.mu.RUnlock()
	return r.PlayPlaylistItem(nextID)
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
	r.renumberPlaylistLocked()
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
	r.renumberPlaylistLocked()

	playlist = make([]db.PlaylistItem, len(r.Playlist))
	copy(playlist, r.Playlist)
	return removed, playlist
}

func (r *Room) TransferHost(from *Client, targetClientID string) (targetNickname string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if from == nil || !from.IsHost || from.ID != r.HostID {
		return "", false
	}

	target, exists := r.Clients[targetClientID]
	if !exists || target.ID == from.ID {
		return "", false
	}

	r.assignHostLocked(target)
	return target.Nickname, true
}
