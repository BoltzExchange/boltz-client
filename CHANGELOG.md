
<a name="v1.2.7"></a>
## [v1.2.7] - 2023-04-03
### Feat
- allow disabling mempool.space integration
- use mempool.space API for fee estimations

### Fix
- LND fee estimation
- linting errors
- Docker build on armv7


<a name="v1.2.6"></a>
## [v1.2.6] - 2021-07-22
### Feat
- add Dockerfile ([#47](https://github.com/BoltzExchange/boltz-lnd/issues/47))

### Fix
- set fee floor to 2 sat/vByte
- claimTransactionId typo ([#45](https://github.com/BoltzExchange/boltz-lnd/issues/45))


<a name="v1.2.5"></a>
## [v1.2.5] - 2021-03-10
### Fix
- replace int data type ([#41](https://github.com/BoltzExchange/boltz-lnd/issues/41))


<a name="v1.2.4"></a>
## [v1.2.4] - 2021-03-01
### Fix
- refund expired deposit swaps ([#38](https://github.com/BoltzExchange/boltz-lnd/issues/38))


<a name="v1.2.3"></a>
## [v1.2.3] - 2021-02-20
### Feat
- save errors to database ([#36](https://github.com/BoltzExchange/boltz-lnd/issues/36))


<a name="v1.2.2"></a>
## [v1.2.2] - 2020-12-24
### Fix
- macaroon directory path


<a name="v1.2.1"></a>
## [v1.2.1] - 2020-12-24
### Feat
- add datadir argument
- add Darwin artifacts
- add command to print macaroon in hex
- add readonly macaroon

### Fix
- broken link in gRPC documentation
- set payment fee limit
- use litoshi as denomination for LTC


<a name="v1.2.0"></a>
## [v1.2.0] - 2020-12-14
### Feat
- print parsed config on startup ([#29](https://github.com/BoltzExchange/boltz-lnd/issues/29))
- add network to GetInfo response
- custom paths for REST proxy
- add REST proxy for gRPC server
- add macaroon authentication for gRPC server
- add TLS support for gRPC server

### Fix
- resolve TLS issues of REST proxy


<a name="v1.1.2"></a>
## [v1.1.2] - 2020-12-01
### Fix
- add Swap ID to DepositResponse


<a name="v1.1.1"></a>
## [v1.1.1] - 2020-09-09
### Fix
- nil pointer dereference when LND is killed ([#18](https://github.com/BoltzExchange/boltz-lnd/issues/18))


<a name="v1.1.0"></a>
## [v1.1.0] - 2020-07-29
### Fix
- reconnect to SSE stream
- no LND P2P connection


<a name="v1.0.0"></a>
## v1.0.0 - 2020-07-03
### Feat
- store transaction ids of Reverse Swaps
- store transaction ids of Swaps
- add ListSwaps command ([#15](https://github.com/BoltzExchange/boltz-lnd/issues/15))
- add Channel Creations to GetSwapInfo
- add database schema version
- inbound parameter for deposit call
- prefix for logger
- add withdraw command
- add deposit command
- improve CLI argument parsing
- add lockup address to Reverse Swap response
- add Litecoin support
- handle locked LNDs
- wait for LND to be synced
- add startup sanity checks
- set default Boltz endpoint based on chain
- add Channel Creations
- add Reverse Swaps ([#3](https://github.com/BoltzExchange/boltz-lnd/issues/3))
- fee of refund transactions
- batch refund transactions
- add refund logic
- add Submarine Swaps

### Fix
- show lockup address for Reverse Swaps
- limits of deposit call
- cleanup streams ([#14](https://github.com/BoltzExchange/boltz-lnd/issues/14))
- invoice expiry timeout
- parsing of percentage fee
- bech32 litecoin addresses
- crash in deposit flow
- crash when Swap could not be found
- add Boltz version check
- CLTV encoding

### Refactor
- improve Channel Creation enforcement


[v1.2.7]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.2.6...v1.2.7
[v1.2.6]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.2.5...v1.2.6
[v1.2.5]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.2.4...v1.2.5
[v1.2.4]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.2.3...v1.2.4
[v1.2.3]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.2.2...v1.2.3
[v1.2.2]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.2.1...v1.2.2
[v1.2.1]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.2.0...v1.2.1
[v1.2.0]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.1.2...v1.2.0
[v1.1.2]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.1.1...v1.1.2
[v1.1.1]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.1.0...v1.1.1
[v1.1.0]: https://github.com/BoltzExchange/boltz-lnd/compare/v1.0.0...v1.1.0
