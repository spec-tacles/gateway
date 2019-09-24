package cmd

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/spec-tacles/gateway/config"
	"github.com/spec-tacles/gateway/gateway"
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
	logger.Println("starting gateway")
	flag.Parse()

	conf, err := config.Read(*configLocation)
	if err != nil {
		logger.Fatalf("unable to load config: %s\n", err)
	}

	var (
		manager  *gateway.Manager
		b        broker.Broker
		logLevel = logLevels[*logLevel]
	)

	// TODO: support more broker types
	b = broker.NewAMQP(conf.Broker.Group, "", nil)
	open := make(chan struct{})
	go tryConnect(b, conf.Broker.URL, &open)

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
	manager.ConnectBroker(b, evts, &open)

	if err := manager.Start(); err != nil {
		logger.Fatalf("failed to connect to discord: %v", err)
	}
}

// tryConnect exponentially increases the retry interval, stopping at 80 seconds
func tryConnect(b broker.Broker, url string, open *chan struct{}) {
	closes := make(chan error)
	defer close(closes)

	for {
		if open == nil {
			*open = make(chan struct{})
		}

		retryInterval := time.Second * 5
		for err := b.Connect(url); err != nil; err = b.Connect(url) {
			logger.Printf("failed to connect to broker, retrying in %d seconds: %v\n", retryInterval/time.Second, err)
			time.Sleep(retryInterval)
			if retryInterval != 80 {
				retryInterval *= 2
			}
		}

		err := b.NotifyClose(closes)
		if err != nil {
			logger.Fatalf("failed to listen to closes: %s\n", err)
		}

		close(*open)
		*open = nil

		err = <-closes
		logger.Printf("connection to broker was closed (%s): reconnecting in 5s\n", err)
		time.Sleep(5 * time.Second)
	}
}
