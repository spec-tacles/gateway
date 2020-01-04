package gateway

import (
	"log"
	"os"
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

	Logger   Logger
	LogLevel int
}

func (opts *ManagerOptions) init() {
	if opts.ShardLimiter == nil {
		opts.ShardLimiter = NewDefaultLimiter(1, 5*time.Second)
	}

	if opts.ServerCount == 0 {
		opts.ServerCount = 1
	}

	if opts.Logger == nil {
		opts.Logger = log.New(os.Stderr, "[Manager] ", log.LstdFlags)
	}
}
