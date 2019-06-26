package cmd

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spec-tacles/gateway/gateway"
	"github.com/spec-tacles/gateway/config"
	"github.com/spec-tacles/go/broker"
	"github.com/spec-tacles/go/rest"
	"github.com/spec-tacles/go/types"
)

var (
	logger    = log.New(os.Stdout, "[CMD] ", log.Ldate|log.Ltime|log.Lshortfile)
	logLevels = map[string]int{
		"suppress": gateway.LogLevelSuppress,
		"info":     gateway.LogLevelInfo,
		"warn":     gateway.LogLevelWarn,
		"debug":    gateway.LogLevelDebug,
		"error":    gateway.LogLevelError,
	}
	logLevel       = flag.String("loglevel", "info", "log level for the client")
	configLocation = flag.String("config", "gateway.toml", "location of the gateway config file")
)

// Run runs the CLI app
func Run() {
	flag.Parse()

	conf := &config.Config{}
	_, err := toml.DecodeFile(*configLocation, conf)
	if err != nil {
		logger.Fatalf("unable to load config: %s\n", err)
	}
	conf.Init()

	var (
		manager  *gateway.Manager
		b        broker.Broker
		logLevel = logLevels[*logLevel]
	)

	// TODO: support more broker types
	b = broker.NewAMQP(conf.Broker.Group, "", nil)
	tryConnect(b, conf.Broker.URL)

	manager = gateway.NewManager(&gateway.ManagerOptions{
		ShardOptions: &gateway.ShardOptions{
			Identify: &types.Identify{
				Token: conf.Token,
			},
		},
		REST:       rest.NewClient(conf.Token),
		LogLevel:   logLevel,
		ShardCount: conf.Shards.Count,
	})

	evts := make(map[string]struct{})
	for _, e := range conf.Events {
		evts[e] = struct{}{}
	}
	manager.ConnectBroker(b, evts)

	if err := manager.Start(); err != nil {
		logger.Fatalf("failed to connect to discord: %v", err)
	}
}

// tryConnect exponentially increases the retry interval, stopping at 80 seconds
func tryConnect(b broker.Broker, url string) {
	retryInterval := time.Second * 5
	for err := b.Connect(url); err != nil; err = b.Connect(url) {
		logger.Printf("failed to connect to broker, retrying in %d seconds: %v\n", retryInterval/time.Second, err)
		time.Sleep(retryInterval)
		if retryInterval != 80 {
			retryInterval *= 2
		}
	}
}
