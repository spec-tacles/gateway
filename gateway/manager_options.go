package gateway

import (
	"log"
	"time"

	"github.com/spec-tacles/go/types"
)

// ShardLimiter controls the rate at which the manager creates and starts shards
type ShardLimiter interface {
	Wait(int) error
}

// ManagerOptions represents NewManager's options
type ManagerOptions struct {
	ShardOptions *ShardOptions
	REST         REST
	ShardLimiter Limiter

	ShardCount  int
	ServerIndex int
	ServerCount int

	OnPacket func(int, *types.ReceivePacket)

	Logger   *log.Logger
	LogLevel int
}

func (opts *ManagerOptions) init() {
	if opts.ShardLimiter == nil {
		// this is supposed to be 5s, but 5s causes every other session to be invalidated
		opts.ShardLimiter = NewDefaultLimiter(1, 5250*time.Millisecond)
	}

	if opts.ServerCount == 0 {
		opts.ServerCount = 1
	}

	if opts.Logger == nil {
		opts.Logger = DefaultLogger
	}
	opts.Logger = ChildLogger(opts.Logger, "[manager]")
}
