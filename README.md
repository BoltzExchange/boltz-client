# Boltzd

Boltz Client for CLN & LND

The daemon that professional routing node runners connect to their CLN & LND nodes for fully unattended channel rebalancing. The idea is to fully automate Liquid Swaps up to the point where node runners end up with mainchain bitcoin again. Beating competition with the significantly lower network fees on Liquid and privacy gain.

Liquid first, CLN first.

Feature List/Brain Dump:

- [ ] Daemon, CLI, gRPC
- [ ] Configurable via CLI or conf file
- [ ] Prio: first manual liquid swaps on CLN, then semi-automated liquid swaps based on channel balance threshold, focus on reverse swaps to obtain inbound
- [ ] Needs a liquid wallet, need to decide if we want a setup process (external mnemonic backup). Libraries/SDKs: [libwally-core](https://github.com/ElementsProject/libwally-core), [gdk](https://github.com/Blockstream/gdk)
- [ ] Depends on [chain-to-chain swaps](https://github.com/BoltzExchange/boltz-backend/issues/63) to be available to move from Liquid back to mainchain
- [ ] Lower Prio: Support manual mainchain swaps, all of [boltz-lnd](https://github.com/BoltzExchange/boltz-lnd)
- [ ] Consider building on and extending [boltz-lnd](https://github.com/BoltzExchange/boltz-lnd), we don't want to maintain two things. We will have to maintain boltz-lnd anyways since it runs on 1000+ umbrels
- [ ] Language: go is an obvious choice especially if [boltz-lnd](https://github.com/BoltzExchange/boltz-lnd) is base and also it compiles to binary, ts or python other options but always come with "setting up environment" trade-off

This repository contains a [Boltz](https://boltz.exchange) client for [LND](https://github.com/lightningnetwork/lnd). It supports Normal Submarine Swaps (from onchain to Lightning coins) and Reverse ones (from Lightning to onchain coins).

The full documentation can be found [here](https://lnd.docs.boltz.exchange/) or in the `docs` folder

## Building

To build Go version 1.14 or higher is required. `boltz-lnd` also has C dependencies which means a C compiler has to be installed to compile the daemon successfully.

The build process requires patches for dependencies, and some build flags, therefore the `Makefile` in the repository root should be used.

```
go mod vendor
make build
```
