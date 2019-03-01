package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spec-tacles/spectacles.go/gateway"
	"github.com/spec-tacles/spectacles.go/rest"
	"github.com/spec-tacles/spectacles.go/types"
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
		REST: rest.NewClient(token),
		OnPacket: func(shard int, r *types.ReceivePacket) {
			fmt.Printf("Received op %d, event %s, and seq %d on shard %d\n", r.Op, r.Event, r.Seq, shard)
		},
		LogLevel: gateway.LogLevelInfo,
	})

	if err := m.Start(); err != nil {
		log.Panicf("failed to start: %v", err)
	}

	select {}
}
