package cmd

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/spec-tacles/gateway/gateway"
	"github.com/spec-tacles/go/broker"
	"github.com/spec-tacles/go/rest"
	"github.com/spec-tacles/go/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var logger = log.New(os.Stdout, "[CMD] ", log.Ldate|log.Ltime|log.Lshortfile)
var logLevels = map[string]int{
	"suppress": gateway.LogLevelSuppress,
	"info":     gateway.LogLevelInfo,
	"warn":     gateway.LogLevelWarn,
	"debug":    gateway.LogLevelDebug,
	"error":    gateway.LogLevelError,
}

var rootCmd = &cobra.Command{
	Use:   "gateway [output]",
	Short: "Connects to the Discord websocket API",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		var (
			onPacket func(shard int, d *types.ReceivePacket)
			output   io.ReadWriter
			manager  *gateway.Manager
			b        broker.Broker
			logLevel = logLevels[viper.GetString("loglevel")]
		)

		if len(args) > 0 {
			// TODO: support more broker types
			b = broker.NewAMQP(viper.GetString("group"), "", nil)
			tryConnect(b, args[0])
		}

		manager = gateway.NewManager(&gateway.ManagerOptions{
			ShardOptions: &gateway.ShardOptions{
				Identify: &types.Identify{
					Token: viper.GetString("token"),
				},
			},
			OnPacket:   onPacket,
			Output:     output,
			REST:       rest.NewClient(viper.GetString("token")),
			LogLevel:   logLevel,
			ShardCount: viper.GetInt("shards"),
		})
		manager.ConnectBroker(b)

		if err := manager.Start(); err != nil {
			logger.Fatalf("failed to connect to discord: %v", err)
		}
	},
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

func initConfig() {
	if viper.GetString("config") == "" {
		viper.AddConfigPath(".")
		viper.SetConfigName(".gateway")
	} else {
		viper.SetConfigFile(viper.GetString("config"))
	}

	viper.SetEnvPrefix("discord")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		logger.Println("Cannot read config:", err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	fg := rootCmd.PersistentFlags()
	fg.StringP("config", "c", "", "location of config file")
	fg.StringP("group", "g", "", "broker group to send Discord events to")
	fg.StringP("token", "t", "", "Discord token used to connect to gateway")
	fg.IntP("shards", "s", 0, "number of shards to spawn")
	fg.StringP("loglevel", "l", "", "log level")
	rootCmd.MarkFlagRequired("token")
	viper.BindPFlag("group", fg.Lookup("group"))
	viper.BindPFlag("token", fg.Lookup("token"))
	viper.BindPFlag("shards", fg.Lookup("shards"))
	viper.BindPFlag("loglevel", fg.Lookup("loglevel"))
}

// Execute the CLI application
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Fatalln(err)
	}
}
