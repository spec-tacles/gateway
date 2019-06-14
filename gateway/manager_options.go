package gateway

import (
	"io"
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
	ShardLimiter ShardLimiter

	ShardCount  int
	ServerIndex int
	ServerCount int

	Output   io.Writer
	OnPacket func(int, *types.ReceivePacket)

	Logger   Logger
	LogLevel int
}

func (opts *ManagerOptions) init() {
	if opts.ShardLimiter == nil {
		opts.ShardLimiter = &defaultIdentifyLimiter{opts}
	}

	if opts.ServerCount == 0 {
		opts.ServerCount = 1
	}

	if opts.Logger == nil {
		opts.Logger = log.New(os.Stdout, "[Manager] ", log.LstdFlags)
	}
}

type defaultIdentifyLimiter struct {
	opts *ManagerOptions
}

const startDelay = time.Second * 5

func (il *defaultIdentifyLimiter) Wait(id int) error {
	if id == il.opts.ServerIndex {
		return nil
	}

	time.Sleep(startDelay)
	return nil
}
