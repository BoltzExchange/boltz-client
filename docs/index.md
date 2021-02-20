# boltz-lnd v1.2.3 documentation

`boltz-lnd` is a [Boltz](https://boltz.exchange) client for [LND](https://github.com/lightningnetwork/lnd). It supports Normal Submarine Swaps (from onchain to Lightning coins) and Reverse Submarine Swaps (from Lightning to onchain coins).

## `boltzd`

`boltzd` is a daemon that should run alongside of your LND node. It connects to your LND node, and the Boltz API to create and execute Swaps.

## `boltzcli`

`boltzcli` is a CLI tool to interact with the gRPC interface `boltzd` exposes.

## Setup

The LND node to which the daemon connects has to be version `v0.10.0-beta` or higher. Also, LND needs to be compiled with these build flags (official binaries from Lightning Labs releases include them):

- `invoicerpc` (hold invoices)
- `routerrpc` (multi path payments)
- `chainrpc` (block listener)
- `walletrpc` (fee estimations)

Binaries for the latest release of `boltz-lnd` can be found on the [releases page](https://github.com/BoltzExchange/boltz-lnd/releases). If no binaries are available for your platform, you can build them yourself with the instructions provided in the [README](https://github.com/BoltzExchange/boltz-lnd#building).

### Configuration

`boltzd` requires a connection to a LND node. In most cases the CLI flags `--lnd.certificate <path to the tls.cert of LND>` and `--lnd.macaroon <path to the admin.macaroon of LND>` should be enough. To view all CLI flags use `--help`.

`boltzd` can also be configured via a TOML file. The full documentation for the configuration file can be found [here](configuration.md).

### Macaroons

The macaroons for the gRPC server of `boltzd` can be found in the `macaroons` folder inside the data directory of the daemon. By default, that data directory is `~/.boltz-lnd` on Linux.
