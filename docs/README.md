---
cover: .gitbook/assets/boltz-client (1).png
coverY: 0
---

# 👋 Introduction

Boltz Client connects to [CLN](https://github.com/ElementsProject/lightning/) or [LND](https://github.com/lightningnetwork/lnd/) nodes and allows for fully unattended channel rebalancing using [Boltz API](https://docs.boltz.exchange/v/api). It is composed of `boltzcli` and `boltzd`.

Design principles:

* CLI-first, fine-grained control and enhanced setup UX via `boltzcli`
* CLN-first: first-class citizen support
* [Liquid](https://liquid.net/)-first: optimized for Lightning -> Liquid -> mainchain swaps
* Create or import Liquid/mainchain wallets, swap to read-only wallets
* Fully backwards compatible with last [boltz-lnd release](https://github.com/BoltzExchange/boltz-client/releases/tag/v1.2.7)

It consists of two main components:

## `boltzd`

`boltzd` is a daemon that should run alongside of your lightning node. It connects to your lightning node and the Boltz API to create and execute Swaps.

## `boltzcli`

`boltzcli` is the CLI tool to interact with the gRPC interface that `boltzd` exposes.

## Installation

### Binaries

Boltz Client is available for linux on `amd64` and `arm64`. Download the latest binaries from the [release](https://github.com/BoltzExchange/boltz-client/releases) page. If you are on another platform, use the docker image below.

### Docker

Boltz Client is also available as [docker image](https://hub.docker.com/r/boltz/boltz-client/tags). Assuming your lnd macaroon and certificate are placed in `~/.lnd`, run:

```
docker create -v ~/.boltz:/root/.boltz -v ~/.lnd:/root/.lnd --name boltz-client boltz/boltz-client:latest
docker start boltz-client
docker exec boltz-client boltzcli getinfo
```

### Building from source

To build, [Go](https://go.dev/) version `1.21` or higher is required. Boltz Client also has C dependencies, which means a C compiler has to be installed to compile the daemon successfully.

Boltz Client depends on [GDK](https://github.com/Blockstream/gdk) by blockstream, which can be either dynamically or statically linked. The recommended way to build from source is linking dynamically as a static link requires compiling gdk aswell.

Run `make build` to build the daemon and CLI. The binaries will be placed at the current path.

You can also run `make install` which will place the binaries into your `GOBIN` (`$HOME/go/bin` by default) directory.

## Setup

Binaries for the latest release of Boltz Client can be found on the [release page](https://github.com/BoltzExchange/boltz-client/releases). If no binaries are available for your platform, you can build them yourself with the instructions above.

### Configuration

Configuration can be done via cli params or a TOML configuration file (default `~/.boltz/boltz.toml`). An example configuration file can be found [here](configuration.md). We suggest you start off with it. We suggest you start off with it.

`boltzd` requires a connection to a lightning node, which ban be CLN or LND. If you set configuration values for both, you can specify which to use with the `node` param.

To view all CLI flags use `--help`.

#### LND

The LND node to which the daemon connects has to be version `v0.10.0-beta` or higher. Also, LND needs to be compiled with these build flags (official binaries from Lightning Labs releases include them):

* `invoicerpc` (hold invoices)
* `routerrpc` (multi path payments)
* `chainrpc` (block listener)
* `walletrpc` (fee estimations)

By default, boltzd will attempt to connect to lnd running at `localhost:10009` (`lnd.host` and `lnd.port`) and looking for certificate and macaroon inside the data directory `~/.lnd` (`lnd.datadir`).

You can manually set the location of the tls certificate (`lnd.certificate`) and admin macaroon (`lnd.macaroon`) instead of speciyfing the data directory aswell.

#### CLN

The daemon connects to CLN through [gRPC](https://docs.corelightning.org/docs/grpc). You need start CLN with the `--grpc-port` CLI flag, or set it in your config:

* `--cln.port` same port as used for `--grpc-port`
* `--cln.host` host of the machine CLN is running on
* `--cln.datadir` data directory of cln (`~/.lightning` by default)

You can manually set the paths of `cln.rootcert`, `cln.privatekey` and `cln.certchain` instead of speciyfing the data directory aswell.

### Macaroons

The macaroons for the gRPC server of `boltzd` can be found in the `macaroons` folder inside the data directory of the daemon. By default, that data directory is `~/.boltz` on Linux.
