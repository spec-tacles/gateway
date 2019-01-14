package gateway

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/spec-tacles/spectacles.go/rest"
	"github.com/spec-tacles/spectacles.go/types"
)

// A Cluster of Shards of the Gateway
type Cluster struct {
	Token           string
	Shards          map[int]*Shard
	Gateway         *types.GatewayBot
	Logger          io.Writer
	DispatchHandler func(types.GatewayPacket)

	identifyLimiter *time.Ticker
	rest            *rest.Client
}

// ClusterOptions for the Cluster
type ClusterOptions struct {
	ShardCount int
	Logger     io.Writer
}

// NewCluster Creates a new Cluster instance
func NewCluster(token string, dispatchHandler func(types.GatewayPacket), options ClusterOptions) *Cluster {
	var cluster = Cluster{
		Token:           token,
		Gateway:         &types.GatewayBot{},
		Logger:          options.Logger,
		identifyLimiter: time.NewTicker(time.Second * 5),
		rest:            rest.NewClient(token),
		Shards:          make(map[int]*Shard),
		DispatchHandler: dispatchHandler,
	}

	if options.ShardCount != 0 {
		cluster.createShards(options.ShardCount, cluster.DispatchHandler)
	}

	return &cluster
}

// Connect all Shards in this Cluster
func (c Cluster) Connect() error {
	err := c.rest.DoJSON(http.MethodGet, "/gateway/bot", nil, c.Gateway)

	if err != nil {
		return err
	}

	if len(c.Shards) == 0 {
		c.createShards(c.Gateway.Shards, c.DispatchHandler)
	}

	wg := sync.WaitGroup{}
	for _, element := range c.Shards {
		wg.Add(1)
		go func(e *Shard) {
			defer wg.Done()

			<-c.identifyLimiter.C
			err := e.Connect()
			if err != nil {
				panic(err)
			}
		}(element)
	}

	wg.Wait()
	return nil
}

func (c Cluster) createShards(shardCount int, dispatchHandler func(types.GatewayPacket)) {
	for i := 0; i < shardCount; i++ {
		c.Shards[i] = NewShard(c, i, c.Logger, dispatchHandler)
	}
}
