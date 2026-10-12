package security

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientIPIgnoresHeadersFromUntrustedPeer(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32,10.0.0.0/8")
	request := httptest.NewRequest("GET", "http://example.test/", nil)
	request.RemoteAddr = "203.0.113.77:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.24")
	request.Header.Set("X-Real-IP", "198.51.100.24")

	if got := ClientIP(request); got != "203.0.113.77" {
		t.Fatalf("untrusted forwarded headers changed client IP: got %q", got)
	}
}

// Behind Docker port publishing or a proxy container, the app sees the bridge
// gateway (a private address) as the peer. Without TRUSTED_PROXIES set, that
// proxy must still be trusted or every visitor shares one rate-limit bucket.
func TestClientIPTrustsPrivateProxyByDefault(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	request := httptest.NewRequest("GET", "http://example.test/", nil)
	request.RemoteAddr = "172.18.0.1:44321"
	request.Header.Set("X-Forwarded-For", "198.51.100.24")

	if got := ClientIP(request); got != "198.51.100.24" {
		t.Fatalf("default proxy trust should honor X-Forwarded-For from a private peer: got %q", got)
	}
}

func TestClientIPIgnoresPrivatePeerWhenExplicitListOmitsIt(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")
	request := httptest.NewRequest("GET", "http://example.test/", nil)
	request.RemoteAddr = "172.18.0.1:44321"
	request.Header.Set("X-Forwarded-For", "198.51.100.24")

	if got := ClientIP(request); got != "172.18.0.1" {
		t.Fatalf("explicit TRUSTED_PROXIES should replace the private-network default: got %q", got)
	}
}

func TestClientIPWalksTrustedForwardedChain(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32,10.0.0.0/8")
	request := httptest.NewRequest("GET", "http://example.test/", nil)
	request.RemoteAddr = "127.0.0.1:8080"
	request.Header.Set("X-Forwarded-For", "198.51.100.24, 10.1.2.3")

	if got := ClientIP(request); got != "198.51.100.24" {
		t.Fatalf("unexpected client IP from forwarded chain: got %q", got)
	}
}

// Inline scripts would also execute javascript: URLs, which the player can
// receive from room state, so script-src must never allow 'unsafe-inline'.
func TestCSPForbidsInlineScripts(t *testing.T) {
	recorder := httptest.NewRecorder()
	ApplyHeaders(recorder, httptest.NewRequest("GET", "http://example.test/", nil))
	csp := recorder.Header().Get("Content-Security-Policy")
	var scriptSrc string
	for _, directive := range strings.Split(csp, ";") {
		if d := strings.TrimSpace(directive); strings.HasPrefix(d, "script-src ") {
			scriptSrc = d
		}
	}
	if scriptSrc == "" {
		t.Fatalf("CSP has no script-src directive: %q", csp)
	}
	if strings.Contains(scriptSrc, "'unsafe-inline'") || strings.Contains(scriptSrc, "'unsafe-eval'") {
		t.Fatalf("script-src must not allow inline or eval scripts: %q", scriptSrc)
	}
}

func TestValidJWTSecretRejectsWeakAndPlaceholderValues(t *testing.T) {
	for _, secret := range []string{
		"",
		strings.Repeat("a", 64),
		"CHANGE_ME_GENERATE_A_RANDOM_SECRET_1234567890",
		defaultJWTSecret,
	} {
		if validJWTSecret(secret) {
			t.Errorf("validJWTSecret(%q) = true; want false", secret)
		}
	}
	if !validJWTSecret("0123456789abcdef0123456789abcdef") {
		t.Fatal("expected a diverse 32-byte hexadecimal secret to be accepted")
	}
}

func TestRateLimiterPrunesExpiredEntries(t *testing.T) {
	limiter := NewRateLimiter(1, 10*time.Millisecond)
	if !limiter.Allow("old-client") {
		t.Fatal("first request should be allowed")
	}
	time.Sleep(25 * time.Millisecond)
	if !limiter.Allow("new-client") {
		t.Fatal("request from a different client should be allowed")
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if _, exists := limiter.byIP["old-client"]; exists {
		t.Fatal("expired client entry was not pruned")
	}
}
