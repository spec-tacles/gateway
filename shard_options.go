package gateway

import (
	"log"
	"os"
	"runtime"
	"time"

	"github.com/spec-tacles/go/types"
)

// Retryer calculates the wait time between retries
type Retryer interface {
	FirstTimeout() time.Duration
	NextTimeout(time.Duration, int) (time.Duration, error)
}

// ShardOptions represents NewShard's options
type ShardOptions struct {
	Identify *types.Identify
	Version  string
	Retryer  Retryer

	OnPacket func(*types.ReceivePacket)

	Logger   Logger
	LogLevel int
}

func (opts *ShardOptions) init() {
	if opts.Version == "" {
		opts.Version = DefaultVersion
	}

	if opts.Logger == nil {
		opts.Logger = defaultLogger
	}

	if opts.Retryer == nil {
		opts.Retryer = defaultRetryer{}
	}

	if opts.Identify != nil {
		if opts.Identify.Properties == nil {
			opts.Identify.Properties = &types.IdentifyProperties{
				OS:      runtime.GOOS,
				Browser: "spectacles.go",
				Device:  "spectacles.go",
			}
		}
	}
}

// clone only clones whatever's necessary
func (opts ShardOptions) clone() *ShardOptions {
	i := *opts.Identify
	opts.Identify = &i
	return &opts
}

var defaultLogger = log.New(os.Stdout, "[Shard] ", log.LstdFlags)

type defaultRetryer struct{}

const maxRetries = 5
const maxRetry = time.Minute * 5

func (defaultRetryer) FirstTimeout() time.Duration { return time.Second }
func (defaultRetryer) NextTimeout(timeout time.Duration, retries int) (time.Duration, error) {
	if retries > 5 {
		return 0, ErrMaxRetriesExceeded
	}

	timeout *= 2

	if timeout > maxRetry {
		timeout = maxRetry
	}

	return timeout, nil
}
