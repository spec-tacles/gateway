package gateway

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spec-tacles/gateway/compression"
	"github.com/spec-tacles/gateway/stats"
	"github.com/spec-tacles/go/types"
)

// Shard represents a Gateway shard
type Shard struct {
	Gateway *types.GatewayBot
	Ping    time.Duration

	conn *Connection

	id            string
	opts          *ShardOptions
	limiter       Limiter
	reopening     atomic.Value
	packets       *sync.Pool
	lastHeartbeat time.Time

	connMu sync.Mutex

	sessionID string
	acks      chan struct{}
	seq       *uint64
}

// NewShard creates a new Gateway shard
func NewShard(opts *ShardOptions) *Shard {
	opts.init()

	return &Shard{
		opts:    opts,
		limiter: NewDefaultLimiter(120, time.Minute),
		packets: &sync.Pool{
			New: func() interface{} {
				return new(types.ReceivePacket)
			},
		},
		id:   strconv.Itoa(opts.Identify.Shard[0]),
		seq:  new(uint64),
		acks: make(chan struct{}),
	}
}

// Open starts a new session. Any errors are fatal.
func (s *Shard) Open() (err error) {
	err = s.connect()
	for s.handleClose(err) {
		err = s.connect()
	}
	return
}

// connect runs a single websocket connection; errors may indicate the connection is recoverable
func (s *Shard) connect() (err error) {
	if s.Gateway == nil {
		return ErrGatewayAbsent
	}

	url := s.gatewayURL()
	s.log(LogLevelInfo, "Connecting using URL: %s", url)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return
	}
	s.conn = NewConnection(conn, compression.NewZstd())

	stop := make(chan struct{}, 0)
	defer close(stop)

	err = s.expectPacket(types.GatewayOpHello, types.GatewayEventNone, s.handleHello(stop))
	if err != nil {
		return
	}

	seq := atomic.LoadUint64(s.seq)
	s.log(LogLevelDebug, "session \"%s\", seq %d", s.sessionID, seq)
	if s.sessionID == "" && seq == 0 {
		if err = s.sendIdentify(); err != nil {
			return
		}

		s.log(LogLevelDebug, "Sent identify upon connecting")

		var handleReady func(p *types.ReceivePacket) error
		handleReady = func(p *types.ReceivePacket) (err error) {
			switch p.Op {
			case types.GatewayOpDispatch:
			case types.GatewayOpInvalidSession:
				// if we encounter an invalid session here, we just need to keep listening for ready events
				// since the normal packet handler will handle re-identification
				err = s.readPacket(handleReady)
			}
			return
		}
		err = s.readPacket(handleReady)
		if err != nil {
			return
		}

		s.log(LogLevelInfo, "received ready event")
	} else {
		if err = s.sendResume(); err != nil {
			return
		}

		s.log(LogLevelDebug, "Sent resume upon connecting")
	}

	// mark shard as alive
	stats.ShardsAlive.WithLabelValues(s.id).Inc()
	s.log(LogLevelDebug, "beginning normal message consumption")
	for {
		err = s.readPacket(nil)
		if err != nil {
			break
		}
	}

	return
}

// CloseWithReason closes the connection and logs the reason
func (s *Shard) CloseWithReason(code int, reason error) error {
	s.log(LogLevelWarn, "%s: closing connection", reason)
	return s.conn.CloseWithCode(code)
}

// Close closes the current session
func (s *Shard) Close() (err error) {
	if err = s.conn.Close(); err != nil {
		return
	}

	s.log(LogLevelInfo, "Cleanly closed connection")
	return
}

func (s *Shard) readPacket(fn func(*types.ReceivePacket) error) (err error) {
	d, err := s.conn.Read()
	if err != nil {
		return
	}

	p := s.packets.Get().(*types.ReceivePacket)
	defer s.packets.Put(p)

	err = json.Unmarshal(d, p)
	if err != nil {
		return
	}
	s.log(LogLevelDebug, "received packet (%d) %s", p.Op, p.Event)

	if p.Event != "" {
		p.Op = types.GatewayOpDispatch
	}

	// record packet received
	stats.PacketsReceived.WithLabelValues(string(p.Event), strconv.Itoa(int(p.Op)), s.id).Inc()

	if s.opts.OnPacket != nil {
		s.opts.OnPacket(p)
	}

	err = s.handlePacket(p)
	if err != nil {
		return
	}

	if fn != nil {
		err = fn(p)
	}
	return
}

// expectPacket reads the next packet, verifies its operation code, and event name (if applicable)
func (s *Shard) expectPacket(op types.GatewayOp, event types.GatewayEvent, handler func(*types.ReceivePacket) error) (err error) {
	err = s.readPacket(func(pk *types.ReceivePacket) error {
		if pk.Op != op {
			return fmt.Errorf("expected op to be %d, got %d", op, pk.Op)
		}

		if op == types.GatewayOpDispatch && pk.Event != event {
			return fmt.Errorf("expected event to be %s, got %s", event, pk.Event)
		}

		if handler != nil {
			return handler(pk)
		}

		return nil
	})

	return
}

// handlePacket handles a packet according to its operation code
func (s *Shard) handlePacket(p *types.ReceivePacket) (err error) {
	switch p.Op {
	case types.GatewayOpDispatch:
		return s.handleDispatch(p)

	case types.GatewayOpHeartbeat:
		return s.sendHeartbeat()

	case types.GatewayOpReconnect:
		if err = s.CloseWithReason(types.CloseUnknownError, ErrReconnectReceived); err != nil {
			return
		}

	case types.GatewayOpInvalidSession:
		resumable := new(bool)
		if err = json.Unmarshal(p.Data, resumable); err != nil {
			return
		}

		if *resumable {
			if err = s.sendResume(); err != nil {
				return
			}

			s.log(LogLevelDebug, "Sent resume in response to invalid resumable session")
			return
		}

		time.Sleep(time.Second * time.Duration(rand.Intn(5)+1))
		if err = s.sendIdentify(); err != nil {
			return
		}

		s.log(LogLevelDebug, "Sent identify in response to invalid non-resumable session")

	case types.GatewayOpHeartbeatACK:
		if s.lastHeartbeat.Unix() != 0 {
			// record latest gateway ping
			s.Ping = time.Now().Sub(s.lastHeartbeat)
			stats.Ping.WithLabelValues(s.id).Observe(float64(s.Ping.Nanoseconds()) / 1e6)
		}
		s.acks <- struct{}{}
	}

	return
}

// handleDispatch handles dispatch packets
func (s *Shard) handleDispatch(p *types.ReceivePacket) (err error) {
	switch p.Event {
	case types.GatewayEventReady:
		r := new(types.Ready)
		if err = json.Unmarshal(p.Data, r); err != nil {
			return
		}

		s.sessionID = r.SessionID

		s.log(LogLevelDebug, "Using version: %d", r.Version)
		s.logTrace(r.Trace)

	case types.GatewayEventResumed:
		r := new(types.Resumed)
		if err = json.Unmarshal(p.Data, r); err != nil {
			return
		}

		s.logTrace(r.Trace)
	}

	return
}

func (s *Shard) handleHello(stop chan struct{}) func(*types.ReceivePacket) error {
	return func(p *types.ReceivePacket) (err error) {
		h := new(types.Hello)
		if err = json.Unmarshal(p.Data, h); err != nil {
			return
		}

		s.logTrace(h.Trace)
		go s.startHeartbeater(time.Duration(h.HeartbeatInterval)*time.Millisecond, stop)
		return
	}
}

// handleClose handles the WebSocket close event. Returns whether the session is recoverable.
func (s *Shard) handleClose(err error) (recoverable bool) {
	// mark shard as offline
	stats.ShardsAlive.WithLabelValues(s.id).Dec()

	recoverable = !websocket.IsCloseError(err, types.CloseAuthenticationFailed, types.CloseInvalidShard, types.CloseShardingRequired)
	if recoverable {
		s.log(LogLevelInfo, "recoverable close: %s", err)
	} else {
		s.log(LogLevelInfo, "unrecoverable close: %s", err)
	}
	return
}

// SendPacket sends a packet
func (s *Shard) SendPacket(op types.GatewayOp, data interface{}) error {
	s.log(LogLevelDebug, "sending packet (%d)", op)
	return s.Send(&types.SendPacket{
		Op:   op,
		Data: data,
	})
}

// Send sends a pre-prepared packet
func (s *Shard) Send(p *types.SendPacket) error {
	d, err := json.Marshal(p)
	if err != nil {
		return err
	}

	s.limiter.Lock()
	s.connMu.Lock()
	defer s.connMu.Unlock()

	// record packet sent
	defer stats.PacketsSent.WithLabelValues("", strconv.Itoa(int(p.Op)), s.id).Inc()

	_, err = s.conn.Write(d)
	return err
}

// sendIdentify sends an identify packet
func (s *Shard) sendIdentify() error {
	s.opts.IdentifyLimiter.Lock()
	return s.SendPacket(types.GatewayOpIdentify, s.opts.Identify)
}

// sendResume sends a resume packet
func (s *Shard) sendResume() error {
	return s.SendPacket(types.GatewayOpResume, &types.Resume{
		Token:     s.opts.Identify.Token,
		SessionID: s.sessionID,
		Seq:       types.Seq(atomic.LoadUint64(s.seq)),
	})
}

// sendHeartbeat sends a heartbeat packet
func (s *Shard) sendHeartbeat() error {
	s.lastHeartbeat = time.Now()
	return s.SendPacket(types.GatewayOpHeartbeat, atomic.LoadUint64(s.seq))
}

// startHeartbeater calls sendHeartbeat on the provided interval
func (s *Shard) startHeartbeater(interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	acked := true

	s.log(LogLevelInfo, "starting heartbeat at interval %d/s", interval/time.Second)
	for {
		select {
		case <-s.acks:
			acked = true
		case <-t.C:
			if !acked {
				s.CloseWithReason(types.CloseSessionTimeout, ErrHeartbeatUnacknowledged)
				return
			}

			if err := s.sendHeartbeat(); err != nil {
				s.log(LogLevelError, "error sending automatic heartbeat: %s", err)
				return
			}
			acked = false

		case <-stop:
			return
		}
	}
}

// gatewayURL returns the Gateway URL with appropriate query parameters
func (s *Shard) gatewayURL() string {
	query := url.Values{
		"v":        {s.opts.Version},
		"encoding": {"json"},
		"compress": {"zstd-stream"},
	}

	return s.Gateway.URL + "/?" + query.Encode()
}
