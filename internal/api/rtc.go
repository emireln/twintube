package api

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// HandleRTCConfig returns STUN/TURN iceServers. TURN uses short-lived HMAC
// credentials when TURN_SECRET and TURN_HOST are configured.
func HandleRTCConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	stunURL := strings.TrimSpace(os.Getenv("STUN_URL"))
	if stunURL == "" {
		stunURL = "stun:stun.l.google.com:19302"
	}

	iceServers := []map[string]interface{}{
		{"urls": []string{stunURL}},
	}

	turnHost := strings.TrimSpace(os.Getenv("TURN_HOST"))
	turnSecret := strings.TrimSpace(os.Getenv("TURN_SECRET"))
	turnRealm := strings.TrimSpace(os.Getenv("TURN_REALM"))
	if turnRealm == "" {
		turnRealm = "twintube"
	}

	if turnHost != "" && turnSecret != "" {
		ttl := int64(3600)
		if v := strings.TrimSpace(os.Getenv("TURN_TTL_SECONDS")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 60 {
				ttl = n
			}
		}
		expiry := time.Now().Unix() + ttl
		username := strconv.FormatInt(expiry, 10) + ":twintube"
		mac := hmac.New(sha1.New, []byte(turnSecret))
		_, _ = mac.Write([]byte(username))
		credential := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		urls := []string{
			"turn:" + turnHost + ":3478?transport=udp",
			"turn:" + turnHost + ":3478?transport=tcp",
		}
		if strings.EqualFold(os.Getenv("TURN_TLS"), "true") || strings.EqualFold(os.Getenv("TURN_TLS"), "1") {
			urls = append(urls, "turns:"+turnHost+":5349?transport=tcp")
		}

		iceServers = append(iceServers, map[string]interface{}{
			"urls":       urls,
			"username":   username,
			"credential": credential,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"iceServers": iceServers,
		"maxPeers":   6,
		"realm":      turnRealm,
	})
}
