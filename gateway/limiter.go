package gateway

import (
	"sync"
	"sync/atomic"
	"time"
)

// Limiter represents something that blocks until a ratelimit has been fulfilled
type Limiter interface {
	Lock()
}

// DefaultLimiter is a limiter that works locally
type DefaultLimiter struct {
	limit    *int32
	duration *int64

	resetsAt  *int64
	available *int32
	mux       sync.Mutex
}

// NewDefaultLimiter creates a default limiter
func NewDefaultLimiter(limit int32, duration time.Duration) Limiter {
	nanos := duration.Nanoseconds()
	return &DefaultLimiter{
		limit:    &limit,
		duration: &nanos,

		resetsAt:  new(int64),
		available: new(int32),
		mux:       sync.Mutex{},
	}
}

// Lock establishes a ratelimited lock on the limiter
func (l *DefaultLimiter) Lock() {
	now := time.Now().UnixNano()

	if atomic.LoadInt64(l.resetsAt) <= now {
		atomic.StoreInt64(l.resetsAt, now+atomic.LoadInt64(l.duration))
		atomic.StoreInt32(l.available, atomic.LoadInt32(l.limit))
	}

	if atomic.LoadInt32(l.available) <= 0 {
		time.Sleep(time.Duration(atomic.LoadInt64(l.resetsAt) - now))
		l.Lock()
		return
	}

	atomic.AddInt32(l.available, -1)
	return
}
