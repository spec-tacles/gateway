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

[shards] # optional
count = 2
ids = [0, 1]

[broker] # optional
type = "amqp"
url = "amqp://localhost"
group = "gateway"
```
