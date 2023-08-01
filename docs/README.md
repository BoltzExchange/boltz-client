---
cover: .gitbook/assets/Screenshot from 2023-07-27 15-24-08.png
coverY: 0
---

# 👋 Introduction

`boltz-lnd` is a [Boltz](https://boltz.exchange) client for [LND](https://github.com/lightningnetwork/lnd). It supports Normal Submarine Swaps (mainchain Bitcoin -> Lightning) and Reverse Submarine Swaps (Lightning -> mainchain Bitcoin). Other layers like Liquid are currently _not_ supported. It consists of two main components:

## `boltzd`

`boltzd` is the daemon that runs alongside your LND node. It connects to your LND node and hooks it up with the [Boltz API](https://api.boltz.exchange/) to create and execute swaps.

## `boltzcli`

`boltzcli` is the CLI tool to interact with the gRPC interface that `boltzd` exposes.

## Setup

The LND node to which the daemon connects has to be version `v0.10.0-beta` or higher. Also, LND needs to be compiled with these build flags (official binaries from [Lightning Labs releases](https://github.com/lightningnetwork/lnd/releases) include them already):

* `invoicerpc` (hold invoices)
* `routerrpc` (multi path payments)
* `chainrpc` (block listener)
* `walletrpc` (fee estimations)

Binaries of `boltz-lnd` can be found [here](https://github.com/BoltzExchange/boltz-lnd/releases). If no binaries are available for your platform, you can build them yourself with the instructions provided in the repositories [README](https://github.com/BoltzExchange/boltz-lnd#building).

### Configuration

`boltzd` requires a connection to a LND node. In most cases the CLI flags `--lnd.certificate <path to the tls.cert of LND>` and `--lnd.macaroon <path to the admin.macaroon of LND>` should be enough. To view all CLI flags use `--help`.

`boltzd` can also be configured via a `TOML` file. The full documentation for the configuration file can be found in the [configuration](configuration.md) section.

### Macaroons

Macaroons for the gRPC server of `boltzd` can be found in the `macaroons` folder inside the data directory of the daemon. By default, that data directory is `~/.boltz-lnd` on Linux.
