package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/spec-tacles/spectacles.go/util"

	"github.com/gorilla/websocket"
	"github.com/spec-tacles/spectacles.go/types"
)

// Shard represents a shard connected to the Discord gateway
type Shard struct {
	cluster        *Cluster
	conn           *websocket.Conn
	limiter        *time.Ticker
	heartbeater    *time.Ticker
	heartbeatAcked bool
	closeChan      chan struct{}
	mux            sync.Mutex

	ID        int
	Token     string
	Seq       int
	SessionID string
	Logger    *util.Logger
}

// NewShard creates a new shard of an cluster
func NewShard(cluster Cluster, id int, writer io.Writer) *Shard {
	return &Shard{
		cluster:        &cluster,
		limiter:        time.NewTicker(500 * time.Millisecond), // 120 / 60s
		heartbeatAcked: true,
		closeChan:      make(chan struct{}),
		mux:            sync.Mutex{},
		ID:             id,
		Token:          cluster.Token,
		Logger:         util.NewLogger(writer, fmt.Sprintf("[Shard %d]", id)),
	}
}

// Connect this shard to the gateway
func (s *Shard) Connect() error {
	c, _, err := websocket.DefaultDialer.Dial(s.cluster.Gateway.URL, nil)
	if err != nil {
		return err
	}

	s.conn = c
	c.SetCloseHandler(s.closeHandler)
	return s.listen()
}

// Heartbeat sends a heartbeat packet
func (s *Shard) Heartbeat() error {
	return s.Send(types.OpHeartbeat, s.Seq)
}

// Authenticate does either Identify or Resume based on SessionID's availability
func (s *Shard) Authenticate() error {
	if s.SessionID != "" {
		return s.Resume()
	}
	return s.Identify()
}

// Identify identifies this connection with the Discord gateway
func (s *Shard) Identify() error {
	return s.Send(types.OpIdentify, types.Identify{
		Token: s.Token,
		Properties: types.IdentifyProperties{
			OS:      runtime.GOOS,
			Browser: "spectacles.go",
			Device:  "spectacles.go",
		},
		Shard: []int{s.ID, s.cluster.TotalShardCount},
	})
}

// Resume tries to resume an old session
func (s *Shard) Resume() error {
	return s.Send(types.OpResume, types.Resume{
		Token:     s.Token,
		Seq:       s.Seq,
		SessionID: s.SessionID,
	})
}

// Reconnect reconnects to the gateway
func (s *Shard) Reconnect(closeCode int, reason string) error {
	err := s.CloseWithReason(closeCode, reason)
	if err != nil {
		return err
	}
	return s.Connect()
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

func (s *Shard) listen() error {
	messages, errChan := s.readMessages()
	defer s.destroy()

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

func (s *Shard) handleMessage(m *types.ReceivePacket) error {
	switch m.OP {
	// { op: 0, d: { ... }, t: 'MESSAGE_CREATE', s: 1234 }
	case types.OpDispatch:
		switch m.Type {
		case "READY":

			break
		case "RESUMED":
			break
		}
		s.cluster.Dispatch <- &types.GatewayPacket{OP: m.OP, Data: m.Data}
	case types.OpHeartbeat:
		return s.Heartbeat()
	case types.OpReconnect:
		return s.Reconnect(types.CloseUnknownError, "OP 7: RECONNECT")
	case types.OpInvalidSession:
		var resumable bool
		err := json.Unmarshal(m.Data, &resumable)
		if err != nil {
			return err
		}

		if resumable {
			return s.Reconnect(types.CloseUnknownError, "Session Invalidated")
		}

		time.Sleep(time.Duration(rand.Float64()*4+1) * time.Second)
		return s.Reconnect(websocket.CloseNormalClosure, "Session Invalidated")
	case types.OpHello:
		pk := &types.Hello{}
		err := json.Unmarshal(m.Data, pk)
		if err != nil {
			return err
		}

		interval := time.Duration(pk.HeartbeatInterval) * time.Millisecond
		s.setHeartbeat(interval)
		return s.Authenticate()
	case types.OpHeartbeatAck:
		s.heartbeatAcked = true
	}

	return nil
}

func (s *Shard) closeHandler(code int, text string) error {
	switch code {
	case types.CloseAuthenticationFailed, types.CloseShardingRequired, types.CloseInvalidShard:
		// Unrecoverable errors
		return fmt.Errorf("received unrecoverable error code %d", code)
	}
	return s.Connect()
}

func (s *Shard) setHeartbeat(i time.Duration) {
	if s.heartbeater == nil {
		s.heartbeater = time.NewTicker(i)
		go func() {
			for range s.heartbeater.C {
				if s.heartbeatAcked {
					s.Heartbeat()
				} else {
					s.Reconnect(types.CloseUnknownError, "No heartbeat acknowledged")
				}
			}
		}()
	} else {
		s.heartbeater.Stop()
	}
}

func (s *Shard) clearHeartbeat() {
	if s.heartbeater != nil {
		s.heartbeater.Stop()
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

func (s *Shard) destroy() {
	s.limiter.Stop()
	s.clearHeartbeat()
	s.conn.Close()
}
