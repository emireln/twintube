package security

import (
	"testing"
	"time"
)

func TestMessageLimiterAllowsBurstThenRefills(t *testing.T) {
	limiter := NewMessageLimiter(10, 5)
	start := time.Now()
	limiter.last = start

	for i := 0; i < 5; i++ {
		if !limiter.allowAt(start) {
			t.Fatalf("message %d within the burst should be allowed", i+1)
		}
	}
	if limiter.allowAt(start) {
		t.Fatal("message beyond the burst should be refused")
	}

	// 100 ms at 10 messages per second refills exactly one token.
	if !limiter.allowAt(start.Add(100 * time.Millisecond)) {
		t.Fatal("one refilled token should allow one message")
	}
	if limiter.allowAt(start.Add(100 * time.Millisecond)) {
		t.Fatal("only one token should have refilled")
	}
}

func TestMessageLimiterCapsBurstAfterIdle(t *testing.T) {
	limiter := NewMessageLimiter(10, 5)
	start := time.Now()
	limiter.last = start

	later := start.Add(time.Hour)
	allowed := 0
	for limiter.allowAt(later) {
		allowed++
		if allowed > 5 {
			t.Fatal("tokens must not accumulate beyond the burst")
		}
	}
	if allowed != 5 {
		t.Fatalf("expected a full burst of 5 after idling, got %d", allowed)
	}
}
