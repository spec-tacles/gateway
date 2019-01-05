package gateway

import (
	"net/http"
	"sync"
	"time"

	"github.com/spec-tacles/spectacles.go/rest"
	"github.com/spec-tacles/spectacles.go/types"
	"github.com/spec-tacles/spectacles.go/util"
)

// A Cluster of Shards of the Gateway
type Cluster struct {
	Token           string
	Shards          map[int]*Shard
	TotalShardCount int
	Gateway         *types.GatewayBot
	Logger          *util.Logger

	identifyLimiter *time.Ticker
	rest            *rest.Client
}

// ClusterOptions for the Cluster
type ClusterOptions struct {
	ShardCount      int
	TotalShardCount int
	Shards          []int
}

// NewCluster Creates a new Cluster instance
func NewCluster(token string, options ClusterOptions) *Cluster {
	var cluster = Cluster{
		Token:           token,
		Gateway:         &types.GatewayBot{},
		identifyLimiter: time.NewTicker(time.Second / 5),
		rest:            rest.NewClient(token),
	}

	if options.Shards != nil {
		cluster.Shards = make(map[int]*Shard)
		for index, element := range options.Shards {
			cluster.Shards[index] = NewShard(cluster, element)
		}

		cluster.TotalShardCount = options.TotalShardCount
	} else if options.ShardCount != 0 {
		cluster.Shards = make(map[int]*Shard)
		for i := 0; i < options.ShardCount; i++ {
			cluster.Shards[i] = NewShard(cluster, i)
		}

		if options.TotalShardCount != 0 {
			cluster.TotalShardCount = options.ShardCount
		}
	}

	return &cluster
}

// Connect all Shards in this Cluster
func (c Cluster) Connect() error {
	err := c.rest.DoJSON(http.MethodGet, "/gateway/bot", nil, c.Gateway)

	if err != nil {
		return err
	}

	if c.Shards == nil {
		c.Shards = make(map[int]*Shard)
		c.TotalShardCount = c.Gateway.Shards
		for i := 0; i < c.Gateway.Shards; i++ {
			c.Shards[i] = NewShard(c, i)
		}
	}

	wg := sync.WaitGroup{}
	for _, element := range c.Shards {
		go func(e *Shard) {
			wg.Add(1)
			defer wg.Done()

			<-c.identifyLimiter.C
			err := e.Connect()
			if err != nil {

			}
		}(element)
	}

	wg.Wait()
	return nil
}
