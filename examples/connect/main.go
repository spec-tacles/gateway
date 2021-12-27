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
	c := gateway.NewShard(&gateway.ShardOptions{
		Identify: &types.Identify{
			Token: token,
		},
		OnPacket: func(r *types.ReceivePacket) {
			fmt.Printf("Received op %d, event %s, seq %d\n", r.Op, r.Event, r.Seq)
		},
		LogLevel: gateway.LogLevelDebug,
	})

	var err error
	c.Gateway, err = gateway.FetchGatewayBot(rest.NewClient(token, "9"))
	if err != nil {
		log.Panicf("failed to load gateway: %v", err)
	}

	ctx := context.Background()
	if err := c.Open(ctx); err != nil {
		log.Panicf("failed to open: %v", err)
	}

	select {}
}
