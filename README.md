# Spectacles Gateway

[![Docker build status](https://img.shields.io/docker/cloud/automated/spectacles/gateway.svg)](https://hub.docker.com/r/spectacles/gateway)

A binary for ingesting data from the Discord gateway to a variety of sources.

## Goals

- [ ] Multiple output destinations
	- [x] STDOUT
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
Usage of spectacles:
  -config string
        location of the Spectacles config file (default "spectacles.toml")
  -loglevel string
        log level for the client (default "info")
```

The config file is required and must conform to the [Spectacles standard config file](https://github.com/spec-tacles/spec/blob/276b7e4737658471b3a74cc06df4795c8901ca8e/config.md).
