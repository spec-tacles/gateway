package gateway

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spec-tacles/spectacles.go/rest"
	"github.com/spec-tacles/spectacles.go/types"
)

// Shard represents a shard connected to the Discord gateway
type Shard struct {
	conn            *websocket.Conn
	limiter         *time.Timer
	identifyLimiter *time.Timer
	messages        chan types.ReceivePacket
	heartbeater     *time.Timer
	heartbeatAcked  bool
	mux             sync.Mutex

	ID    int
	Count int
	Token string
	Rest  *rest.Client
	Seq   int
}

// NewShard makes a new shard
func NewShard(token string, id int) *Shard {
	return &Shard{
		limiter:         time.NewTimer(120 / time.Minute), // 120 / 60s
		identifyLimiter: time.NewTimer(time.Second / 5),   // 1 / 5s
		messages:        make(chan types.ReceivePacket),
		heartbeatAcked:  true,
		mux:             sync.Mutex{},
		ID:              id,
		Token:           token,
		Rest:            rest.NewClient(token),
	}
}

// Connect this shard to the gateway
func (s *Shard) Connect() error {
	var gateway *types.GatewayBot
	err := s.Rest.DoJSON(http.MethodGet, "/gateway/bot", nil, gateway)

	c, _, err := websocket.DefaultDialer.Dial(gateway.URL, nil)
	if err != nil {
		return err
	}
	s.conn = c

	if s.Count < 1 {
		s.Count = gateway.Shards
	}

	go s.listen()
	return nil
}

// Heartbeat sends a heartbeat packet
func (s *Shard) Heartbeat() error {
	return s.Send(types.OpHeartbeat, s.Seq)
}

// Identify identifies this connection with the Discord gateway
func (s *Shard) Identify() error {
	<-s.identifyLimiter.C
	return s.Send(types.OpIdentify, types.Identify{
		Token: s.Token,
		Properties: types.IdentifyProperties{
			OS:      runtime.GOOS,
			Browser: "spectacles.go",
			Device:  "spectacles.go",
		},
		Shard: []int{s.ID, s.Count},
	})
}

// Reconnect reconnects to the gateway
func (s *Shard) Reconnect() error {
	s.Close()
	s.Connect()
	return nil
}

// Send a packet to the gateway
func (s *Shard) Send(op int, d interface{}) error {
	<-s.limiter.C
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.conn.WriteJSON(&types.SendPacket{
		OP: op,
		D:  d,
	})
}

// Close this shard connection
func (s *Shard) Close() {
	close(s.messages)
}

func (s *Shard) listen() {
	go s.readMessages()
	defer s.cleanup()

	var m types.ReceivePacket
	for m = range s.messages {
		switch m.OP {
		case types.OpReconnect:
			s.Reconnect()
		case types.OpHello:
			pk := &types.Hello{}
			json.Unmarshal(m.D, pk)

			interval := time.Duration(pk.HeartbeatInterval) * time.Millisecond
			s.setHeartbeat(interval)
			s.Identify()
		case types.OpHeartbeat:
			s.Heartbeat()
		case types.OpHeartbeatAck:
			s.heartbeatAcked = true
		}
	}
}

func (s *Shard) cleanup() {
	s.limiter.Stop()
	s.identifyLimiter.Stop()
	s.clearHeartbeat()
	s.conn.Close()
}

func (s *Shard) setHeartbeat(i time.Duration) {
	if s.heartbeater == nil {
		s.heartbeater = time.NewTimer(i)
		go func() {
			for range s.heartbeater.C {
				if s.heartbeatAcked {
					s.Heartbeat()
				} else {
					s.Reconnect()
				}
			}
		}()
	} else {
		s.heartbeater.Reset(i)
	}
}

func (s *Shard) clearHeartbeat() {
	if s.heartbeater != nil {
		s.heartbeater.Stop()
	}
}

func (s *Shard) readMessages() {
	var err error
	for {
		var payload types.ReceivePacket
		err = s.conn.ReadJSON(payload)
		if err != nil {
			s.Close()
			break
		}

		s.messages <- payload
	}
}
