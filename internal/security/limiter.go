package security

import (
	"math"
	"sync"
	"time"
)

// MessageLimiter is a token bucket for one connection's message rate. Tokens refill
// at perSecond up to burst. Each message spends one token.
type MessageLimiter struct {
	mu        sync.Mutex
	perSecond float64
	burst     float64
	tokens    float64
	last      time.Time
}

func NewMessageLimiter(perSecond, burst int) *MessageLimiter {
	return &MessageLimiter{
		perSecond: float64(perSecond),
		burst:     float64(burst),
		tokens:    float64(burst),
		last:      time.Now(),
	}
}

// Allow spends one token and reports whether the message may be processed.
func (l *MessageLimiter) Allow() bool {
	return l.allowAt(time.Now())
}

func (l *MessageLimiter) allowAt(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elapsed := now.Sub(l.last).Seconds(); elapsed > 0 {
		l.tokens = math.Min(l.burst, l.tokens+elapsed*l.perSecond)
		l.last = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
