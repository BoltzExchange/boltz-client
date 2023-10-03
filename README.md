# Boltz LND

This repository contains `boltz-lnd`, a [Boltz](https://boltz.exchange) client for [LND](https://github.com/lightningnetwork/lnd). It supports Normal Submarine Swaps (Bitcoin -> Lightning) and Reverse Submarine Swaps (Lightning -> Bitcoin).

The full documentation can be found [here](https://docs.boltz.exchange/boltz-lnd) or in the [`docs`](./docs) folder of this repo.

## Building

To build `boltz-lnd`, [go](https://github.com/golang/go) version 1.14 or higher is required. `boltz-lnd` also has C dependencies which means a C compiler has to be installed to compile the daemon successfully.

The build process requires patches for dependencies, and some build flags, therefore the provided [`Makefile`](Makefile) should be used. 

To start the build process, run:
```
go mod vendor
make build
```
