package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"twintube/internal/auth"
	"twintube/internal/db"
	"twintube/internal/room"
	"twintube/internal/utils"

	"golang.org/x/crypto/bcrypt"
)

type RoomAPIHandler struct {
	Manager *room.RoomManager
}

func NewRoomAPIHandler(rm *room.RoomManager) *RoomAPIHandler {
	return &RoomAPIHandler{Manager: rm}
}

type CreateRoomRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type CreateRoomResponse struct {
	RoomCode  string `json:"roomCode"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"isPrivate"`
}

func (h *RoomAPIHandler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req CreateRoomRequest
	if r.Method == http.MethodPost && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	var userID string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := auth.ParseJWTToken(tokenStr); err == nil {
			userID = claims.UserID
		}
	}

	roomID := utils.GenerateRoomCode()
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "Room " + roomID
	}

	var pwdHash string
	if req.Password != "" {
		bytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
		if err == nil {
			pwdHash = string(bytes)
		}
	}

	var expiresAt *time.Time
	if userID != "" {
		exp := time.Now().Add(7 * 24 * time.Hour)
		expiresAt = &exp
	}

	// Only persist owned rooms for signed-in users; guests get ephemeral in-memory rooms.
	if db.Database != nil && userID != "" {
		if err := db.Database.CreateRoomRecord(roomID, req.Name, userID, pwdHash, req.Password != "", expiresAt); err != nil {
			http.Error(w, `{"error":"Failed to create room"}`, http.StatusInternalServerError)
			return
		}
	}

	rmRoom := h.Manager.GetOrCreateRoom(roomID)
	rmRoom.Name = req.Name
	rmRoom.IsPrivate = req.Password != ""
	rmRoom.OwnerID = userID
	rmRoom.PasswordHash = pwdHash
	rmRoom.ExpiresAt = expiresAt

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateRoomResponse{
		RoomCode:  roomID,
		URL:       "/room/" + roomID,
		Name:      req.Name,
		IsPrivate: req.Password != "",
	})
}

func (h *RoomAPIHandler) HandleMyRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.ParseJWTToken(tokenStr)
	if err != nil {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if db.Database == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"rooms": []interface{}{}})
		return
	}

	rooms, err := db.Database.ListRoomsByOwner(claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"Failed to load rooms"}`, http.StatusInternalServerError)
		return
	}

	type RoomItem struct {
		ID          string     `json:"id"`
		Name        string     `json:"name"`
		HasPassword bool       `json:"hasPassword"`
		ExpiresAt   *time.Time `json:"expiresAt"`
		CreatedAt   time.Time  `json:"createdAt"`
	}

	list := make([]RoomItem, 0, len(rooms))
	for _, rm := range rooms {
		list = append(list, RoomItem{
			ID:          rm.ID,
			Name:        rm.Name,
			HasPassword: rm.PasswordHash != "",
			ExpiresAt:   rm.ExpiresAt,
			CreatedAt:   rm.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"rooms": list})
}

func (h *RoomAPIHandler) HandleRoomInfo(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	code := strings.TrimPrefix(path, "/api/room/")
	code = strings.TrimSuffix(code, "/info")
	code = strings.TrimSpace(code)

	if code == "" {
		http.Error(w, `{"error":"Room code required"}`, http.StatusBadRequest)
		return
	}

	rmRoom := h.Manager.GetOrCreateRoom(code)
	w.Header().Set("Content-Type", "application/json")

	if rmRoom.IsExpired() {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"expired":          true,
			"requiresPassword": false,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":               rmRoom.ID,
		"name":             rmRoom.Name,
		"requiresPassword": rmRoom.RequiresPassword(),
		"expired":          false,
	})
}

func (h *RoomAPIHandler) HandleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	claims, err := auth.ParseJWTToken(strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	code := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
	code = strings.TrimSpace(code)

	if db.Database != nil {
		if err := db.Database.DeleteOwnedRoom(code, claims.UserID); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
	}

	h.Manager.RemoveRoom(code)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}
