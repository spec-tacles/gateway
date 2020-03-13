package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spec-tacles/go/types"
)

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

// Config represents configuration structure for the gateway
type Config struct {
	Token      string
	Events     []string
	Intents    []string
	RawIntents uint
	Shards     struct {
		Count int
		IDs   []int
	}
	Broker struct {
		Type           string
		URL            string
		Group          string
		MessageTimeout duration `toml:"message_timeout"`
	}
	Prometheus struct {
		Address  string
		Endpoint string
	}
}

// Read reads the config from file
func Read(file string) (conf *Config, err error) {
	conf = &Config{}
	toml.DecodeFile(file, conf)
	conf.LoadEnv()
	err = conf.Init()
	return
}

// Init initializes default config values
func (c *Config) Init() error {
	if c.Token == "" {
		return errors.New("missing Discord token")
	}

	if c.Broker.URL == "" {
		c.Broker.URL = "amqp://localhost"
	}

	if c.Broker.Group == "" {
		c.Broker.Group = "gateway"
	}

	if c.Broker.MessageTimeout.Duration == time.Duration(0) {
		c.Broker.MessageTimeout = duration{2 * time.Minute}
	}

	if c.RawIntents == 0 {
		for _, intent := range c.Intents {
			switch intent {
			case "GUILDS":
				c.RawIntents |= types.IntentGuilds
			case "GUILD_MEMBERS":
				c.RawIntents |= types.IntentGuildMembers
			case "GUILD_BANS":
				c.RawIntents |= types.IntentGuildBans
			case "GUILD_EMOJIS":
				c.RawIntents |= types.IntentGuildEmojis
			case "GUILD_INTEGRATIONS":
				c.RawIntents |= types.IntentGuildIntegrations
			case "GUILD_WEBHOOKS":
				c.RawIntents |= types.IntentGuildWebhooks
			case "GUILD_INVITES":
				c.RawIntents |= types.IntentGuildInvites
			case "GUILD_VOICE_STATES":
				c.RawIntents |= types.IntentGuildVoiceStates
			case "GUILD_PRESENCES":
				c.RawIntents |= types.IntentGuildPresences
			case "GUILD_MESSAGES":
				c.RawIntents |= types.IntentGuildMessages
			case "GUILD_MESSAGE_REACTIONS":
				c.RawIntents |= types.IntentGuildMessageReactions
			case "GUILD_MESSAGE_TYPING":
				c.RawIntents |= types.IntentGuildMessageTyping
			case "DIRECT_MESSAGES":
				c.RawIntents |= types.IntentDirectMessages
			case "DIRECT_MESSAGE_REACTIONS":
				c.RawIntents |= types.IntentDirectMessageReactions
			case "DIRECT_MESSAGE_TYPING":
				c.RawIntents |= types.IntentDirectMessageTyping
			}
		}
	}

	return nil
}

// LoadEnv loads environment variables into the config, overwriting any existing values
func (c *Config) LoadEnv() {
	var v string

	v = os.Getenv("DISCORD_TOKEN")
	if v != "" {
		c.Token = v
	}

	v = os.Getenv("DISCORD_EVENTS")
	if v != "" {
		c.Events = strings.Split(v, ",")
	}

	v = os.Getenv("DISCORD_INTENTS")
	if v != "" {
		c.Intents = strings.Split(v, ",")
	}

	v = os.Getenv("DISCORD_RAW_INTENTS")
	if v != "" {
		i, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			c.RawIntents = uint(i)
		}
	}

	v = os.Getenv("DISCORD_SHARD_COUNT")
	if v != "" {
		i, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			c.Shards.Count = int(i)
		}
	}

	v = os.Getenv("DISCORD_SHARD_IDS")
	if v != "" {
		ids := strings.Split(v, ",")
		c.Shards.IDs = make([]int, len(ids))
		for i, id := range ids {
			convID, err := strconv.Atoi(id)
			if err != nil {
				c.Shards.IDs[i] = convID
			}
		}
	}

	v = os.Getenv("BROKER_TYPE")
	if v != "" {
		c.Broker.Type = v
	}

	v = os.Getenv("BROKER_URL")
	if v != "" {
		c.Broker.URL = v
	}

	v = os.Getenv("BROKER_GROUP")
	if v != "" {
		c.Broker.Group = v
	}

	v = os.Getenv("BROKER_MESSAGE_TIMEOUT")
	if v != "" {
		timeout, err := time.ParseDuration(v)
		if err != nil {
			c.Broker.MessageTimeout = duration{timeout}
		}
	}

	v = os.Getenv("PROMETHEUS_ADDRESS")
	if v != "" {
		c.Prometheus.Address = v
	}

	v = os.Getenv("PROMETHEUS_ENDPOINT")
	if v != "" {
		c.Prometheus.Endpoint = v
	}
}

func (c *Config) String() string {
	strs := []string{
		fmt.Sprintf("Events:      %v", c.Events),
		fmt.Sprintf("Intents:     %v", c.Intents),
		fmt.Sprintf("Raw intents: %d", c.RawIntents),
		fmt.Sprintf("Shard count: %d", c.Shards.Count),
		fmt.Sprintf("Shard IDs:   %v", c.Shards.IDs),
		fmt.Sprintf("Broker:      %+v", c.Broker),
		fmt.Sprintf("Prometheus:  %+v", c.Prometheus),
	}

	return strings.Join(strs, "\n")
}
