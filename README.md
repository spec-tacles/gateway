# Spectacles Gateway

[![Docker pulls](https://img.shields.io/docker/pulls/spectacles/gateway)](https://hub.docker.com/r/spectacles/gateway)

A binary for ingesting data from the Discord gateway to a variety of sources.

## Goals

- [ ] Multiple output destinations
	- [x] AMQP
	- [ ] Redis
	- [ ] ???
- [x] Sharding
	- [x] Internal
	- [x] External
	- [ ] Auto (fully managed)
- [x] OS support
	- [x] Linux
	- [x] Windows
- [x] Distributable binary builds
- [x] Multithreading
- [ ] Zero-alloc message handling
- [x] Discord compression (ZSTD)
- [x] Automatic restarting
- [ ] Failover

## Usage

```
Usage of gateway:
  -config string
        location of the gateway config file (default "gateway.toml")
  -loglevel string
        log level for the client (default "info")
```

The gateway can be configured using either a config file or environment variables. Environment
variables take precedence over their corresponding entry in the config file.

### Config file

```toml
token = "" # Discord token
events = [] # array of gateway event names to publish

# everything below is optional

intents = [] # array of gateway intents to send when identifying
# https://gist.github.com/msciotti/223272a6f976ce4fda22d271c23d72d9

[shards]
count = 2
ids = [0, 1]

[broker]
type = "amqp" # if unspecified, uses STDIO for sending/receiving
url = "amqp://localhost"
group = "gateway"
message_timeout = "2m" # this is the default value

[prometheus]
address = ":8080"
endpoint = "/metrics"
```

### Environment variables

Each of the below environment variables corresponds exactly to the config file above.

- `DISCORD_TOKEN`
- `EVENTS`: comma-separated list of gateway events

Optional:

- `DISCORD_INTENTS`: comma-separated list of gateway intents
- `DISCORD_RAW_INTENTS`: bitfield containing raw intent flags
- `DISCORD_SHARD_COUNT`
- `DISCORD_SHARD_IDS`: comma-separated list of shard IDs
- `BROKER_TYPE`
- `BROKER_URL`
- `BROKER_GROUP`
- `BROKER_MESSAGE_TIMEOUT`: https://golang.org/pkg/time/#ParseDuration
- `PROMETHEUS_ADDRESS`
- `PROMETHEUS_ENDPOINT`
