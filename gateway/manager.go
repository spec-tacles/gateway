package gateway

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/spec-tacles/gateway/stats"
	"github.com/spec-tacles/go/broker"
	"github.com/spec-tacles/go/types"
)

// Manager manages Gateway shards
type Manager struct {
	Shards  map[int]*Shard
	Gateway *types.GatewayBot
	opts    *ManagerOptions
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

	wg := sync.WaitGroup{}
	for i := m.opts.ServerIndex; i < m.opts.ShardCount; i += m.opts.ServerCount {
		id := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := m.Spawn(id)
			if err != nil {
				m.log(LogLevelError, "connection error in shard %d: %s", id, err)
			}
		}()
	}

	stats.TotalShards.Add(float64(m.opts.ShardCount))
	wg.Wait()
	return
}

// Spawn a new shard with the specified ID
func (m *Manager) Spawn(id int) (err error) {
	g, err := m.FetchGateway()
	if err != nil {
		return
	}

	opts := m.opts.ShardOptions.clone()
	opts.Identify.Shard = []int{id, m.opts.ShardCount}
	opts.LogLevel = m.opts.LogLevel
	opts.IdentifyLimiter = m.opts.ShardLimiter
	if opts.Logger == nil {
		opts.Logger = m.opts.Logger
	}

	if m.opts.OnPacket != nil {
		opts.OnPacket = func(r *types.ReceivePacket) {
			m.opts.OnPacket(id, r)
		}
	}

	s := NewShard(opts)
	s.Gateway = g
	m.Shards[id] = s

	err = s.Open()
	if err != nil {
		return
	}

	return s.Close()
}

// FetchGateway fetches the gateway or from cache
func (m *Manager) FetchGateway() (g *types.GatewayBot, err error) {
	if m.Gateway != nil {
		g = m.Gateway
	} else {
		g, err = FetchGatewayBot(m.opts.REST)
		m.Gateway = g
	}
	return
}

// ConnectBroker connects a broker to this manager. It forwards all packets from the gateway and
// consumes packets from the broker for all shards it's responsible for.
func (m *Manager) ConnectBroker(b *BrokerManager, events map[string]struct{}, timeout time.Duration) {
	if b == nil {
		return
	}

	m.opts.OnPacket = func(shard int, d *types.ReceivePacket) {
		if d.Op != types.GatewayOpDispatch {
			return
		}

		if _, ok := events[string(d.Event)]; !ok {
			return
		}

		err := b.PublishOptions(broker.PublishOptions{
			Event:   string(d.Event),
			Data:    d.Data,
			Timeout: timeout,
		})
		if err != nil {
			m.log(LogLevelError, "failed to publish packet to broker: %s", err)
		}
	}

	b.SetCallback(m.HandleEvent)
	go m.Subscribe(b, "SEND")
	for id := range m.Shards {
		go m.Subscribe(b, strconv.FormatInt(int64(id), 10))
	}
}

// Subscribe subscribes to the given event on the given broker and logs any errors
func (m *Manager) Subscribe(b *BrokerManager, event string) {
	err := b.Subscribe(event)
	if err != nil {
		m.log(LogLevelError, "failed to subscribe to event \"%s\": %s", event, err)
	}
}

// HandleEvent handles an incoming message to be potentially sent on a shard that this manager is
// responsible for
func (m *Manager) HandleEvent(event string, d []byte) {
	var (
		shard  *Shard
		packet *types.SendPacket
	)
	if event == "SEND" {
		p := &UnknownSendPacket{}
		err := json.Unmarshal(d, p)
		if err != nil {
			m.log(LogLevelWarn, "unable to parse SEND packet: %s", err)
			return
		}

		shard = m.Shards[int(p.GuildID>>22%uint64(m.opts.ShardCount))]
		if shard == nil {
			// TODO: republish back to AMQP
			return
		}
		packet = p.Packet
	} else {
		shardID, err := strconv.ParseInt(event, 10, 64)
		if err != nil {
			m.log(LogLevelWarn, "received unexpected non-int event from AMQP: %s", err)
		}
		shard = m.Shards[int(shardID)]
		if shard == nil {
			m.log(LogLevelWarn, "received event for shard %d which does not exist", shardID)
			return
		}

		err = json.Unmarshal(d, packet)
		if err != nil {
			m.log(LogLevelWarn, "unable to parse packet intended for shard %d: %s", shardID, err)
			return
		}
	}

	err := shard.Send(packet)
	if err != nil {
		m.log(LogLevelError, "error sending packet (%d): %s", packet.Op, err)
	}
}
