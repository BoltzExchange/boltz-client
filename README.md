# Boltz LND - Fix for rcp error

This repository contains a [Boltz](https://boltz.exchange) client for [LND](https://github.com/lightningnetwork/lnd). It supports Normal Submarine Swaps (from onchain to Lightning coins) and Reverse ones (from Lightning to onchain coins).

The full documentation can be found [here](https://lnd.docs.boltz.exchange/) or in the `docs` folder

## Building

To build Go version 1.14 or higher is required. `boltz-lnd` also has C dependencies which means a C compiler has to be installed to compile the daemon successfully.

The build process requires patches for dependencies, and some build flags, therefore the `Makefile` in the repository root should be used. 

```
go mod vendor
make build
```