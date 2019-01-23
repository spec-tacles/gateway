package gateway

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spec-tacles/spectacles.go/rest"
	"github.com/spec-tacles/spectacles.go/types"
	"github.com/spec-tacles/spectacles.go/util"
)

// A Cluster of Shards of the Gateway
type Cluster struct {
	rest            *rest.Client
	dispatchHandler func(types.GatewayPacket)

	Token      string
	Shards     map[int]*Shard
	Gateway    *types.GatewayBot
	Writer     *io.Writer
	ShardCount int
	Logger     *util.Logger
	LogLevel   int
}

// ClusterOptions for the Cluster
type ClusterOptions struct {
	ShardCount int
	LogLevel   int
	Writer     io.Writer
}

// NewCluster Creates a new Cluster instance
func NewCluster(token string, dispatchHandler func(types.GatewayPacket), options ClusterOptions) *Cluster {
	return &Cluster{
		dispatchHandler: dispatchHandler,
		rest:            rest.NewClient(token),

		Token:      token,
		Gateway:    &types.GatewayBot{},
		Logger:     util.NewLogger(options.LogLevel, options.Writer, "[Cluster]"),
		Writer:     &options.Writer,
		Shards:     make(map[int]*Shard),
		ShardCount: options.ShardCount,
		LogLevel:   options.LogLevel,
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

	c.Logger.Debug(fmt.Sprintf("Starting %d shards", c.ShardCount))

	for i, shard := range c.Shards {
		c.Logger.Debug(fmt.Sprint("Starting shard ", i))
		shard.Connect()
		c.Logger.Debug(fmt.Sprintf("Shard %d succesfully started", i))

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
		c.Shards[i] = NewShard(ShardInfo{
			Cluster:         c,
			ID:              i,
			DispatchHandler: dispatchHandler,
			ErrorHandler:    errorHandler,
		})
	}
}
