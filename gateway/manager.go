package gateway

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/spec-tacles/gateway/stats"
	"github.com/spec-tacles/go/broker"
	"github.com/spec-tacles/go/types"
)

// RepublishPacket represents a SEND packet that now has a shard ID and must be re-published back to AMQP
type RepublishPacket struct {
	ShardID int
	Packet  *types.SendPacket
}

// Manager manages Gateway shards
type Manager struct {
	Shards      map[int]*Shard
	Gateway     *types.GatewayBot
	opts        *ManagerOptions
	gatewayLock sync.Mutex
}

// NewManager creates a new Gateway manager
func NewManager(opts *ManagerOptions) *Manager {
	opts.init()

	return &Manager{
		Shards:      make(map[int]*Shard),
		opts:        opts,
		gatewayLock: sync.Mutex{},
	}
}

// Start starts all shards
func (m *Manager) Start(ctx context.Context) (err error) {
	if m.opts.ShardCount == 0 {
		m.log(LogLevelDebug, "Shard count unspecified: using Discord recommended value")

		var g *types.GatewayBot
		g, err = m.FetchGateway()
		if err != nil {
			return
		}

		m.opts.ShardCount = g.Shards
	}

	expected := m.opts.ShardCount / m.opts.ServerCount
	if m.opts.ServerIndex < (m.opts.ShardCount % m.opts.ServerCount) {
		expected++
	}

	m.log(LogLevelInfo, "Starting %d shard(s) out of %d total", expected, m.opts.ShardCount)

	wg := sync.WaitGroup{}
	for i := m.opts.ServerIndex; i < m.opts.ShardCount; i += m.opts.ServerCount {
		id := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			stats.TotalShards.Add(1)
			defer stats.TotalShards.Sub(1)

			err := m.Spawn(ctx, id)
			if err != nil {
				m.log(LogLevelError, "Fatal error in shard %d: %s", id, err)
			} else {
				m.log(LogLevelDebug, "Shard %d closing gracefully", id)
			}
		}()
	}

	wg.Wait()
	return
}

// Spawn a new shard with the specified ID
func (m *Manager) Spawn(ctx context.Context, id int) (err error) {
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

	err = s.Open(ctx)
	if err != nil {
		return
	}

	return s.Close()
}

// FetchGateway fetches the gateway or from cache
func (m *Manager) FetchGateway() (g *types.GatewayBot, err error) {
	m.gatewayLock.Lock()
	defer m.gatewayLock.Unlock()

	if m.Gateway != nil {
		g = m.Gateway
	} else {
		g, err = FetchGatewayBot(m.opts.REST)
		m.log(LogLevelDebug, "Loaded gateway info %+v", g)
		m.Gateway = g
	}
	return
}

// ConnectBroker connects a broker to this manager. It forwards all packets from the gateway and
// consumes packets from the broker for all shards it's responsible for.
func (m *Manager) ConnectBroker(ctx context.Context, b broker.Broker, events map[string]struct{}) {
	ch := make(chan broker.Message)
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

		err := b.Publish(ctx, string(d.Event), d.Data)
		if err != nil {
			m.log(LogLevelError, "failed to publish packet to broker: %s", err)
		}
	}

	go func() {
		for msg := range ch {
			m.handleMessage(ctx, b, msg)
		}
	}()

	eventList := make([]string, len(m.Shards)+1)
	eventList = append(eventList, "SEND")
	for id := range m.Shards {
		eventList = append(eventList, strconv.FormatInt(int64(id), 10))
	}

	go b.Subscribe(ctx, eventList, ch)
}

func (m *Manager) handleMessage(ctx context.Context, b broker.Broker, msg broker.Message) {
	var (
		shard  *Shard
		packet *types.SendPacket
	)

	if msg.Event() == "SEND" {
		p := &UnknownSendPacket{}
		switch body := msg.Body().(type) {
		case []byte:
			err := json.Unmarshal(body, p)
			if err != nil {
				m.log(LogLevelWarn, "unable to parse SEND packet: %s", err)
				return
			}
		default:
			m.log(LogLevelWarn, "unexpected SEND packet type %T", body)
			return
		}

		shardID := int(p.GuildID >> 22 % uint64(m.opts.ShardCount))
		shard = m.Shards[shardID]
		if shard == nil {
			data, err := json.Marshal(p.Packet)
			if err != nil {
				m.log(LogLevelError, "error serializing SEND packet data (%+v): %s", *p.Packet, err)
				return
			}

			err = b.Publish(ctx, strconv.Itoa(shardID), data)
			if err != nil {
				m.log(LogLevelError, "error re-publishing SEND packet data to shard %d: %s", shardID, err)
			}
			return
		}
		packet = p.Packet
	} else {
		shardID, err := strconv.Atoi(msg.Event())
		if err != nil {
			m.log(LogLevelWarn, "received unexpected non-int event from AMQP: %s", err)
		}
		shard = m.Shards[shardID]
		if shard == nil {
			m.log(LogLevelWarn, "received event for shard %d which does not exist", shardID)
			return
		}

		switch body := msg.Body().(type) {
		case []byte:
			err := json.Unmarshal(body, packet)
			if err != nil {
				m.log(LogLevelWarn, "unable to parse packet intended for shard %d: %s", shardID, err)
				return
			}
		default:
			m.log(LogLevelWarn, "unexpected packet type %T", body)
			return
		}
	}

	err := shard.Send(packet)
	if err != nil {
		m.log(LogLevelError, "error sending packet (%d): %s", packet.Op, err)
	}
}
