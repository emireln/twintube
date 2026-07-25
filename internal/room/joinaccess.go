package room

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"twintube/internal/db"
)

const joinTokenIssuer = "twintube-room-join"

// JoinMeta describes room access requirements without creating a room shell.
type JoinMeta struct {
	RoomID       string
	Name         string
	OwnerID      string
	PasswordHash string
	Expired      bool
	Found        bool
}

func (m JoinMeta) RequiresPassword() bool {
	return m.PasswordHash != ""
}

type roomJoinClaims struct {
	RoomID string `json:"roomId"`
	jwt.RegisteredClaims
}

var joinTokenSecret []byte

// SetJoinTokenSecret configures the HMAC key for short-lived room join tokens.
func SetJoinTokenSecret(secret []byte) {
	joinTokenSecret = secret
}

// LookupJoinMeta reads room access metadata from memory or DB without creating rooms.
func (rm *RoomManager) LookupJoinMeta(roomID string) JoinMeta {
	defaultName := "Room " + roomID

	if rmRoom := rm.GetRoom(roomID); rmRoom != nil {
		if rmRoom.IsExpired() {
			return JoinMeta{RoomID: roomID, Expired: true, Found: true}
		}
		return JoinMeta{
			RoomID:       roomID,
			Name:         rmRoom.Name,
			OwnerID:      rmRoom.OwnerIDValue(),
			PasswordHash: rmRoom.PasswordHashValue(),
			Found:        true,
		}
	}

	if db.Database != nil {
		rec, err := db.Database.GetRoomByID(roomID)
		if err != nil || rec == nil {
			return JoinMeta{RoomID: roomID, Name: defaultName}
		}

		expired := false
		if rec.OwnerID != "" && rec.ExpiresAt != nil && !rec.ExpiresAt.After(time.Now()) {
			expired = true
		}

		name := rec.Name
		if name == "" {
			name = defaultName
		}

		return JoinMeta{
			RoomID:       roomID,
			Name:         name,
			OwnerID:      rec.OwnerID,
			PasswordHash: rec.PasswordHash,
			Expired:      expired,
			Found:        true,
		}
	}

	return JoinMeta{RoomID: roomID, Name: defaultName}
}

// IssueJoinToken returns a short-lived token proving room password was verified via HTTP.
func IssueJoinToken(roomID string) (string, error) {
	if len(joinTokenSecret) == 0 {
		return "", fmt.Errorf("join token secret not configured")
	}

	jti, err := newJoinJTI()
	if err != nil {
		return "", err
	}

	claims := roomJoinClaims{
		RoomID: roomID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(3 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    joinTokenIssuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(joinTokenSecret)
}

// ValidateJoinToken checks signature, expiry, and room binding.
func ValidateJoinToken(tokenString, roomID string) bool {
	if tokenString == "" || roomID == "" || len(joinTokenSecret) == 0 {
		return false
	}

	claims := &roomJoinClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return joinTokenSecret, nil
	})
	if err != nil || !token.Valid {
		return false
	}
	return claims.Issuer == joinTokenIssuer && claims.RoomID == roomID && claims.ID != ""
}

func newJoinJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
