---
cover: .gitbook/assets/boltz-client (1).png
coverY: 0
---

# 👋 Introduction

Boltz Client connects to [CLN](https://github.com/ElementsProject/lightning/) or [LND](https://github.com/lightningnetwork/lnd/) nodes and allows for fully unattended channel rebalancing using [Boltz API](https://docs.boltz.exchange/v/api). It is composed of `boltzcli` and `boltzd`.

Design principles:

* CLI-first, fine-grained control and enhanced setup UX via `boltzcli`
* CLN-first: full support of all features
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

Boltz Client is only available for linux. Download the latest binaries from the [release](https://github.com/BoltzExchange/boltz-client/releases) page.

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

`boltzd` requires a connection to a lightning node. It can connect to either CLN or LND.

#### LND

The LND node to which the daemon connects has to be version `v0.10.0-beta` or higher. Also, LND needs to be compiled with these build flags (official binaries from Lightning Labs releases include them):

* `invoicerpc` (hold invoices)
* `routerrpc` (multi path payments)
* `chainrpc` (block listener)
* `walletrpc` (fee estimations)

In most cases, the CLI flags `--lnd.certificate <path to the tls.cert of LND>` and `--lnd.macaroon <path to the admin.macaroon of LND>` should be enough.

#### CLN

The daemon connects to CLN through [gRPC](https://docs.corelightning.org/docs/grpc). You need start CLN with the `--grpc-port` CLI flag, or set it in your config:

* `--cln.port` same port as used for `--grpc-port`
* `--cln.host` host of the machine CLN is running on
* `--cln.datadir` data directory of cln

To view all CLI flags use `--help`.

`boltzd` can also be configured via a TOML file. The documentation for the configuration file can be found [here](configuration.md).

### Macaroons

The macaroons for the gRPC server of `boltzd` can be found in the `macaroons` folder inside the data directory of the daemon. By default, that data directory is `~/.boltz-client` on Linux.
