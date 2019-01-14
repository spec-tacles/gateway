package gateway

import (
	"io"
	"net/http"
	"time"

	"github.com/spec-tacles/spectacles.go/rest"
	"github.com/spec-tacles/spectacles.go/types"
)

// A Cluster of Shards of the Gateway
type Cluster struct {
	Token      string
	Shards     map[int]*Shard
	Gateway    *types.GatewayBot
	Logger     io.Writer
	ShardCount int

	rest            *rest.Client
	dispatchHandler func(types.GatewayPacket)
}

// ClusterOptions for the Cluster
type ClusterOptions struct {
	ShardCount int
	Logger     io.Writer
}

// NewCluster Creates a new Cluster instance
func NewCluster(token string, dispatchHandler func(types.GatewayPacket), options ClusterOptions) *Cluster {
	return &Cluster{
		dispatchHandler: dispatchHandler,

		Token:      token,
		Gateway:    &types.GatewayBot{},
		Logger:     options.Logger,
		rest:       rest.NewClient(token),
		Shards:     make(map[int]*Shard),
		ShardCount: options.ShardCount,
	}
}

// Connect all Shards in this Cluster
func (c *Cluster) Connect() error {
	err := c.rest.DoJSON(http.MethodGet, "/gateway/bot", nil, c.Gateway)
	if err != nil {
		return err
	}

	if c.ShardCount == 0 {
		c.ShardCount = c.Gateway.Shards
	}

	errChan := make(chan error)

	c.createShards(c.ShardCount, c.dispatchHandler, func(err error) {
		errChan <- err
	})

	for i, shard := range c.Shards {
		err := shard.Connect()
		if err != nil {
			return err
		}

		if i != len(c.Shards)-1 {
			select {
			case err := <-errChan:
				return err
			case <-time.After(time.Second * 5):
			}
		}
	}

	return <-errChan
}

func (c *Cluster) createShards(shardCount int, dispatchHandler func(types.GatewayPacket), errorHandler func(error)) {
	for i := 0; i < shardCount; i++ {
		c.Shards[i] = NewShard(c, i, c.Logger, dispatchHandler, errorHandler)
	}
}
