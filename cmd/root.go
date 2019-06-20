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
		var dest string
		if len(args) < 1 {
			dest = "-"
		} else {
			dest = args[0]
		}

		var (
			onPacket func(shard int, d *types.ReceivePacket)
			output   io.ReadWriter
			logLevel = logLevels[viper.GetString("loglevel")]
		)
		if dest == "-" {
			output = os.Stdout
			logLevel = gateway.LogLevelSuppress
		} else {
			amqp := broker.NewAMQP(viper.GetString("group"), "", nil)
			tryConnect(amqp, dest)

			onPacket = func(shard int, d *types.ReceivePacket) {
				amqp.Publish(string(d.Event), d.Data)
			}
		}

		manager := gateway.NewManager(&gateway.ManagerOptions{
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

		if err := manager.Start(); err != nil {
			logger.Fatalf("failed to connect to discord: %v", err)
		}
		select {}
	},
}

// tryConnect exponentially increases the retry interval, stopping at 80 seconds
func tryConnect(amqp *broker.AMQP, url string) {
	retryInterval := time.Second * 5
	for err := amqp.Connect(url); err != nil; err = amqp.Connect(url) {
		logger.Printf("failed to connect to amqp, retrying in %d seconds: %v\n", retryInterval/time.Second, err)
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
