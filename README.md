# Boltz LND

This repository contains a [Boltz](https://boltz.exchange) client for [LND](https://github.com/lightningnetwork/lnd). It supports Normal Submarine Swaps (from onchain to Lightning coins) and Reverse ones (from Lightning to onchain coins).

**TODO: channel creations**

## `boltzd`

`boltzd` is a daemon that should run alongside of your LND node. It connects to your LND node, and the Boltz API to create and exeucte Swaps. 

The LND node to which the daemon connects to, has to be version `v0.10.0-beta` or higher. Also, LND needs to be compiled with these build flags (binaries from the official Lightning Labs releases include them):

- `invoicerpc` (hold invoices)
- `routerrpc` (multi path payments)
- `chainrpc` (block listener)
- `walletrpc` (fee estimations)

The daemon can be configured with CLI parameters, or a config file. A sample config file can be found below.

```toml
[Boltz]
url = "<URL to the Boltz API>"

[LND]
# Host of the LND gRPC interface
host = "127.0.0.1"
# Port of the LND gRPC interface
port = 10009
# Path to the admin macaroon of LND
macaroon = ""
# Path to the gRPC certificate of LND
certificate = ""
```

## `boltz-cli`

`boltz-cli` is a cli tool to interact with the gRPC interface `boltzd` exposes. 