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
gateway [output] [flags]

Flags:
	-c, --config string     location of config file
	-g, --group string      broker group to send Discord events to
	-h, --help              help for spectacles
	-l, --loglevel string   log level (default "info")
	-s, --shards int        number of shards to spawn
	-t, --token string      Discord token used to connect to gateway
```

`output` is a URL to an output destination (including protocol). If output is `-`, output is set to STDOUT: logging is automatically disabled. All destinations (except STDOUT) must adhere to the [Spectacles specification](https://github.com/spec-tacles/spec).

Applications reading from STDOUT should expect uncompressed JSON packets as received from the Discord gateway. No additional information is provided.

### Configuration Options

Config can be specified in JSON, YAML, TOML, HCL, or Java properties files or in environment variables prefixed with `DISCORD_`. We highly recommend providing sensitive information (such as your token) in either file or environment variable form to avoid disclosing it when viewing processes in `htop` or other monitoring tools.
