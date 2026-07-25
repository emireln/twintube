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

	"twintube/internal/api"
	"twintube/internal/auth"
	"twintube/internal/db"
	"twintube/internal/room"
	"twintube/internal/security"
	"twintube/internal/utils"
	"twintube/internal/version"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkWebSocketOrigin,
}

var joinPasswordLimiter = security.NewRateLimiter(10, 5*time.Minute)

func loadEnvFile(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func main() {
	loadEnvFile(".env")
	port := flag.Int("port", 8080, "Port for the HTTP server")
	dbPath := flag.String("db", "twintube.db", "SQLite database file path fallback")
	flag.Parse()

	if envPort := os.Getenv("PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", port)
	}

	auth.InitJWTSecret()
	room.SetJoinTokenSecret(auth.JWTSecretBytes())
	security.ValidateJWTSecret()

	if _, err := db.InitDB(*dbPath); err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}

	authLimiter := security.NewRateLimiter(20, time.Minute)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", security.Middleware(http.StripPrefix("/static/", fs)))

	apiHandler := api.NewRoomAPIHandler(room.Manager)

	// Auth Routes
	http.HandleFunc("/api/auth/register", security.Wrap(authLimiter.Middleware(auth.HandleRegister)))
	http.HandleFunc("/api/auth/login", security.Wrap(authLimiter.Middleware(auth.HandleLogin)))
	http.HandleFunc("/api/auth/me", security.Wrap(auth.HandleGetMe))
	http.HandleFunc("/api/auth/profile", security.Wrap(auth.HandleUpdateProfile))
	http.HandleFunc("/api/auth/password", security.Wrap(auth.HandleChangePassword))

	// API Room & Metadata Routes
	http.HandleFunc("/api/youtube/info", security.Wrap(handleVideoInfo))
	http.HandleFunc("/api/video/info", security.Wrap(handleVideoInfo))
	http.HandleFunc("/api/room/create", security.Wrap(apiHandler.HandleCreateRoom))
	http.HandleFunc("/api/rooms/mine", security.Wrap(apiHandler.HandleMyRooms))
	http.HandleFunc("/api/room/", security.Wrap(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/info") {
			apiHandler.HandleRoomInfo(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/access") {
			apiHandler.HandleRoomAccess(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	http.HandleFunc("/api/rooms/", security.Wrap(apiHandler.HandleDeleteRoom))
	http.HandleFunc("/api/version", security.Wrap(handleVersion))

	// WebSocket & SPA Routes
	http.HandleFunc("/ws", security.Wrap(handleWebSocket))
	http.HandleFunc("/", security.Wrap(serveLandingOrRoom))

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[SERVER] TwinTube v%s (%s) running at http://localhost:%d", version.Version, version.Commit, *port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}

func checkWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	allowed := allowedOrigins()
	for _, a := range allowed {
		if origin == a {
			return true
		}
	}
	return false
}

func allowedOrigins() []string {
	appDomain := os.Getenv("APP_DOMAIN")
	if appDomain == "" {
		appDomain = "twintube.site"
	}
	return []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"https://" + appDomain,
		"https://www." + appDomain,
		"http://" + appDomain,
	}
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
		"build":   version.Build,
	})
}

func serveLandingOrRoom(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/" {
		http.ServeFile(w, r, filepath.Join(".", "static", "index.html"))
		return
	}
	if strings.HasPrefix(path, "/room/") {
		http.ServeFile(w, r, filepath.Join(".", "static", "room.html"))
		return
	}

	http.ServeFile(w, r, filepath.Join(".", "static", path))
}

func handleVideoInfo(w http.ResponseWriter, r *http.Request) {
	videoInput := r.URL.Query().Get("v")
	if videoInput == "" {
		http.Error(w, `{"error":"Missing video parameter"}`, http.StatusBadRequest)
		return
	}

	info, err := utils.ExtractVideoInfo(videoInput)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	client := &room.Client{
		ID:       utils.GenerateRandomID(8),
		IsGuest:  true,
		RemoteIP: security.ClientIP(r),
		Conn:     conn,
		Send:     make(chan room.WSMessage, 256),
	}

	go clientWritePump(client)
	go clientReadPump(client)
}

func clientReadPump(c *room.Client) {
	defer func() {
		if c.Room != nil {
			c.Room.Unregister <- c
		}
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(32 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg room.WSMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			continue
		}

		handleIncomingAction(c, msg)
	}
}

func clientWritePump(c *room.Client) {
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

func handleIncomingAction(c *room.Client, msg room.WSMessage) {
	switch msg.Action {
	case "JOIN_ROOM":
		if c.Room != nil {
			sendError(c, "Already joined a room.")
			return
		}

		var payload struct {
			RoomID    string `json:"roomId"`
			Nickname  string `json:"nickname"`
			Token     string `json:"token"`
			JoinToken string `json:"joinToken"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		payload.RoomID = strings.TrimSpace(payload.RoomID)
		if payload.RoomID == "" || !utils.IsValidRoomID(payload.RoomID) {
			denyRoomJoin(c, "Invalid room ID.")
			return
		}

		payload.JoinToken = strings.TrimSpace(payload.JoinToken)

		c.IsGuest = true
		if payload.Token != "" {
			claims, err := auth.ParseJWTToken(payload.Token)
			if err == nil && claims != nil && db.Database != nil {
				user, err := db.Database.GetUserByID(claims.UserID)
				if err == nil && user != nil {
					c.UserID = user.ID
					c.Nickname = user.Username
					c.AvatarURL = user.AvatarURL
					c.IsGuest = false
				}
			}
		}

		if c.IsGuest {
			if nick := utils.SanitizeNickname(payload.Nickname); nick != "" {
				c.Nickname = nick
			} else {
				c.Nickname = "Guest_" + c.ID[:4]
			}
		}

		meta := room.Manager.LookupJoinMeta(payload.RoomID)
		if !meta.Found {
			denyRoomJoin(c, "Room not found.")
			return
		}
		if meta.Expired {
			if db.Database != nil && meta.OwnerID != "" {
				_ = db.Database.DeleteOwnedRoom(payload.RoomID, meta.OwnerID)
			}
			room.Manager.RemoveRoom(payload.RoomID)
			denyRoomJoin(c, "This room has expired.")
			return
		}

		if db.Database != nil {
			if rec, err := db.Database.GetRoomByID(payload.RoomID); err == nil && rec != nil && rec.OwnerID == "" {
				_ = db.Database.DeleteEphemeralRoomData(payload.RoomID)
			}
		}

		isOwner := c.UserID != "" && meta.OwnerID != "" && c.UserID == meta.OwnerID
		if meta.RequiresPassword() && !isOwner {
			if !room.ValidateJoinToken(payload.JoinToken, payload.RoomID) {
				failKey := payload.RoomID + "|" + c.RemoteIP
				if !joinPasswordLimiter.Allow(failKey) {
					denyRoomJoin(c, "Too many failed password attempts. Try again later.")
					return
				}
				denyRoomJoin(c, "Room access denied. Verify the password first.")
				return
			}
		}

		target := room.Manager.OpenExistingRoom(payload.RoomID)
		if target == nil {
			denyRoomJoin(c, "Room not found.")
			return
		}
		if target.IsExpired() {
			room.Manager.RemoveRoom(payload.RoomID)
			denyRoomJoin(c, "This room has expired.")
			return
		}

		c.RoomID = payload.RoomID
		c.Room = target
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
			Platform    string  `json:"platform"`
			MediaKind   string  `json:"mediaKind"`
			SourceURL   string  `json:"sourceUrl"`
			Seekable    *bool   `json:"seekable"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		meta := room.MediaMeta{
			Platform:  payload.Platform,
			MediaKind: payload.MediaKind,
			SourceURL: payload.SourceURL,
		}
		if payload.Seekable != nil {
			meta.Seekable = *payload.Seekable
		} else if payload.MediaKind != "live" {
			meta.Seekable = true
		}
		c.Room.UpdateVideoState(payload.VideoID, payload.Status, payload.CurrentTime, payload.Title, meta)

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

		content := strings.TrimSpace(payload.Content)
		if len(content) > 500 {
			content = content[:500]
		}

		chatMsg := db.ChatMessage{
			Type:      "chat",
			Nickname:  c.Nickname,
			Content:   content,
			IsSystem:  false,
			Timestamp: time.Now().Format("15:04"),
		}

		if db.Database != nil && c.Room != nil && c.Room.IsPersistent() {
			_ = db.Database.SaveChatMessage(c.RoomID, c.UserID, c.Nickname, chatMsg.Content, false)
		}

		raw, _ := json.Marshal(chatMsg)
		c.Room.Broadcast <- room.WSMessage{
			Action:  "CHAT_MESSAGE",
			Payload: raw,
		}

	case "ADD_QUEUE":
		if c.Room == nil {
			return
		}
		var payload struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		info, err := utils.ExtractVideoInfo(payload.URL)
		if err != nil {
			log.Printf("[ROOM] ExtractVideoInfo error: %v", err)
			sendError(c, "Could not add video. Check the URL and try again.")
			return
		}

		title := info.Title
		if t := strings.TrimSpace(payload.Title); t != "" && (info.Platform == "local" || info.Platform == "direct" || info.Platform == "hls") {
			if len(t) > 200 {
				t = t[:200]
			}
			title = t
		}

		item := db.PlaylistItem{
			ID:           utils.GenerateRandomID(6),
			RoomID:       c.RoomID,
			VideoID:      info.VideoID,
			Title:        title,
			Author:       info.Author,
			ThumbnailURL: info.ThumbnailURL,
			AddedBy:      c.Nickname,
			Platform:     info.Platform,
			MediaKind:    info.MediaKind,
			SourceURL:    info.SourceURL,
			Seekable:     info.Seekable,
		}
		item, playlist := c.Room.AppendPlaylistItem(item)
		if db.Database != nil && c.Room.IsPersistent() {
			_ = db.Database.SavePlaylistItem(item)
		}

		c.Room.BroadcastSystemAlert(fmt.Sprintf("%s added '%s' to the queue.", c.Nickname, title))

		raw, _ := json.Marshal(playlist)
		c.Room.Broadcast <- room.WSMessage{
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

		targetItem, playlist, found := c.Room.PlayPlaylistItem(payload.ItemID)
		if !found {
			return
		}

		if db.Database != nil && c.Room.IsPersistent() {
			_ = db.Database.DeletePlaylistItem(targetItem.ID)
		}
		c.Room.UpdateVideoState(targetItem.VideoID, "PLAYING", 0.0, targetItem.Title, room.MediaMeta{
			Platform:  targetItem.Platform,
			MediaKind: targetItem.MediaKind,
			SourceURL: targetItem.SourceURL,
			Seekable:  targetItem.Seekable,
		})
		c.Room.BroadcastSystemAlert(fmt.Sprintf("Now playing: '%s'", targetItem.Title))

		raw, _ := json.Marshal(playlist)
		c.Room.Broadcast <- room.WSMessage{
			Action:  "QUEUE_UPDATE",
			Payload: raw,
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

		removed, playlist := c.Room.RemovePlaylistItem(payload.ItemID)
		if removed && db.Database != nil && c.Room.IsPersistent() {
			_ = db.Database.DeletePlaylistItem(payload.ItemID)
		}

		raw, _ := json.Marshal(playlist)
		c.Room.Broadcast <- room.WSMessage{
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

		if targetNick, ok := c.Room.TransferHost(c, payload.TargetClientID); ok {
			c.Room.BroadcastSystemAlert(fmt.Sprintf("%s transferred Host status to %s.", c.Nickname, targetNick))
		}

		c.Room.BroadcastUserList()

	case "VIDEO_REACTION":
		if c.Room == nil {
			return
		}
		var payload struct {
			Reaction string `json:"reaction"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		allowed := map[string]bool{
			"happy.webp":     true,
			"energetic.webp": true,
			"stressed.webp":  true,
			"tired.webp":     true,
		}
		if !allowed[payload.Reaction] {
			return
		}

		raw, _ := json.Marshal(map[string]interface{}{
			"reaction": payload.Reaction,
			"nickname": c.Nickname,
		})
		c.Room.Broadcast <- room.WSMessage{
			Action:  "VIDEO_REACTION",
			Payload: raw,
		}

	case "LOCAL_FILE_STATUS":
		if c.Room == nil {
			return
		}
		var payload struct {
			VideoID string `json:"videoId"`
			Ready   bool   `json:"ready"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		payload.VideoID = strings.TrimSpace(payload.VideoID)
		if payload.Ready && (!strings.HasPrefix(payload.VideoID, "local:") || len(payload.VideoID) > 128) {
			return
		}
		c.Room.SetLocalFileReady(c, payload.VideoID, payload.Ready)

	case "SYNC_REQUEST":
		if c.Room == nil {
			return
		}
		state := c.Room.SnapshotState()
		nowMs := time.Now().UnixNano() / 1e6
		statePayload := map[string]interface{}{
			"videoId":         state.VideoID,
			"title":           state.Title,
			"status":          state.Status,
			"currentTime":     c.Room.GetCalculatedTime(),
			"serverTimestamp": nowMs,
			"forceReload":     true,
			"platform":        state.Platform,
			"mediaKind":       state.MediaKind,
			"sourceUrl":       state.SourceURL,
			"seekable":        state.Seekable,
		}

		raw, _ := json.Marshal(statePayload)
		c.Send <- room.WSMessage{
			Action:    "STATE_UPDATE",
			Payload:   raw,
			Timestamp: nowMs,
		}
	}
}

func sendError(c *room.Client, message string) {
	raw, _ := json.Marshal(map[string]string{"message": message})
	select {
	case c.Send <- room.WSMessage{Action: "ERROR", Payload: raw}:
	default:
	}
}

func denyRoomJoin(c *room.Client, message string) {
	sendError(c, message)
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = c.Conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "room access denied"),
		)
		_ = c.Conn.Close()
	}()
}
