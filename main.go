package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow cross-origin requests for API / VPS flexibility
	},
}

func main() {
	port := flag.Int("port", 8080, "Port for the HTTP server")
	dbPath := flag.String("db", "twintube.db", "SQLite database file path fallback")
	flag.Parse()

	// Environment variable port override for VPS/Docker
	if envPort := os.Getenv("PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", port)
	}

	initJWTSecret()

	// Initialize Database (PostgreSQL or SQLite fallback)
	if _, err := InitDB(*dbPath); err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}

	// Serve Static Files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", securityHeadersMiddleware(http.StripPrefix("/static/", fs)))

	// API Auth Routes
	http.HandleFunc("/api/auth/register", handleRegister)
	http.HandleFunc("/api/auth/login", handleLogin)
	http.HandleFunc("/api/auth/me", handleGetMe)

	// API Room & Metadata Routes
	http.HandleFunc("/api/youtube/info", handleYouTubeInfo)
	http.HandleFunc("/api/room/create", handleCreateRoom)

	// WebSocket & SPA Routes
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/", serveIndexOrRoom)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[SERVER] TwinTube production ready at http://localhost:%d", *port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}

// securityHeadersMiddleware injects standard security headers
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

// serveIndexOrRoom serves index.html for root and /room/{id} endpoints
func serveIndexOrRoom(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" || strings.HasPrefix(path, "/room/") {
		http.ServeFile(w, r, filepath.Join(".", "static", "index.html"))
		return
	}

	http.ServeFile(w, r, filepath.Join(".", "static", path))
}

// handleCreateRoom creates a new unique room code
func handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	roomCode := GenerateRoomCode()
	Manager.GetOrCreateRoom(roomCode)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"roomCode": roomCode,
		"url":      "/room/" + roomCode,
	})
}

// handleYouTubeInfo fetches video metadata via YouTube oEmbed
func handleYouTubeInfo(w http.ResponseWriter, r *http.Request) {
	videoInput := r.URL.Query().Get("v")
	if videoInput == "" {
		http.Error(w, `{"error":"Missing video parameter"}`, http.StatusBadRequest)
		return
	}

	videoID, err := ExtractYouTubeID(videoInput)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	meta, err := FetchYouTubeMetadata(videoID)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch video metadata"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"videoId":      videoID,
		"title":        meta.Title,
		"author":       meta.AuthorName,
		"thumbnailUrl": meta.ThumbnailURL,
	})
}

// handleWebSocket handles WebSocket connections
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	client := &Client{
		ID:      GenerateRandomID(8),
		IsGuest: true,
		Conn:    conn,
		Send:    make(chan WSMessage, 256),
	}

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		if c.Room != nil {
			c.Room.Unregister <- c
		}
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(4096)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error for client %s: %v", c.ID, err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("[WS] Error unmarshalling message: %v", err)
			continue
		}

		c.handleIncomingAction(msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			jsonBytes, err := json.Marshal(message)
			if err == nil {
				w.Write(jsonBytes)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleIncomingAction(msg WSMessage) {
	switch msg.Action {
	case "JOIN_ROOM":
		var payload struct {
			RoomID   string `json:"roomId"`
			Nickname string `json:"nickname"`
			Token    string `json:"token"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		if payload.RoomID == "" {
			payload.RoomID = "default"
		}

		// Authenticate via JWT Token if present
		c.IsGuest = true
		if payload.Token != "" {
			claims, err := ParseJWTToken(payload.Token)
			if err == nil && claims != nil && Database != nil {
				user, err := Database.GetUserByID(claims.UserID)
				if err == nil && user != nil {
					c.UserID = user.ID
					c.Nickname = user.Username
					c.AvatarURL = user.AvatarURL
					c.IsGuest = false
				}
			}
		}

		if c.IsGuest {
			if payload.Nickname != "" {
				c.Nickname = payload.Nickname
			} else {
				c.Nickname = "Guest_" + c.ID[:4]
			}
		}

		c.RoomID = payload.RoomID
		c.Room = Manager.GetOrCreateRoom(payload.RoomID)
		c.Room.Register <- c

	case "STATE_CHANGE":
		if c.Room == nil {
			return
		}
		var payload struct {
			VideoID     string  `json:"videoId"`
			Title       string  `json:"title"`
			Status      string  `json:"status"`
			CurrentTime float64 `json:"currentTime"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		c.Room.UpdateVideoState(payload.VideoID, payload.Status, payload.CurrentTime, payload.Title)

		if payload.Status == "PLAYING" {
			c.Room.BroadcastSystemAlert(c.Nickname + " resumed playback.")
		} else if payload.Status == "PAUSED" {
			c.Room.BroadcastSystemAlert(fmt.Sprintf("%s paused the video at %.1fs.", c.Nickname, payload.CurrentTime))
		}

	case "CHAT_MESSAGE":
		if c.Room == nil {
			return
		}
		var payload struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil || strings.TrimSpace(payload.Content) == "" {
			return
		}

		chatMsg := ChatMessage{
			Type:      "chat",
			Nickname:  c.Nickname,
			Content:   strings.TrimSpace(payload.Content),
			IsSystem:  false,
			Timestamp: time.Now().Format("15:04"),
		}

		if Database != nil {
			_ = Database.SaveChatMessage(c.RoomID, c.UserID, c.Nickname, chatMsg.Content, false)
		}

		raw, _ := json.Marshal(chatMsg)
		c.Room.Broadcast <- WSMessage{
			Action:  "CHAT_MESSAGE",
			Payload: raw,
		}

	case "ADD_QUEUE":
		if c.Room == nil {
			return
		}
		var payload struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		videoID, err := ExtractYouTubeID(payload.URL)
		if err != nil {
			return
		}

		meta, _ := FetchYouTubeMetadata(videoID)

		c.Room.mu.Lock()
		item := PlaylistItem{
			ID:           GenerateRandomID(6),
			RoomID:       c.RoomID,
			VideoID:      videoID,
			Title:        meta.Title,
			Author:       meta.AuthorName,
			ThumbnailURL: meta.ThumbnailURL,
			Position:     len(c.Room.Playlist),
			AddedBy:      c.Nickname,
		}
		c.Room.Playlist = append(c.Room.Playlist, item)
		if Database != nil {
			_ = Database.SavePlaylistItem(item)
		}
		c.Room.mu.Unlock()

		c.Room.BroadcastSystemAlert(fmt.Sprintf("%s added '%s' to the queue.", c.Nickname, meta.Title))

		c.Room.mu.RLock()
		raw, _ := json.Marshal(c.Room.Playlist)
		c.Room.mu.RUnlock()

		c.Room.Broadcast <- WSMessage{
			Action:  "QUEUE_UPDATE",
			Payload: raw,
		}

	case "PLAY_QUEUE_ITEM":
		if c.Room == nil {
			return
		}
		var payload struct {
			ItemID string `json:"itemId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		c.Room.mu.Lock()
		var targetItem *PlaylistItem
		var newPlaylist []PlaylistItem

		for _, item := range c.Room.Playlist {
			if item.ID == payload.ItemID {
				targetItem = &item
			} else {
				newPlaylist = append(newPlaylist, item)
			}
		}

		if targetItem != nil {
			c.Room.Playlist = newPlaylist
			if Database != nil {
				_ = Database.DeletePlaylistItem(targetItem.ID)
			}
		}
		c.Room.mu.Unlock()

		if targetItem != nil {
			c.Room.UpdateVideoState(targetItem.VideoID, "PLAYING", 0.0, targetItem.Title)
			c.Room.BroadcastSystemAlert(fmt.Sprintf("Now playing: '%s'", targetItem.Title))

			c.Room.mu.RLock()
			raw, _ := json.Marshal(c.Room.Playlist)
			c.Room.mu.RUnlock()

			c.Room.Broadcast <- WSMessage{
				Action:  "QUEUE_UPDATE",
				Payload: raw,
			}
		}

	case "REMOVE_QUEUE_ITEM":
		if c.Room == nil {
			return
		}
		var payload struct {
			ItemID string `json:"itemId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		c.Room.mu.Lock()
		var newPlaylist []PlaylistItem
		for _, item := range c.Room.Playlist {
			if item.ID != payload.ItemID {
				newPlaylist = append(newPlaylist, item)
			} else if Database != nil {
				_ = Database.DeletePlaylistItem(item.ID)
			}
		}
		c.Room.Playlist = newPlaylist
		c.Room.mu.Unlock()

		c.Room.mu.RLock()
		raw, _ := json.Marshal(c.Room.Playlist)
		c.Room.mu.RUnlock()

		c.Room.Broadcast <- WSMessage{
			Action:  "QUEUE_UPDATE",
			Payload: raw,
		}

	case "TRANSFER_HOST":
		if c.Room == nil || !c.IsHost {
			return
		}
		var payload struct {
			TargetClientID string `json:"targetId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		c.Room.mu.Lock()
		if targetClient, ok := c.Room.Clients[payload.TargetClientID]; ok {
			c.IsHost = false
			c.Room.HostID = targetClient.ID
			targetClient.IsHost = true
			c.Room.BroadcastSystemAlert(fmt.Sprintf("%s transferred Host status to %s.", c.Nickname, targetClient.Nickname))
		}
		c.Room.mu.Unlock()

		c.Room.BroadcastUserList()

	case "SYNC_REQUEST":
		if c.Room == nil {
			return
		}
		calculatedTime := c.Room.GetCalculatedTime()
		c.Room.mu.RLock()
		statePayload := map[string]interface{}{
			"videoId":         c.Room.State.VideoID,
			"title":           c.Room.State.Title,
			"status":          c.Room.State.Status,
			"currentTime":     calculatedTime,
			"serverTimestamp": time.Now().UnixNano() / 1e6,
		}
		c.Room.mu.RUnlock()

		raw, _ := json.Marshal(statePayload)
		c.Send <- WSMessage{
			Action:    "STATE_UPDATE",
			Payload:   raw,
			Timestamp: time.Now().UnixNano() / 1e6,
		}
	}
}
