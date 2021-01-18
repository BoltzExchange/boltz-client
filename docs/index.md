# boltz-lnd v1.2.2 documentation

`boltz-lnd` is a [Boltz](https://boltz.exchange) client for [LND](https://github.com/lightningnetwork/lnd). It supports Normal Submarine Swaps (from onchain to Lightning coins) and Reverse Submarine Swaps (from Lightning to onchain coins).

## `boltzd`

`boltzd` is a daemon that should run alongside of your LND node. It connects to your LND node, and the Boltz API to create to execute Swaps.

The LND node to which the daemon connects to, has to be version `v0.10.0-beta` or higher. Also, LND needs to be compiled with these build flags (binaries from the official Lightning Labs releases include them):

- `invoicerpc` (hold invoices)
- `routerrpc` (multi path payments)
- `chainrpc` (block listener)
- `walletrpc` (fee estimations)

Documentation for the configuration of the daemon can be found [here](configuration.md)

## `boltzcli`

`boltzcli` is a CLI tool to interact with the gRPC interface `boltzd` exposes.
