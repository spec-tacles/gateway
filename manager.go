package gateway

import (
	"fmt"
	"log"
	"os"

	"github.com/spec-tacles/spectacles.go/types"
)

// Manager manages Gateway shards
type Manager struct {
	Shards map[int]*Shard
	opts   *ManagerOptions
}

// NewManager creates a new Gateway manager
func NewManager(opts *ManagerOptions) *Manager {
	opts.init()

	return &Manager{
		Shards: make(map[int]*Shard),
		opts:   opts,
	}
}

// Start starts all shards
func (m *Manager) Start() (err error) {
	g, err := FetchGatewayBot(m.opts.REST)
	if err != nil {
		return
	}

	if m.opts.ShardCount == 0 {
		m.opts.ShardCount = g.Shards
	}

	expected := m.opts.ShardCount / m.opts.ServerCount
	if m.opts.ServerIndex < (m.opts.ShardCount % m.opts.ServerCount) {
		expected++
	}

	m.log(LogLevelInfo, "Starting %d shard(s)", expected)

	for i := m.opts.ServerIndex; i < m.opts.ShardCount; i += m.opts.ServerCount {
		if err = m.opts.ShardLimiter.Wait(i); err != nil {
			return
		}

		opts := m.opts.ShardOptions.clone()
		opts.Identify.Shard = []int{i, m.opts.ShardCount}
		if opts.Logger == nil {
			opts.Logger = log.New(os.Stdout, fmt.Sprintf("[Shard %d] ", i), log.LstdFlags)
		}

		s := NewShard(opts)
		s.Gateway = g

		{
			id := i
			opts.OnPacket = func(r *types.ReceivePacket) {
				m.opts.OnPacket(id, r)
			}
		}

		if err = s.Open(); err != nil {
			for id, s := range m.Shards {
				if err = s.Close(); err != nil {
					m.log(LogLevelInfo, "Error while closing shard %d: %v", id, err)
				}
			}

			return
		}

		m.Shards[i] = s

		m.log(LogLevelInfo, "Started shard %d", i)
	}

	return
}
