package config

import (
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

func Read(file string) (conf *Config, err error) {
	conf = &Config{}
	_, err = toml.DecodeFile(file, conf)
	if err != nil {
		return
	}
	conf.Init()
	return
}

// Init initializes default config values
func (c *Config) Init() {
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
}
