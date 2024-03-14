
<a name="v2.0.0"></a>
## [v2.0.0] - 2024-03-14
### Feat
- release workflow
- return_immediately parameter for reverse swap creation ([#94](https://github.com/BoltzExchange/boltz-client/issues/94))
- refund rpc ([#89](https://github.com/BoltzExchange/boltz-client/issues/89))
- allow custom swap invoice ([#91](https://github.com/BoltzExchange/boltz-client/issues/91))
- use proper currency type in wallets grpc ([#90](https://github.com/BoltzExchange/boltz-client/issues/90))
- compat dockerfile
- getpairs rpc ([#88](https://github.com/BoltzExchange/boltz-client/issues/88))
- replace sse with ws ([#85](https://github.com/BoltzExchange/boltz-client/issues/85))
- cooperative submarine claims ([#84](https://github.com/BoltzExchange/boltz-client/issues/84))
- v2 api ([#83](https://github.com/BoltzExchange/boltz-client/issues/83))

### Fix
- make sure fresh wallet addresses are used in autoswap
- cli typo
- autoswap setup swaptype ([#115](https://github.com/BoltzExchange/boltz-client/issues/115))
- makefile command binaries
- db migrations ([#100](https://github.com/BoltzExchange/boltz-client/issues/100))
- dont require currency when setting subaccount in cli
- proper semver version check ([#92](https://github.com/BoltzExchange/boltz-client/issues/92))
- dont specify unnecessary expiry in test invoice

### Refactor
- make `refund_address` optional in rpc
- rename autoSend to sendFromInternal ([#116](https://github.com/BoltzExchange/boltz-client/issues/116))
- use concrete type for block_heights in grpc
- cleanup autoswap config ([#96](https://github.com/BoltzExchange/boltz-client/issues/96))
- use proper message type for autoswap config instead of json ([#93](https://github.com/BoltzExchange/boltz-client/issues/93))
- improve boltz package ([#86](https://github.com/BoltzExchange/boltz-client/issues/86))
- parse null timestamp from db as 0


<a name="v2.0.0-beta"></a>
## [v2.0.0-beta] - 2024-02-05
### Feat
- improve errors if wallet subaccount doesnt exist ([#82](https://github.com/BoltzExchange/boltz-client/issues/82))
- allow settings cli flags through env vars
- allow node configuration through datadir + network ([#77](https://github.com/BoltzExchange/boltz-client/issues/77))
- add referralId

### Fix
- exclude electrum tests from unit tests
- install ca-certificates in docker image to avoid ssl errors
- correct request to lnd `ConnectPeer`
- only try to decode config if file exists
- specify entrypoint isntead of cmd in docker image
- gitbook back to home link
- add missing `transaction.lockupFailed` event
- dont close event stream when reconnecting
- update version table after migrating db

### Refactor
- print data folder warning after config
- change wallet data directory
- rename autobudget config values ([#79](https://github.com/BoltzExchange/boltz-client/issues/79))
- allow specifying multiple channel ids instead of one ([#73](https://github.com/BoltzExchange/boltz-client/issues/73))
- move docs to gitbook


<a name="v1.3.0-rc1"></a>
## [v1.3.0-rc1] - 2023-11-28
### Feat
- listswaps filter ([#78](https://github.com/BoltzExchange/boltz-client/issues/78))
- subaccount improvements ([#79](https://github.com/BoltzExchange/boltz-client/issues/79))
- initial lightning implementation
- use in-memory connection for tests
- split `Start` into `Start` and `Init`, NoTls
- integration tests

### Fix
- use right logger
- .gitignore


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
- add Dockerfile ([#47](https://github.com/BoltzExchange/boltz-client/issues/47))

### Fix
- set fee floor to 2 sat/vByte
- claimTransactionId typo ([#45](https://github.com/BoltzExchange/boltz-client/issues/45))


<a name="v1.2.5"></a>
## [v1.2.5] - 2021-03-10
### Fix
- replace int data type ([#41](https://github.com/BoltzExchange/boltz-client/issues/41))


<a name="v1.2.4"></a>
## [v1.2.4] - 2021-03-01
### Fix
- refund expired deposit swaps ([#38](https://github.com/BoltzExchange/boltz-client/issues/38))


<a name="v1.2.3"></a>
## [v1.2.3] - 2021-02-20
### Feat
- save errors to database ([#36](https://github.com/BoltzExchange/boltz-client/issues/36))


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
- print parsed config on startup ([#29](https://github.com/BoltzExchange/boltz-client/issues/29))
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
- nil pointer dereference when LND is killed ([#18](https://github.com/BoltzExchange/boltz-client/issues/18))


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
- add ListSwaps command ([#15](https://github.com/BoltzExchange/boltz-client/issues/15))
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
- add Reverse Swaps ([#3](https://github.com/BoltzExchange/boltz-client/issues/3))
- fee of refund transactions
- batch refund transactions
- add refund logic
- add Submarine Swaps

### Fix
- show lockup address for Reverse Swaps
- limits of deposit call
- cleanup streams ([#14](https://github.com/BoltzExchange/boltz-client/issues/14))
- invoice expiry timeout
- parsing of percentage fee
- bech32 litecoin addresses
- crash in deposit flow
- crash when Swap could not be found
- add Boltz version check
- CLTV encoding

### Refactor
- improve Channel Creation enforcement


[v2.0.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.0.0-beta...v2.0.0
[v2.0.0-beta]: https://github.com/BoltzExchange/boltz-client/compare/v1.3.0-rc1...v2.0.0-beta
[v1.3.0-rc1]: https://github.com/BoltzExchange/boltz-client/compare/v1.2.7...v1.3.0-rc1
[v1.2.7]: https://github.com/BoltzExchange/boltz-client/compare/v1.2.6...v1.2.7
[v1.2.6]: https://github.com/BoltzExchange/boltz-client/compare/v1.2.5...v1.2.6
[v1.2.5]: https://github.com/BoltzExchange/boltz-client/compare/v1.2.4...v1.2.5
[v1.2.4]: https://github.com/BoltzExchange/boltz-client/compare/v1.2.3...v1.2.4
[v1.2.3]: https://github.com/BoltzExchange/boltz-client/compare/v1.2.2...v1.2.3
[v1.2.2]: https://github.com/BoltzExchange/boltz-client/compare/v1.2.1...v1.2.2
[v1.2.1]: https://github.com/BoltzExchange/boltz-client/compare/v1.2.0...v1.2.1
[v1.2.0]: https://github.com/BoltzExchange/boltz-client/compare/v1.1.2...v1.2.0
[v1.1.2]: https://github.com/BoltzExchange/boltz-client/compare/v1.1.1...v1.1.2
[v1.1.1]: https://github.com/BoltzExchange/boltz-client/compare/v1.1.0...v1.1.1
[v1.1.0]: https://github.com/BoltzExchange/boltz-client/compare/v1.0.0...v1.1.0
