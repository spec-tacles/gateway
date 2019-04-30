package cmd

import (
	"log"
	"os"
	"time"

	"github.com/spec-tacles/go/broker"
	"github.com/spec-tacles/go/gateway"
	"github.com/spec-tacles/go/rest"
	"github.com/spec-tacles/go/types"
	"github.com/spf13/cobra"
)

var amqpUrl, amqpGroup, token, logLevel string
var shardCount int

var logger = log.New(os.Stdout, "[CMD] ", log.Ldate|log.Ltime|log.Lshortfile)
var logLevels = map[string]int{
	"suppress": gateway.LogLevelSuppress,
	"info":     gateway.LogLevelInfo,
	"warn":     gateway.LogLevelWarn,
	"debug":    gateway.LogLevelDebug,
	"error":    gateway.LogLevelError,
}

var RootCmd = &cobra.Command{
	Use:   "spectacles",
	Short: "Connects to the Discord websocket API using spectacles.go",
	Run: func(cmd *cobra.Command, args []string) {
		amqp := broker.NewAMQP(amqpGroup, "", func(string, []byte) {})
		tryConnect(amqp)

		manager := gateway.NewManager(&gateway.ManagerOptions{
			ShardOptions: &gateway.ShardOptions{
				Identify: &types.Identify{
					Token: token,
				},
			},
			OnPacket: func(shard int, d *types.ReceivePacket) {
				amqp.Publish(string(d.Event), d.Data)
			},
			REST:     rest.NewClient(token),
			LogLevel: logLevels[logLevel],
		})

		if err := manager.Start(); err != nil {
			log.Fatalf("failed to connect to discord: %v", err)
		}
		select {}
	},
}

// tryConnect exponentially increases the retry interval, stopping at 80 seconds
func tryConnect(amqp *broker.AMQP) {
	retryInterval := time.Second * 5
	for err := amqp.Connect(amqpUrl); err != nil; {
		logger.Printf("failed to connect to amqp, retrying in %d seconds: %v\n", retryInterval, err)
		time.Sleep(retryInterval)
		if retryInterval != 80 {
			retryInterval *= 2
		}
	}
}

func init() {
	RootCmd.Flags().StringVarP(&amqpGroup, "group", "g", "", "The broker group to send Discord events to.")
	RootCmd.Flags().StringVarP(&amqpUrl, "url", "u", "", "The broker URL to connect to.")
	RootCmd.Flags().StringVarP(&token, "token", "t", "", "The Discord token used to connect to the gateway.")
	RootCmd.Flags().IntVarP(&shardCount, "shardcount", "c", 0, "The number of shards to spawn.")
	RootCmd.Flags().StringVarP(&logLevel, "loglevel", "l", "info", "The log level.")
}
