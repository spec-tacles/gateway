package cmd

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	if conf.Prometheus.Address != "" {
		var mainHandler http.Handler
		if conf.Prometheus.Endpoint == "" {
			mainHandler = promhttp.Handler()
		} else {
			http.Handle(conf.Prometheus.Endpoint, promhttp.Handler())
		}

		logger.Printf("exposing Prometheus stats at %v%v", conf.Prometheus.Address, conf.Prometheus.Endpoint)
		go func() {
			logger.Fatal(http.ListenAndServe(conf.Prometheus.Address, mainHandler))
		}()
	}

	var (
		manager  *gateway.Manager
		logLevel = logLevels[*logLevel]
	)

	// TODO: support more broker types
	bm := gateway.NewBrokerManager(broker.NewAMQP(conf.Broker.Group, "", nil), logger)
	go bm.Connect(conf.Broker.URL)

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
	manager.ConnectBroker(bm, evts)

	if err := manager.Start(); err != nil {
		logger.Fatalf("failed to connect to discord: %v", err)
	}
}
