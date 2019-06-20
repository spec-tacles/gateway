package gateway

import "time"

type Limiter interface {
	Lock()
}

type DefaultLimiter struct {
	limit    int32
	duration time.Duration

	resetsAt  int64
	available int32
}

func NewDefaultLimiter(limit int32, duration time.Duration) Limiter {
	return &DefaultLimiter{
		limit:    limit,
		duration: duration,
	}
}

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
