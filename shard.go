package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spec-tacles/spectacles.go/types"
	"github.com/spec-tacles/spectacles.go/util"
)

// Shard represents a shard connected to the Discord gateway
type Shard struct {
	conn            *websocket.Conn
	limiter         *time.Ticker
	heartbeater     *time.Ticker
	heartbeatAcked  bool
	closeChan       chan struct{}
	mux             sync.Mutex
	dispatchHandler func(types.ReceivePacket)
	errorHandler    func(error)
	gateway         string
	shardCount      int

	Cluster   *Cluster
	ID        int
	Token     string
	Seq       int
	SessionID string
	Logger    *util.Logger
	Trace     []string
}

// ShardInfo holds information to create a Shard
type ShardInfo struct {
	Token           string
	ID              int
	Logger          io.Writer
	DispatchHandler func(types.ReceivePacket)
	ErrorHandler    func(error)
	Cluster         *Cluster
	ShardCount      int
	Gateway         string
	LogLevel        int
}

// NewShard creates a new Shard
func NewShard(info ShardInfo) *Shard {
	var token string
	var shardCount int
	var logLevel int
	var logger io.Writer
	if info.Cluster == nil {
		token = info.Token
		shardCount = info.ShardCount
		logLevel = info.LogLevel
		logger = info.Logger
	} else {
		token = info.Cluster.Token
		shardCount = info.Cluster.ShardCount
		logLevel = info.Cluster.LogLevel
		logger = info.Cluster.Logger.Destination
	}

	return &Shard{
		limiter:         time.NewTicker(500 * time.Millisecond), // 120 / 60s
		heartbeatAcked:  true,
		mux:             sync.Mutex{},
		dispatchHandler: info.DispatchHandler,
		errorHandler:    info.ErrorHandler,
		gateway:         info.Gateway,
		shardCount:      shardCount,
		closeChan:       make(chan struct{}),
		ID:              info.ID,
		Token:           token,
		Logger:          util.NewLogger(logLevel, logger, fmt.Sprintf("[Shard %d]", info.ID)),
		Seq:             0,
		Cluster:         info.Cluster,
	}
}

// SetDispatchHandler sets the current callback function of this Shard
func (s *Shard) SetDispatchHandler(dispatchHandler func(types.ReceivePacket)) {
	s.dispatchHandler = dispatchHandler
}

// SetErrorHandler sets the current error handler function of this Shard
func (s *Shard) SetErrorHandler(errorHandler func(error)) {
	s.errorHandler = errorHandler
}

// Connect this shard to the gateway
func (s *Shard) Connect() {
	s.Logger.Debug("Connecting to Websocket...")

	s.heartbeatAcked = true

	var gateway string
	if s.Cluster != nil {
		gateway = s.Cluster.Gateway.URL
	} else {
		gateway = s.gateway
	}

	c, _, err := websocket.DefaultDialer.Dial(gateway+"?v=7", nil)

	if err != nil {
		s.Logger.Error("Connection to Websocket errored, retrying...")
		s.Connect()
		return
	}

	go func() {
		err := s.listen()
		if err != nil {
			s.Reconnect(types.CloseUnknownError, "Error in WS Connection")
		}
	}()

	s.conn = c
	c.SetCloseHandler(s.closeHandler)
}

// Heartbeat sends a heartbeat packet
func (s *Shard) Heartbeat() error {
	s.Logger.Debug("Sending a heartbeat")
	return s.Send(types.OpHeartbeat, s.Seq)
}

// Authenticate does either Identify or Resume based on SessionID's availability
func (s *Shard) Authenticate() error {
	s.Logger.Debug("Authenticate to Discord")
	if s.SessionID != "" {
		return s.Resume()
	}
	return s.Identify()
}

// Identify identifies this connection with the Discord gateway
func (s *Shard) Identify() error {
	s.Logger.Debug("Identifying as a new session")

	shardCount := s.shardCount
	if shardCount == 0 {
		shardCount = len(s.Cluster.Shards)
	}

	return s.Send(types.OpIdentify, types.Identify{
		Token: s.Token,
		Properties: types.IdentifyProperties{
			OS:      runtime.GOOS,
			Browser: "spectacles.go",
			Device:  "spectacles.go",
		},
		Shard: []int{s.ID, shardCount},
	})
}

// Resume tries to resume an old session
func (s *Shard) Resume() error {
	s.Logger.Debug(fmt.Sprintf("Attempting to resume session %s", s.SessionID))
	return s.Send(types.OpResume, types.Resume{
		Token:     s.Token,
		Seq:       s.Seq,
		SessionID: s.SessionID,
	})
}

// Reconnect reconnects to the gateway
func (s *Shard) Reconnect(closeCode int, reason string) {
	s.Logger.Warn(fmt.Sprintf("Reconnecting to Gateway with CloseReason %d: %s", closeCode, reason))
	if s.conn != nil {
		// We ignore the error since the lib doesn't support checking for ws status :angeryboye:
		s.CloseWithReason(closeCode, reason)
		s.conn.Close()
	}
	s.Connect()
}

// Send a packet to the gateway
func (s *Shard) Send(op int, d interface{}) error {
	<-s.limiter.C
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.conn.WriteJSON(&types.SendPacket{
		OP:   op,
		Data: d,
	})
}

// CloseWithReason closes the websocket connection while sending the CloseFrame before
func (s *Shard) CloseWithReason(closeCode int, reason string) error {
	err := s.SendCloseFrame(closeCode, reason)
	if err != nil {
		return err
	}
	return s.Close()
}

// SendCloseFrame sends a close frame to the websocket
func (s *Shard) SendCloseFrame(closeCode int, reason string) error {
	pk := websocket.FormatCloseMessage(closeCode, reason)
	return s.conn.WriteMessage(websocket.CloseMessage, pk)
}

// Close implements io.Closer
func (s *Shard) Close() error {
	s.closeChan <- struct{}{}
	return nil
}

func (s *Shard) handleMessage(m *types.ReceivePacket) error {
	switch m.OP {
	case types.OpDispatch:
		switch m.Type {
		case "READY":
			var pkt types.Ready
			var err = json.Unmarshal(m.Data, &pkt)
			if err != nil {
				return err
			}
			s.SessionID = pkt.SessionID
			s.Trace = pkt.Trace
			s.Logger.Debug(fmt.Sprintf("READY %s -> %s %s", s.Trace[0], s.Trace[1], s.SessionID))
		case "RESUMED":
			var pkt types.Resumed
			var err = json.Unmarshal(m.Data, &pkt)
			if err != nil {
				return err
			}
			s.Trace = pkt.Trace
			s.Logger.Debug(fmt.Sprintf("RESUMED %s -> %s %s | replayed %d events.", s.Trace[0], s.Trace[1], s.SessionID, m.Seq-s.Seq))
		}
		if m.Seq > s.Seq {
			s.Seq = m.Seq
		}
		if s.dispatchHandler == nil {
			s.Logger.Error("No Callback for Dispatches registered")
			return nil
		}
		s.Logger.Debug(fmt.Sprintf("Received Dispatch of type %s", m.Type))
		s.dispatchHandler(*m)
	case types.OpHeartbeat:
		s.Logger.Debug(fmt.Sprintf("Received Keep-Alive request  (OP %d). Sending response...", types.OpHeartbeat))
		return s.Heartbeat()
	case types.OpReconnect:
		s.Logger.Debug(fmt.Sprintf("Received Reconnect request (OP %d). Closing connection now...", types.OpReconnect))
		s.Reconnect(types.CloseUnknownError, "OP 7: RECONNECT")
		return nil
	case types.OpInvalidSession:
		s.Logger.Debug(fmt.Sprintf("Received Invalidate request (OP %d). Invalidating....", types.OpInvalidSession))
		var resumable bool
		err := json.Unmarshal(m.Data, &resumable)
		if err != nil {
			return err
		}

		if resumable {
			s.Reconnect(types.CloseUnknownError, "Session Invalidated")
			return nil
		}

		s.SessionID = ""
		time.Sleep(time.Duration(rand.Float64()*4+1) * time.Second)
		s.Reconnect(websocket.CloseNormalClosure, "Session Invalidated")
		return nil
	case types.OpHello:
		s.Logger.Debug(fmt.Sprintf("Received HELLO packet (OP %d). Initializing keep-alive...", types.OpHello))
		pk := &types.Hello{}
		err := json.Unmarshal(m.Data, pk)

		if err != nil {
			return err
		}

		interval := time.Duration(pk.HeartbeatInterval) * time.Millisecond
		s.configureHeartbeat(&interval)
		s.Trace = pk.Trace
		return s.Authenticate()
	case types.OpHeartbeatAck:
		s.heartbeatAcked = true
		s.Logger.Debug(fmt.Sprintf("Received Heartbeat Ack (OP %d)", types.OpHeartbeatAck))
	default:
		s.Logger.Debug(fmt.Sprintf("Received unknown op-code: %d", m.OP))
	}

	return nil
}

func (s *Shard) listen() error {
	messages, errChan := s.readMessages()

	for {
		select {
		case m := <-messages:
			err := s.handleMessage(m)
			if err != nil {
				return err
			}

		case err := <-errChan:
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}

			return err

		case <-s.closeChan:
			return nil
		}
	}
}

func (s *Shard) readMessages() (<-chan *types.ReceivePacket, <-chan error) {
	messages, errChan := make(chan *types.ReceivePacket), make(chan error)

	go func() {
		for {
			payload := &types.ReceivePacket{}
			err := s.conn.ReadJSON(payload)
			if err != nil {
				errChan <- err
				break
			}

			messages <- payload
		}
	}()

	return messages, errChan
}

func (s *Shard) closeHandler(code int, text string) error {
	s.conn = nil
	switch code {
	case types.CloseAuthenticationFailed, types.CloseShardingRequired, types.CloseInvalidShard:
		// Unrecoverable errors
		msg := fmt.Sprintf("Websocket disconnected with unrecoverable code %d: %s, disconnecting...", code, text)
		s.Logger.Error(msg)
		s.errorHandler(fmt.Errorf(msg))
		return nil
	}
	s.Logger.Debug(fmt.Sprintf("Websocket disconnected with code %d: %s, attempting to reconnect and resume...", code, text))
	s.Connect()

	return nil
}

func (s *Shard) configureHeartbeat(i *time.Duration) {
	if s.heartbeater != nil {
		s.heartbeater.Stop()
	}

	if i == nil {
		return
	}

	s.Logger.Debug(fmt.Sprintf("Setting Heartbeat interval to %s", i))
	s.heartbeater = time.NewTicker(*i)
	go func() {
		for range s.heartbeater.C {
			if s.heartbeatAcked {
				s.Heartbeat()
				s.heartbeatAcked = false
			} else {
				s.Logger.Debug("Received no heartbeat acknowledged in time, assuming zombie connection.")
				s.Reconnect(types.CloseUnknownError, "No heartbeat acknowledged")
			}
		}
	}()
}

func (s *Shard) destroy() {
	s.limiter.Stop()
	s.configureHeartbeat(nil)
	s.conn.Close()
}
