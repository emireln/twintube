package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"twintube/internal/auth"
	"twintube/internal/db"
	"twintube/internal/room"
	"twintube/internal/security"
	"twintube/internal/utils"

	"golang.org/x/crypto/bcrypt"
)

var roomAccessLimiter = security.NewRateLimiter(20, 5*time.Minute)

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
	hadAuth := strings.HasPrefix(authHeader, "Bearer ")
	if hadAuth {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := auth.ParseJWTToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}
		userID = claims.UserID
	}

	roomID := utils.GenerateRoomCode()
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "Room " + roomID
	}

	var pwdHash string
	if req.Password != "" {
		if len(req.Password) < 4 {
			http.Error(w, `{"error":"Room password must be at least 4 characters"}`, http.StatusBadRequest)
			return
		}
		bytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
		if err != nil {
			http.Error(w, `{"error":"Failed to secure room password"}`, http.StatusInternalServerError)
			return
		}
		pwdHash = string(bytes)
	}

	var expiresAt *time.Time
	if userID != "" {
		exp := time.Now().Add(7 * 24 * time.Hour)
		expiresAt = &exp
	}

	// Only persist owned rooms for signed-in users; guests get ephemeral in-memory rooms.
	var rmRoom *room.Room
	if userID == "" {
		rmRoom = h.Manager.CreateGuestRoom(roomID, req.Name, pwdHash, req.Password != "")
	} else {
		if db.Database != nil {
			if err := db.Database.CreateRoomRecord(roomID, req.Name, userID, pwdHash, req.Password != "", expiresAt); err != nil {
				http.Error(w, `{"error":"Failed to create room"}`, http.StatusInternalServerError)
				return
			}
		}
		rmRoom = h.Manager.GetOrCreateRoom(roomID)
		rmRoom.Name = req.Name
		rmRoom.IsPrivate = req.Password != ""
		rmRoom.OwnerID = userID
		rmRoom.PasswordHash = pwdHash
		rmRoom.ExpiresAt = expiresAt
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateRoomResponse{
		RoomCode:  roomID,
		URL:       "/room/" + roomID,
		Name:      rmRoom.Name,
		IsPrivate: rmRoom.IsPrivate,
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

	if !utils.IsValidRoomID(code) {
		http.Error(w, `{"error":"Invalid room code"}`, http.StatusBadRequest)
		return
	}

	info := h.lookupRoomInfo(code)

	var userID string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := auth.ParseJWTToken(tokenStr); err == nil && claims != nil {
			userID = claims.UserID
		}
	}

	isOwner := userID != "" && info.OwnerID != "" && userID == info.OwnerID

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":               info.ID,
		"name":             info.Name,
		"requiresPassword": info.RequiresPassword,
		"expired":          info.Expired,
		"isOwner":          isOwner,
	})
}

type roomAccessRequest struct {
	Password string `json:"password"`
}

func (h *RoomAPIHandler) HandleRoomAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	code := strings.TrimPrefix(path, "/api/room/")
	code = strings.TrimSuffix(code, "/access")
	code = strings.TrimSpace(code)

	if code == "" || !utils.IsValidRoomID(code) {
		http.Error(w, `{"error":"Invalid room code"}`, http.StatusBadRequest)
		return
	}

	meta := h.Manager.LookupJoinMeta(code)
	if meta.Expired {
		http.Error(w, `{"error":"This room has expired."}`, http.StatusGone)
		return
	}

	var userID string
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := auth.ParseJWTToken(tokenStr); err == nil && claims != nil {
			userID = claims.UserID
		}
	}

	isOwner := userID != "" && meta.OwnerID != "" && userID == meta.OwnerID
	if isOwner || !meta.RequiresPassword() {
		token, err := room.IssueJoinToken(code)
		if err != nil {
			http.Error(w, `{"error":"Failed to issue access token"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"accessGranted": true,
			"joinToken":     token,
			"expiresIn":     180,
		})
		return
	}

	var req roomAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	req.Password = strings.TrimSpace(req.Password)
	if len(req.Password) < 4 {
		http.Error(w, `{"error":"Room password must be at least 4 characters"}`, http.StatusBadRequest)
		return
	}

	failKey := code + "|" + security.ClientIP(r)
	if !roomAccessLimiter.Allow(failKey) {
		http.Error(w, `{"error":"Too many failed password attempts. Try again later."}`, http.StatusTooManyRequests)
		return
	}

	if !auth.CheckPasswordHash(req.Password, meta.PasswordHash) {
		http.Error(w, `{"error":"Incorrect room password."}`, http.StatusForbidden)
		return
	}

	token, err := room.IssueJoinToken(code)
	if err != nil {
		http.Error(w, `{"error":"Failed to issue access token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"accessGranted": true,
		"joinToken":     token,
		"expiresIn":     180,
	})
}

type roomInfoSnapshot struct {
	ID               string
	Name             string
	OwnerID          string
	RequiresPassword bool
	Expired          bool
}

func (h *RoomAPIHandler) lookupRoomInfo(code string) roomInfoSnapshot {
	meta := h.Manager.LookupJoinMeta(code)
	if meta.Expired {
		return roomInfoSnapshot{ID: code, Expired: true}
	}
	return roomInfoSnapshot{
		ID:               meta.RoomID,
		Name:             meta.Name,
		OwnerID:          meta.OwnerID,
		RequiresPassword: meta.RequiresPassword(),
	}
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

	if !utils.IsValidRoomID(code) {
		http.Error(w, `{"error":"Invalid room code"}`, http.StatusBadRequest)
		return
	}

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
