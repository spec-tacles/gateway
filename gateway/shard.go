package gateway

import (
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spec-tacles/go/types"
)

// Shard represents a Gateway shard
type Shard struct {
	Gateway *types.GatewayBot

	opts      *ShardOptions
	limiter   *limiter
	reopening atomic.Value

	connMu sync.Mutex
	conn   *websocket.Conn

	sessionID string
	acked     atomic.Value
	seq       *uint64
}

// NewShard creates a new Gateway shard
func NewShard(opts *ShardOptions) *Shard {
	opts.init()

	return &Shard{
		opts:    opts,
		limiter: newLimiter(120, time.Minute),
		seq:     new(uint64),
	}
}

// Open starts a new session
func (s *Shard) Open() (err error) {
	if s.Gateway == nil {
		return ErrGatewayAbsent
	}

	url := s.gatewayURL()
	s.log(LogLevelInfo, "Connecting using URL: %s", url)

	s.conn, _, err = websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return
	}

	s.conn.SetCloseHandler(s.handleClose)

	p, err := s.expectPacket(types.GatewayOpHello, types.GatewayEventNone)
	if err != nil {
		return
	}

	h := new(types.Hello)
	if err = json.Unmarshal(p.Data, h); err != nil {
		return
	}

	s.logTrace(h.Trace)

	if err = s.handlePacket(p); err != nil {
		return
	}

	stop := make(chan struct{}, 0)
	stoppable := false

	defer func() {
		if !stoppable {
			close(stop)
		}
	}()

	go func(stop <-chan struct{}) {
		if err := s.startHeartbeater(time.Duration(h.HeartbeatInterval)*time.Millisecond, stop); err != nil {
			if err == ErrHeartbeatUnacknowledged {
				s.log(LogLevelInfo, "Heartbeat was not acknowledged in time, reconnecting")
				if err := s.reopen(); err != nil {
					s.log(LogLevelError, "Failed to reconnect: %v", err)
				}
			} else {
				s.log(LogLevelError, "Heartbeat error: %v", err)
			}
		}
	}(stop)

	if s.sessionID == "" && atomic.LoadUint64(s.seq) == 0 {
		if err = s.sendIdentify(); err != nil {
			return
		}

		s.log(LogLevelDebug, "Sent identify upon connecting")

		p, err = s.expectPacket(types.GatewayOpDispatch, types.GatewayEventReady)
		if err != nil {
			return
		}

		if err = s.handlePacket(p); err != nil {
			return
		}
	} else {
		if err = s.sendResume(); err != nil {
			return
		}

		s.log(LogLevelDebug, "Sent resume upon connecting")
	}

	stoppable = true

	go func(stop chan struct{}) {
		packetChan := make(chan *types.ReceivePacket)
		go func() {
			defer close(stop)

			for {
				p, err := s.readPacket()
				if err != nil {
					if err != io.EOF && err != io.ErrUnexpectedEOF {
						s.log(LogLevelError, "Error while reading packet: %v", err)
					}

					close(packetChan)
					return
				}

				packetChan <- p
			}
		}()

		conn := s.conn

		for p := range packetChan {
			if err := s.handlePacket(p); err != nil {
				s.log(LogLevelError, "Error while handling packet: %v", err)
			}
		}

		if s.conn == conn {
			if err := s.reopen(); err != nil {
				s.log(LogLevelError, "Failed to reconnect: %v", err)
			}
		}
	}(stop)

	return
}

// Close closes the current session
func (s *Shard) Close() (err error) {
	s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Normal Closure"))

	if err = s.conn.Close(); err != nil {
		return
	}

	s.log(LogLevelInfo, "Cleanly closed connection")
	return
}

// reopen closes and opens the connection
func (s *Shard) reopen() (err error) {
	reopening := s.reopening.Load()
	if reopening == nil || reopening.(bool) {
		return
	}

	s.reopening.Store(true)
	defer s.reopening.Store(false)

	if err = s.Close(); err != nil {
		return
	}

	timeout := s.opts.Retryer.FirstTimeout()
	retries := 0

	for {
		if timeout, err = s.opts.Retryer.NextTimeout(timeout, retries); err != nil {
			return
		}

		s.log(LogLevelInfo, "Reconnect attempt #%d", retries)

		err = s.Open()
		if err == nil {
			return
		}

		s.log(LogLevelError, "Error while reconnecting: %v", err)

		time.Sleep(timeout)
		retries++
	}
}

// readPacket reads the next packet
func (s *Shard) readPacket() (p *types.ReceivePacket, err error) {
	m, r, err := s.conn.NextReader()
	if err != nil {
		return
	}

	// TODO: Make zlib streams work (it's contextless atm)
	if m == websocket.BinaryMessage {
		z, err := zlib.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer z.Close()

		r = z
	}

	if s.opts.Output != nil {
		r = io.TeeReader(r, s.opts.Output)
	}

	p = new(types.ReceivePacket)
	if err = json.NewDecoder(r).Decode(p); err != nil {
		return
	}

	if s.opts.OnPacket != nil {
		go s.opts.OnPacket(p)
	}

	return
}

// expectPacket reads the next packet, verifies its operation code, and event name (if applicable)
func (s *Shard) expectPacket(op types.GatewayOp, event types.GatewayEvent) (p *types.ReceivePacket, err error) {
	p, err = s.readPacket()
	if err != nil {
		return
	}

	if p.Op != op {
		return p, fmt.Errorf("expected op to be %d, got: %d", op, p.Op)
	}

	if op == types.GatewayOpDispatch && p.Event != event {
		return p, fmt.Errorf("expected event to be %s, got: %s", event, p.Event)
	}

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
		s.log(LogLevelDebug, "Attempting to reconnect session in response to reconnect")

		if err = s.reopen(); err != nil {
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
		s.acked.Store(true)
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

// handleClose handles the WebSocket close event
func (s *Shard) handleClose(code int, reason string) (err error) {
	level := LogLevelError
	if code == websocket.CloseNormalClosure {
		level = LogLevelInfo
	}

	s.log(level, "Server closed connection with code %d and reason %s", code, reason)

	switch code {
	case types.CloseAuthenticationFailed, types.CloseInvalidShard, types.CloseShardingRequired: // Fatal error codes
		return
	}

	return s.reopen()
}

// sendPacket sends a packet
func (s *Shard) sendPacket(op types.GatewayOp, data interface{}) error {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	s.limiter.lock()

	return s.conn.WriteJSON(&types.SendPacket{
		Op:   op,
		Data: data,
	})
}

// sendIdentify sends an identify packet
func (s *Shard) sendIdentify() error {
	// TODO: rate limit identify packets
	return s.sendPacket(types.GatewayOpIdentify, s.opts.Identify)
}

// sendResume sends a resume packet
func (s *Shard) sendResume() error {
	return s.sendPacket(types.GatewayOpResume, &types.Resume{
		Token:     s.opts.Identify.Token,
		SessionID: s.sessionID,
		Seq:       types.Seq(atomic.LoadUint64(s.seq)),
	})
}

// sendHeartbeat sends a heartbeat packet
func (s *Shard) sendHeartbeat() error {
	acked := s.acked.Load()
	if acked != nil && !acked.(bool) {
		return ErrHeartbeatUnacknowledged
	}

	s.acked.Store(true)

	return s.sendPacket(types.GatewayOpHeartbeat, atomic.LoadUint64(s.seq))
}

// startHeartbeater calls sendHeartbeat on the provided interval
func (s *Shard) startHeartbeater(interval time.Duration, stop <-chan struct{}) (err error) {
	t := time.NewTicker(interval)

	for {
		select {
		case <-t.C:
			if err = s.sendHeartbeat(); err != nil {
				return
			}

		case <-stop:
			t.Stop()
			return
		}
	}
}

// gatewayURL returns the Gateway URL with appropriate query parameters
func (s *Shard) gatewayURL() string {
	query := url.Values{
		"v":        {s.opts.Version},
		"encoding": {"json"},
		// "compress": {"zlib-stream"},
	}

	return s.Gateway.URL + "/?" + query.Encode()
}
