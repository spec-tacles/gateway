package gateway

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spec-tacles/spectacles/cache"
	"golang.org/x/time/rate"
)

// Shard represents a shard connected to the Discord gateway
type Shard struct {
	conn    *websocket.Conn
	limiter *rate.Limiter

	ID    int
	Token string

	Info cache.ShardInfo
}

func NewShard(id int, token string) *Shard {
	return &Shard{
		conn:    nil,
		limiter: rate.NewLimiter(rate.Every(60*time.Second), 120),

		ID:    id,
		Token: token,
	}
}

// Connect this shard to the gateway
func (s *Shard) Connect() error {
	gateway := s.Info.Gateway()
	c, _, err := websocket.DefaultDialer.Dial(gateway, http.Header{})
	if err != nil {
		return err
	}

	s.conn = c
	return nil
}
