package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"twintube/internal/db"
	"twintube/internal/utils"
)

var jwtSecret []byte

func InitJWTSecret() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "twintube_default_super_secret_vps_key_change_me"
	}
	jwtSecret = []byte(secret)
}

type Claims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	UsernameOrEmail string `json:"usernameOrEmail"`
	Password        string `json:"password"`
}

type AuthResponse struct {
	Token string  `json:"token"`
	User  db.User `json:"user"`
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func JWTSecretBytes() []byte {
	if len(jwtSecret) == 0 {
		InitJWTSecret()
	}
	out := make([]byte, len(jwtSecret))
	copy(out, jwtSecret)
	return out
}

func GenerateJWTToken(userID, username string) (string, error) {
	if len(jwtSecret) == 0 {
		InitJWTSecret()
	}

	expirationTime := time.Now().Add(72 * time.Hour)
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "twintube-auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ParseJWTToken(tokenString string) (*Claims, error) {
	if len(jwtSecret) == 0 {
		InitJWTSecret()
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)

	if len(req.Username) < 3 || len(req.Username) > 25 {
		http.Error(w, `{"error":"Username must be between 3 and 25 characters"}`, http.StatusBadRequest)
		return
	}

	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	if !emailRegex.MatchString(req.Email) {
		http.Error(w, `{"error":"Invalid email address"}`, http.StatusBadRequest)
		return
	}

	if len(req.Password) < 6 {
		http.Error(w, `{"error":"Password must be at least 6 characters long"}`, http.StatusBadRequest)
		return
	}

	if db.Database != nil {
		existing, _ := db.Database.GetUserByUsernameOrEmail(req.Username, req.Email)
		if existing != nil {
			http.Error(w, `{"error":"Username or email is already registered"}`, http.StatusConflict)
			return
		}
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		http.Error(w, `{"error":"Failed to process password"}`, http.StatusInternalServerError)
		return
	}

	user := db.User{
		ID:           utils.GenerateRandomID(10),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		AvatarURL:    defaultAvatarURL(req.Username),
		CreatedAt:    time.Now(),
	}

	if db.Database != nil {
		if err := db.Database.CreateUser(user); err != nil {
			http.Error(w, `{"error":"Failed to create user account"}`, http.StatusInternalServerError)
			return
		}
	}

	token, err := GenerateJWTToken(user.ID, user.Username)
	if err != nil {
		http.Error(w, `{"error":"Failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User:  user,
	})
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	req.UsernameOrEmail = strings.TrimSpace(req.UsernameOrEmail)
	req.Password = strings.TrimSpace(req.Password)

	if req.UsernameOrEmail == "" || req.Password == "" {
		http.Error(w, `{"error":"Username/Email and Password are required"}`, http.StatusBadRequest)
		return
	}

	if db.Database == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusInternalServerError)
		return
	}

	user, err := db.Database.GetUserByUsernameOrEmail(req.UsernameOrEmail, req.UsernameOrEmail)
	if err != nil || user == nil {
		http.Error(w, `{"error":"Invalid username/email or password"}`, http.StatusUnauthorized)
		return
	}

	if !CheckPasswordHash(req.Password, user.PasswordHash) {
		http.Error(w, `{"error":"Invalid username/email or password"}`, http.StatusUnauthorized)
		return
	}

	token, err := GenerateJWTToken(user.ID, user.Username)
	if err != nil {
		http.Error(w, `{"error":"Failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User:  *user,
	})
}

func HandleGetMe(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := ParseJWTToken(tokenString)
	if err != nil {
		http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	if db.Database == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusInternalServerError)
		return
	}

	user, err := db.Database.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

type UpdateProfileRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func userFromRequest(r *http.Request) (*Claims, *db.User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, nil, fmt.Errorf("unauthorized")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := ParseJWTToken(tokenString)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid token")
	}

	if db.Database == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}

	user, err := db.Database.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		return nil, nil, fmt.Errorf("user not found")
	}

	return claims, user, nil
}

func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func isValidAvatarURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	if len(raw) > 2048 {
		return false
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	// Uploaded avatars served from this host
	if strings.HasPrefix(lower, "/uploads/avatars/") {
		return !strings.Contains(raw, "..")
	}
	return false
}

func defaultAvatarURL(username string) string {
	return fmt.Sprintf("https://api.dicebear.com/7.x/bottts/png?size=128&seed=%s", url.QueryEscape(username))
}

func HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, user, err := userFromRequest(r)
	if err != nil {
		writeJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.AvatarURL = strings.TrimSpace(req.AvatarURL)

	if len(req.Username) < 3 || len(req.Username) > 25 {
		writeJSONError(w, "Username must be between 3 and 25 characters", http.StatusBadRequest)
		return
	}

	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	if !emailRegex.MatchString(req.Email) {
		writeJSONError(w, "Invalid email address", http.StatusBadRequest)
		return
	}

	if !isValidAvatarURL(req.AvatarURL) {
		writeJSONError(w, "Avatar must be an http(s) link or an uploaded image", http.StatusBadRequest)
		return
	}

	if req.AvatarURL == "" {
		req.AvatarURL = defaultAvatarURL(req.Username)
	}

	taken, err := db.Database.IsUsernameOrEmailTaken(req.Username, req.Email, user.ID)
	if err != nil {
		writeJSONError(w, "Failed to validate profile", http.StatusInternalServerError)
		return
	}
	if taken {
		writeJSONError(w, "Username or email is already in use", http.StatusConflict)
		return
	}

	if err := db.Database.UpdateUserProfile(user.ID, req.Username, req.Email, req.AvatarURL); err != nil {
		writeJSONError(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	user.Username = req.Username
	user.Email = req.Email
	user.AvatarURL = req.AvatarURL

	token, err := GenerateJWTToken(user.ID, user.Username)
	if err != nil {
		writeJSONError(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User:  *user,
	})
}

func HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, user, err := userFromRequest(r)
	if err != nil {
		writeJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	req.CurrentPassword = strings.TrimSpace(req.CurrentPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)

	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeJSONError(w, "Current and new password are required", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 6 {
		writeJSONError(w, "New password must be at least 6 characters long", http.StatusBadRequest)
		return
	}

	if !CheckPasswordHash(req.CurrentPassword, user.PasswordHash) {
		writeJSONError(w, "Current password is incorrect", http.StatusUnauthorized)
		return
	}

	passwordHash, err := HashPassword(req.NewPassword)
	if err != nil {
		writeJSONError(w, "Failed to process password", http.StatusInternalServerError)
		return
	}

	if err := db.Database.UpdateUserPassword(user.ID, passwordHash); err != nil {
		writeJSONError(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Password updated"})
}

const maxAvatarBytes = 2 << 20 // 2 MiB

func avatarsDir() string {
	dir := strings.TrimSpace(os.Getenv("AVATARS_DIR"))
	if dir == "" {
		dir = filepath.Join(".", "uploads", "avatars")
	}
	return dir
}

func detectImageExt(header []byte) (string, string, bool) {
	ctype := http.DetectContentType(header)
	switch ctype {
	case "image/jpeg":
		return ".jpg", ctype, true
	case "image/png":
		return ".png", ctype, true
	case "image/gif":
		return ".gif", ctype, true
	case "image/webp":
		return ".webp", ctype, true
	default:
		return "", ctype, false
	}
}

// HandleUploadAvatar accepts a multipart image and stores it under /uploads/avatars/.
func HandleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, user, err := userFromRequest(r)
	if err != nil {
		writeJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+512)
	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		writeJSONError(w, "Image must be 2 MB or smaller", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeJSONError(w, "Choose an image file to upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	ext, _, ok := detectImageExt(head)
	if !ok {
		writeJSONError(w, "Only JPEG, PNG, WebP, or GIF images are allowed", http.StatusBadRequest)
		return
	}

	dir := avatarsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeJSONError(w, "Failed to prepare avatar storage", http.StatusInternalServerError)
		return
	}

	// Remove previous uploads for this user (any extension).
	for _, oldExt := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
		_ = os.Remove(filepath.Join(dir, user.ID+oldExt))
	}

	filename := user.ID + ext
	destPath := filepath.Join(dir, filename)
	out, err := os.Create(destPath)
	if err != nil {
		writeJSONError(w, "Failed to save avatar", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := out.Write(head); err != nil {
		writeJSONError(w, "Failed to save avatar", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		writeJSONError(w, "Failed to save avatar", http.StatusInternalServerError)
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s?v=%d", filename, time.Now().Unix())
	if err := db.Database.UpdateUserProfile(user.ID, user.Username, user.Email, avatarURL); err != nil {
		writeJSONError(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}
	user.AvatarURL = avatarURL

	token, err := GenerateJWTToken(user.ID, user.Username)
	if err != nil {
		writeJSONError(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User:  *user,
	})
}
