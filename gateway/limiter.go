package gateway

import "time"

// Limiter represents something that blocks until a ratelimit has been fulfilled
type Limiter interface {
	Lock()
}

// DefaultLimiter is a limiter that works locally
type DefaultLimiter struct {
	limit    int32
	duration time.Duration

	resetsAt  int64
	available int32
}

// NewDefaultLimiter creates a default limiter
func NewDefaultLimiter(limit int32, duration time.Duration) Limiter {
	return &DefaultLimiter{
		limit:    limit,
		duration: duration,
	}
}

// Lock establishes a ratelimited lock on the limiter
func (l *DefaultLimiter) Lock() {
	now := time.Now().UnixNano()

	if l.resetsAt <= now {
		l.resetsAt = now + int64(l.duration)
		l.available = l.limit
	}

	if l.available <= 0 {
		time.Sleep(time.Duration(l.resetsAt - now))
		l.Lock()
		return
	}

	l.available--
	return
}
