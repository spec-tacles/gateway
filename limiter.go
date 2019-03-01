package gateway

import (
	"time"
)

type limiter struct {
	limit    int32
	duration time.Duration

	resetsAt  int64
	available int32
}

func newLimiter(limit int32, duration time.Duration) *limiter {
	return &limiter{
		limit:    limit,
		duration: duration,
	}
}

func (l *limiter) lock() {
	now := time.Now().UnixNano()

	if l.resetsAt <= now {
		l.resetsAt = now + int64(l.duration)
		l.available = l.limit
	}

	if l.available <= 0 {
		time.Sleep(time.Duration(l.resetsAt - now))
		l.lock()
		return
	}

	l.available--
	return
}
