package cmd

import (
	"context"
	"flag"
	"net/http"
	"os"

	"github.com/mediocregopher/radix/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rabbitmq/amqp091-go"
	"github.com/spec-tacles/gateway/config"
	"github.com/spec-tacles/gateway/gateway"
	"github.com/spec-tacles/go/broker"
	"github.com/spec-tacles/go/broker/amqp"
	"github.com/spec-tacles/go/broker/redis"
	"github.com/spec-tacles/go/rest"
	"github.com/spec-tacles/go/types"
)

var (
	logger    = gateway.ChildLogger(gateway.DefaultLogger, "[CMD]")
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

var redisClient *radix.Client

func getRedis(ctx context.Context, conf *config.Config) radix.Client {
	if redisClient != nil {
		return *redisClient
	}

	newClient, err := radix.PoolConfig{
		Size: conf.Redis.PoolSize,
	}.New(ctx, "tcp", conf.Redis.URL)

	if err != nil {
		logger.Fatalf("Unable to connect to redis: %s", err)
	}

	redisClient = &newClient
	return newClient
}

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
		manager    *gateway.Manager
		b          broker.Broker
		shardStore gateway.ShardStore
		logLevel   = logLevels[*logLevel]
		ctx        = context.Background()
	)

	switch conf.Broker.Type {
	case "amqp":
		conn, err := amqp091.Dial(conf.AMQP.URL)
		if err != nil {
			logger.Fatalf("error connecting to AMQP: %s", err)
		}

		amqp := &amqp.AMQP{
			Group:   conf.Broker.Group,
			Timeout: conf.Broker.MessageTimeout.Duration,
		}
		amqp.Init(conn)
		b = amqp
	case "redis":
		client := getRedis(ctx, conf)
		r := redis.NewRedis(client, conf.Broker.Group)
		r.UnackTimeout = conf.Broker.MessageTimeout.Duration

		b = r
	default:
		b = &broker.RWBroker{R: os.Stdin, W: os.Stdout}
	}

	switch conf.ShardStore.Type {
	case "redis":
		redis := getRedis(ctx, conf)
		shardStore = &gateway.RedisShardStore{
			Redis:  redis,
			Prefix: conf.ShardStore.Prefix,
		}
	}

	manager = gateway.NewManager(&gateway.ManagerOptions{
		ShardOptions: &gateway.ShardOptions{
			Store: shardStore,
			Identify: &types.Identify{
				Token:    conf.Token,
				Intents:  int(conf.RawIntents),
				Presence: &conf.Presence,
			},
			Version: conf.GatewayVersion,
		},
		REST:       rest.NewClient(conf.Token, "9"),
		LogLevel:   logLevel,
		ShardCount: conf.Shards.Count,
	})

	evts := make(map[string]struct{})
	for _, e := range conf.Events {
		evts[e] = struct{}{}
	}
	manager.ConnectBroker(ctx, b, evts)

	logger.Printf("using config:\n%+v\n", conf)
	if err := manager.Start(ctx); err != nil {
		logger.Fatalf("failed to connect to discord: %v", err)
	}
}
