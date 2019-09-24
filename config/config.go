package config

import (
	"github.com/BurntSushi/toml"
)

// Config represents configuration structure for the gateway
type Config struct {
	Token  string
	Events []string
	Shards struct {
		Count int
		IDs   []int
	}
	Broker struct {
		URL   string
		Group string
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
}
