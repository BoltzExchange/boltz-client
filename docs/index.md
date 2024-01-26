# boltz-client v1.2.3 documentation

`boltz-client` is a [Boltz](https://boltz.exchange) client for [LND](https://github.com/lightningnetwork/lnd). It supports Normal Submarine Swaps (from onchain to Lightning coins) and Reverse Submarine Swaps (from Lightning to onchain coins).

## `boltzd`

`boltzd` is a daemon that should run alongside of your lightning node. It connects to your lightning node, and the Boltz API to create and execute Swaps.

## `boltzcli`

`boltzcli` is a CLI tool to interact with the gRPC interface `boltzd` exposes.

## Setup

Binaries for the latest release of `boltz-client` can be found on the [releases page](https://github.com/BoltzExchange/boltz-client/releases). If no binaries are available for your platform, you can build them yourself with the instructions provided in the [README](https://github.com/BoltzExchange/boltz-client#building).

### Configuration

`boltzd` requires a connection to a lightning node. It can connect to either CLN or LND

#### LND

The LND node to which the daemon connects has to be version `v0.10.0-beta` or higher. Also, LND needs to be compiled with these build flags (official binaries from Lightning Labs releases include them):

- `invoicerpc` (hold invoices)
- `routerrpc` (multi path payments)
- `chainrpc` (block listener)
- `walletrpc` (fee estimations)

In most cases the CLI flags `--lnd.certificate <path to the tls.cert of LND>` and `--lnd.macaroon <path to the admin.macaroon of LND>` should be enough.

#### CLN

The daemon connects to CLN through [gRPC](https://docs.corelightning.org/docs/grpc).
You need start CLN with the `--grpc-port` cli flag or set it in your config.
Then, set the following CLI flags or config values:

- `--cln.port` same port as used for `--grpc-port`
- `--cln.host` host of the machine CLN is running on
- `--cln.datadir` data directory of cln

To view all CLI flags use `--help`.

`boltzd` can also be configured via a TOML file. The full documentation for the configuration file can be found [here](configuration.md).

### Macaroons

The macaroons for the gRPC server of `boltzd` can be found in the `macaroons` folder inside the data directory of the daemon. By default, that data directory is `~/.boltz-client` on Linux.