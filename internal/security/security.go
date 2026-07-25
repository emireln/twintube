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
	if secret != "" && secret != defaultJWTSecret {
		return
	}
	if isProduction() {
		log.Fatal("[SECURITY] JWT_SECRET must be set to a strong random value in production")
	}
	log.Println("[SECURITY] WARNING: Using default JWT secret — set JWT_SECRET before deploying")
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
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' https://www.youtube.com https://www.youtube.com/iframe_api https://s.ytimg.com https://cdn.jsdelivr.net",
		"style-src 'self' 'unsafe-inline'",
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
	mu       sync.Mutex
	limit    int
	window   time.Duration
	byIP     map[string]*rateBucket
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		byIP:   make(map[string]*rateBucket),
	}
}

func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
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
