package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

func initJWTSecret() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "twintube_default_super_secret_vps_key_change_me"
	}
	jwtSecret = []byte(secret)
}

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	AvatarURL    string    `json:"avatarUrl"`
	CreatedAt    time.Time `json:"createdAt"`
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
	Token string `json:"token"`
	User  User   `json:"user"`
}

// HashPassword hashes plaintext password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// CheckPasswordHash compares plaintext password with hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWTToken issues a signed JWT token
func GenerateJWTToken(userID, username string) (string, error) {
	if len(jwtSecret) == 0 {
		initJWTSecret()
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

// ParseJWTToken parses and validates a signed JWT token
func ParseJWTToken(tokenString string) (*Claims, error) {
	if len(jwtSecret) == 0 {
		initJWTSecret()
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

// HTTP Auth Handlers
func handleRegister(w http.ResponseWriter, r *http.Request) {
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

	// Check if username or email already exists
	if Database != nil {
		existing, _ := Database.GetUserByUsernameOrEmail(req.Username, req.Email)
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

	user := User{
		ID:           GenerateRandomID(10),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		AvatarURL:    fmt.Sprintf("https://api.dicebear.com/7.x/bottts/svg?seed=%s", req.Username),
		CreatedAt:    time.Now(),
	}

	if Database != nil {
		if err := Database.CreateUser(user); err != nil {
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

func handleLogin(w http.ResponseWriter, r *http.Request) {
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

	if Database == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusInternalServerError)
		return
	}

	user, err := Database.GetUserByUsernameOrEmail(req.UsernameOrEmail, req.UsernameOrEmail)
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

func handleGetMe(w http.ResponseWriter, r *http.Request) {
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

	if Database == nil {
		http.Error(w, `{"error":"Database unavailable"}`, http.StatusInternalServerError)
		return
	}

	user, err := Database.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
