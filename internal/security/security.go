package security

import (
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultJWTSecret = "twintube_default_super_secret_vps_key_change_me"

// ValidateJWTSecret warns or exits when the signing key is missing in production.
func ValidateJWTSecret() {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if validJWTSecret(secret) {
		return
	}
	if isProduction() {
		log.Fatal("[SECURITY] JWT_SECRET must be at least 32 bytes, non-placeholder, and contain sufficient character diversity in production")
	}
	log.Println("[SECURITY] WARNING: JWT_SECRET is weak or unset — set a random value of at least 32 bytes before deploying")
}

func validJWTSecret(secret string) bool {
	if len(secret) < 32 || secret == defaultJWTSecret {
		return false
	}
	lower := strings.ToLower(secret)
	for _, marker := range []string{"change_me", "change-this", "change_this", "your_super", "your_secret", "placeholder", "example", "replace_me", "password"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	unique := make(map[rune]struct{})
	for _, ch := range secret {
		unique[ch] = struct{}{}
	}
	return len(unique) >= 12
}

func isProduction() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	return env == "production" || env == "prod"
}

// ApplyHeaders sets standard security response headers.
func ApplyHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(self), geolocation=()")
	w.Header().Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' https://www.youtube.com https://www.youtube.com/iframe_api https://s.ytimg.com https://cdn.jsdelivr.net",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com data:",
		"img-src 'self' data: blob: https:",
		"media-src 'self' blob: https: http:",
		"connect-src 'self' ws: wss: https:",
		"frame-src 'self' https://www.youtube.com https://www.youtube-nocookie.com https://player.vimeo.com https://player.twitch.tv https://www.dailymotion.com https://streamable.com https:",
		"worker-src 'self' blob:",
		"base-uri 'self'",
		"form-action 'self'",
	}, "; "))
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

// Middleware wraps an handler with security headers.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ApplyHeaders(w, r)
		next.ServeHTTP(w, r)
	})
}

// Wrap applies security headers to a HandlerFunc.
func Wrap(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ApplyHeaders(w, r)
		fn(w, r)
	}
}

// --- Simple in-memory rate limiter (auth endpoints) ---

type rateBucket struct {
	count    int
	windowAt time.Time
}

type RateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	byIP      map[string]*rateBucket
	cleanupAt time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		byIP:   make(map[string]*rateBucket),
	}
}

func ClientIP(r *http.Request) string {
	peer := parseIPValue(r.RemoteAddr)
	if peer == nil {
		return r.RemoteAddr
	}
	if !isTrustedProxy(peer) {
		return peer.String()
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := parseIPValue(parts[i])
			if ip == nil {
				continue
			}
			if !isTrustedProxy(ip) {
				return ip.String()
			}
		}
	}
	if xrip := parseIPValue(r.Header.Get("X-Real-IP")); xrip != nil && !isTrustedProxy(xrip) {
		return xrip.String()
	}
	return peer.String()
}

func parseIPValue(value string) net.IP {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	return net.ParseIP(value)
}

func isTrustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	for _, entry := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if trustedIP := net.ParseIP(entry); trustedIP != nil && trustedIP.Equal(ip) {
			return true
		}
		if _, network, err := net.ParseCIDR(entry); err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if rl.cleanupAt.IsZero() || now.Sub(rl.cleanupAt) >= rl.window {
		for key, bucket := range rl.byIP {
			if now.Sub(bucket.windowAt) >= rl.window {
				delete(rl.byIP, key)
			}
		}
		rl.cleanupAt = now
	}
	b, ok := rl.byIP[ip]
	if !ok || now.Sub(b.windowAt) >= rl.window {
		rl.byIP[ip] = &rateBucket{count: 1, windowAt: now}
		return true
	}
	if b.count >= rl.limit {
		return false
	}
	b.count++
	return true
}

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(ClientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"Too many requests. Please try again later."}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
