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

The config file is required.

```toml
token = "" # Discord token
events = [] # array of event names to publish

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
