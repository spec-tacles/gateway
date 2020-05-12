# Spectacles Gateway

[![Docker pulls](https://img.shields.io/docker/pulls/spectacles/gateway)](https://hub.docker.com/r/spectacles/gateway)

The Spectacles gateway acts as a standalone process between your Discord bot application and the
Discord gateway, allowing your bot to focus entirely on application logic. This has numerous
benefits:

1. **Seamless upgrades.** If you configure the Spectacles Gateway to use one of the supported
message broker protocols, you can restart your bot and not lose any messages from the Discord
Gateway.
2. **Load scalability.** With the Spectacles Gateway responsible for all of the Discord logic,
you can scale your bot to handle high-load situations without worrying about restarting shards
and sessions.
3. **Feature scalability.** Since Discord messages get sent into a message broker, you can consume
them from more than just your bot application. Making a dashboard is trivial since you can run a
web application independently of your bot application and receive the exact same data.

## Getting Started

The recommended usage is through Docker, but pre-built binaries are also available in Github
Actions or you can compile it yourself using the latest Go compiler. Note that C build tools must
be available on your machine.

### Example

This example uses Docker to launch the most basic form of gateway with only the `MESSAGE_CREATE`
event being output to STDOUT.

```
$ docker run --rm -it
	-e DISCORD_TOKEN="your token"
	-e DISCORD_EVENTS=MESSAGE_CREATE
	spectacles/gateway
```

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
type = "amqp" # only supported type; any other value sends/receives from STDIN/STDOUT
group = "gateway"
message_timeout = "2m" # this is the default value: https://golang.org/pkg/time/#ParseDuration
url = "amqp://localhost"

[prometheus]
address = ":8080"
endpoint = "/metrics"

[shard_store]
type = "redis" # only supported type
prefix = "gateway" # string to prefix shard-store keys

[redis]
url = "localhost:6379"
pool_size = 5 # size of Redis connection pool
```

### Environment variables

Each of the below environment variables corresponds exactly to the config file above.

- `DISCORD_TOKEN`
- `DISCORD_EVENTS`: comma-separated list of gateway events

Optional:

- `DISCORD_INTENTS`: comma-separated list of gateway intents
- `DISCORD_RAW_INTENTS`: bitfield containing raw intent flags
- `DISCORD_SHARD_COUNT`
- `DISCORD_SHARD_IDS`: comma-separated list of shard IDs
- `BROKER_TYPE`
- `BROKER_GROUP`
- `BROKER_MESSAGE_TIMEOUT`
- `PROMETHEUS_ADDRESS`
- `PROMETHEUS_ENDPOINT`
- `SHARD_STORE_TYPE`
- `SHARD_STORE_PREFIX`

External connections:

- `AMQP_URL`
- `REDIS_URL`
- `REDIS_POOL_SIZE`

## How It Works

The Spectacles Gateway handles all of the Discord logic and simply forwards events to the specified
message broker. Your application is completely unaware of the existence of shards and just focuses
on handling incoming messages.

By default, the Spectacles Gateway sends and receives data through standard input and output. For
optimal use, you should use one of the available message broker protocols (currently only AMQP) to
send output to an external message broker (we recommend RabbitMQ). Your application can then
consume messages from the message broker.

Logs are output to STDERR and can be used to inspect the state of the gateway at any point. The
Spectacles Gateway also offers integration with Prometheus to enable detailed stats collection.

If you configure a shard storage solution (currently only Redis), shard information will be stored
there and used if/when the Spectacles Gateway restarts. If the Gateway restarts quickly enough, it
will be able to resume sessions without re-identifying to Discord.

## Goals

- [ ] Multiple output destinations
	- [x] STDIO
	- [x] AMQP
	- [ ] Redis
	- [ ] ???
- [x] Sharding
	- [x] Internal
	- [x] External
	- [ ] Auto (fully managed)
- [x] Distributable binary builds
	- [x] Linux
	- [x] Windows
- [x] Multithreading
- [x] Zero-alloc message handling
- [x] Discord compression (ZSTD)
- [x] Automatic restarting
- [ ] Failover
- [x] Session resuming
	- [x] Local
	- [x] Redis
