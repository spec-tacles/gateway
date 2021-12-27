package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spec-tacles/gateway/gateway"
	"github.com/spec-tacles/go/rest"
	"github.com/spec-tacles/go/types"
)

var token = os.Getenv("TOKEN")

func main() {
	m := gateway.NewManager(&gateway.ManagerOptions{
		ShardOptions: &gateway.ShardOptions{
			Identify: &types.Identify{
				Token: token,
			},
			LogLevel: gateway.LogLevelDebug,
		},
		REST: rest.NewClient(token, "9"),
		OnPacket: func(shard int, r *types.ReceivePacket) {
			fmt.Printf("Received op %d, event %s, and seq %d on shard %d\n", r.Op, r.Event, r.Seq, shard)
		},
		LogLevel: gateway.LogLevelInfo,
	})

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		log.Panicf("failed to start: %v", err)
	}

	select {}
}
