# Boltz Client

`boltz-client` connects to [CLN](https://github.com/ElementsProject/lightning/) or [LND](https://github.com/lightningnetwork/lnd/) nodes and allows for fully unattended channel rebalancing using [Boltz API](https://docs.boltz.exchange/v/api). It is composed of `boltzcli` and `boltzd`. The official documentation is available [here](https://docs.boltz.exchange/v/boltz-client/).

Design principles:
- CLI-first, fine-grained control and enhanced setup UX via `boltzcli`
- CLN-first: full support of all features
- [Liquid](https://liquid.net/)-first: optimized for Lightning -> Liquid -> mainchain swaps
- Create or import wallets, use node's wallet, swap to read-only wallets 
- Fully backwards compatible with last [boltz-lnd release](https://github.com/BoltzExchange/boltz-client/releases/tag/v1.2.7)


## Installation


### Binaries

`boltz-client` is only available for linux.
Download the latest binaries from the [releases](https://github.com/BoltzExchange/boltz-client/releases) page.

### Docker

`boltz-client` is also available as [docker image](https://hub.docker.com/r/boltz/boltz-client/tags). Assuming your lnd macaroon and certificate are placed in `~/.lnd`, run:
```
docker create -v ~/.boltz:/root/.boltz -v ~/.lnd:/root/.lnd --name boltz-client boltz/boltz-client:latest
docker start boltz-client
docker exec boltz-client boltzcli getinfo
```

### Building from source

To build, [Go](https://go.dev/) version `1.21` or higher is required. `boltz-client` also has C dependencies, which means a C compiler has to be installed to compile the daemon successfully.

`boltz-client` depends on [GDK](https://github.com/Blockstream/gdk) by blockstream, which can be either dynamically or statically linked.
The recommended way to build from source is linking dynamically as a static link requires compiling gdk aswell.

Run `make build` to build the daemon and cli. The binaries will be placed at the current path.

You can also run `make install` which will place the binaries into your `GOBIN` (`$HOME/go/bin` by default) directory.

