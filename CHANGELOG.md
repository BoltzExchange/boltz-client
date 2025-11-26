
<a name="v2.10.0"></a>
## [v2.10.0] - 2025-11-26
### Feat
- add command to remove tenant ([#597](https://github.com/BoltzExchange/boltz-client/issues/597))
- bdk rust bindings ([#566](https://github.com/BoltzExchange/boltz-client/issues/566))

### Fix
- do not to refund swaps without address or wallet ([#606](https://github.com/BoltzExchange/boltz-client/issues/606))
- go electrum connection timeout context ([#607](https://github.com/BoltzExchange/boltz-client/issues/607))
- add migration for legacy btc wallets ([#604](https://github.com/BoltzExchange/boltz-client/issues/604))
- always encrypt all wallet credentials fields ([#588](https://github.com/BoltzExchange/boltz-client/issues/588))
- full scan wallet on import ([#587](https://github.com/BoltzExchange/boltz-client/issues/587))
- also update submarine lockup tx on `transaction.confirmed` ([#584](https://github.com/BoltzExchange/boltz-client/issues/584))
- only try refunding submarine swap if funds were locked ([#577](https://github.com/BoltzExchange/boltz-client/issues/577))

### Refactor
- liquid backend initialization ([#605](https://github.com/BoltzExchange/boltz-client/issues/605))
- increase default BDK sync interval ([#592](https://github.com/BoltzExchange/boltz-client/issues/592))
- use tempdir as datadir in integration tests ([#602](https://github.com/BoltzExchange/boltz-client/issues/602))
- wallet backend logs ([#603](https://github.com/BoltzExchange/boltz-client/issues/603))
- rm redundant invoice paid check ([#600](https://github.com/BoltzExchange/boltz-client/issues/600))
- use rustls for electrum ([#590](https://github.com/BoltzExchange/boltz-client/issues/590))
- always use mainnet for rescue keychain ([#585](https://github.com/BoltzExchange/boltz-client/issues/585))


<a name="v2.9.1"></a>
## [v2.9.1] - 2025-11-14

<a name="v2.9.0"></a>
## [v2.9.0] - 2025-10-21
### Feat
- add esplora http client
- retry reverse and chain claims ([#570](https://github.com/BoltzExchange/boltz-client/issues/570))
- introduce `WalletBackend` interface

### Fix
- check if chain swap is already claimed in status update
- aquire update lock before querying claimable swaps
- flaky status checks in reverse and chain tests
- check for successfull state and not status in cli
- sync loop synchronization
- check for confirmation when querying claimable swaps ([#571](https://github.com/BoltzExchange/boltz-client/issues/571))

### Refactor
- finalize chain and reverse swaps after claim ([#573](https://github.com/BoltzExchange/boltz-client/issues/573))
- rm `RequireConfirmed` flag when finding output
- simplify manual claim test
- default electrum config
- use `MultiChainProvider` in rpcserver as default
- consolidate onchain interfaces into single `ChainProvider`
- abstract basic liquid wallet tests out into onchain package
- move sync loop out of liquid wallet


<a name="v2.8.9"></a>
## [v2.8.9] - 2025-10-09
### Feat
- retry reverse and chain claims ([#570](https://github.com/BoltzExchange/boltz-client/issues/570))


<a name="v2.8.8"></a>
## [v2.8.8] - 2025-09-29
### Feat
- add timeout to mempool HTTP client ([#562](https://github.com/BoltzExchange/boltz-client/issues/562))

### Fix
- do not set reverse swap claim address without MRH ([#564](https://github.com/BoltzExchange/boltz-client/issues/564))

### Refactor
- always build lwk artifacts in make command


<a name="v2.8.7"></a>
## [v2.8.7] - 2025-09-25
### Feat
- hide mrh behind flag ([#557](https://github.com/BoltzExchange/boltz-client/issues/557))


<a name="v2.8.6"></a>
## [v2.8.6] - 2025-09-24
### Fix
- respect mempool configuration value ([#553](https://github.com/BoltzExchange/boltz-client/issues/553))
- update lwk bindings ([#551](https://github.com/BoltzExchange/boltz-client/issues/551))


<a name="v2.8.5"></a>
## [v2.8.5] - 2025-09-24
### Feat
- switch CLN from pay to xpay ([#546](https://github.com/BoltzExchange/boltz-client/issues/546))
- boltz fallback fee provider ([#539](https://github.com/BoltzExchange/boltz-client/issues/539))
- optimize fee estimation ([#543](https://github.com/BoltzExchange/boltz-client/issues/543))

### Fix
- successfull -> succesful ([#548](https://github.com/BoltzExchange/boltz-client/issues/548))
- `Stop` race condition ([#538](https://github.com/BoltzExchange/boltz-client/issues/538))
- Dockerfile base image versions ([#540](https://github.com/BoltzExchange/boltz-client/issues/540))
- backend log typo ([#532](https://github.com/BoltzExchange/boltz-client/issues/532))


<a name="v2.8.4"></a>
## [v2.8.4] - 2025-08-22

<a name="v2.8.3"></a>
## [v2.8.3] - 2025-08-20
### Feat
- allow constructing transactions with absolute fees ([#528](https://github.com/BoltzExchange/boltz-client/issues/528))

### Fix
- add Boltz API timeout ([#529](https://github.com/BoltzExchange/boltz-client/issues/529))


<a name="v2.8.2"></a>
## [v2.8.2] - 2025-07-29
### Feat
- `ignoreMrh` flag for submarine swaps ([#522](https://github.com/BoltzExchange/boltz-client/issues/522))

### Fix
- lwk create send lock ([#524](https://github.com/BoltzExchange/boltz-client/issues/524))
- lwk sync race ([#523](https://github.com/BoltzExchange/boltz-client/issues/523))


<a name="v2.8.1"></a>
## [v2.8.1] - 2025-07-21
### Feat
- validate refund address when creating submarine swap ([#521](https://github.com/BoltzExchange/boltz-client/issues/521))
- notify wallets of claim and direct transactions immediately ([#520](https://github.com/BoltzExchange/boltz-client/issues/520))
- derive keys from swap mnemonic ([#487](https://github.com/BoltzExchange/boltz-client/issues/487))

### Fix
- reset waterfalls recipient in lwk ([#514](https://github.com/BoltzExchange/boltz-client/issues/514))

### Refactor
- always broadcast with all providers in `MultiTxProvider` ([#517](https://github.com/BoltzExchange/boltz-client/issues/517))
- allow hyphens and underscores in wallet name ([#516](https://github.com/BoltzExchange/boltz-client/issues/516))


<a name="v2.8.0"></a>
## [v2.8.0] - 2025-07-16
### Feat
- populate external outputs in lwk ([#505](https://github.com/BoltzExchange/boltz-client/issues/505))
- lwk fee estimations ([#506](https://github.com/BoltzExchange/boltz-client/issues/506))
- spent txo map in lwk ([#502](https://github.com/BoltzExchange/boltz-client/issues/502))
- lwk esplora concurrency ([#501](https://github.com/BoltzExchange/boltz-client/issues/501))
- lwk server integration ([#462](https://github.com/BoltzExchange/boltz-client/issues/462))
- lwk wallet implementation ([#451](https://github.com/BoltzExchange/boltz-client/issues/451))

### Fix
- lwk `GetSendFee` send amount calculation ([#507](https://github.com/BoltzExchange/boltz-client/issues/507))
- dont include spent outputs in balance
- direct tx tests for btc ([#503](https://github.com/BoltzExchange/boltz-client/issues/503))
- do not require amount param when sweeping wallet ([#498](https://github.com/BoltzExchange/boltz-client/issues/498))
- subscribe to previous swapIds after connecting ([#491](https://github.com/BoltzExchange/boltz-client/issues/491))
- set `swap` to nil when recommended amount is smaller than reserve ([#493](https://github.com/BoltzExchange/boltz-client/issues/493))
- docs CI ([#492](https://github.com/BoltzExchange/boltz-client/issues/492))
- check if reverse swap routing fee is nil ([#488](https://github.com/BoltzExchange/boltz-client/issues/488))
- reverse swap double broadcast ([#486](https://github.com/BoltzExchange/boltz-client/issues/486))

### Refactor
- lower default sync interval to 15 seconds
- switch docs to vitepress ([#490](https://github.com/BoltzExchange/boltz-client/issues/490))


<a name="v2.7.2"></a>
## [v2.7.2] - 2025-07-03
### Fix
- subscribe to previous swapIds after connecting ([#491](https://github.com/BoltzExchange/boltz-client/issues/491))
- set `swap` to nil when recommended amount is smaller than reserve ([#493](https://github.com/BoltzExchange/boltz-client/issues/493))


<a name="v2.7.1"></a>
## [v2.7.1] - 2025-06-13
### Feat
- routing fee limit cli flag ([#479](https://github.com/BoltzExchange/boltz-client/issues/479))

### Fix
- correct name for `LightningOptions` in config ([#478](https://github.com/BoltzExchange/boltz-client/issues/478))
- routing limit cli flag typo ([#477](https://github.com/BoltzExchange/boltz-client/issues/477))
- reverse swap bip21 error handling ([#476](https://github.com/BoltzExchange/boltz-client/issues/476))


<a name="v2.7.0"></a>
## [v2.7.0] - 2025-06-13
### Feat
- max routing fee ([#473](https://github.com/BoltzExchange/boltz-client/issues/473))
- password authentication ([#461](https://github.com/BoltzExchange/boltz-client/issues/461))

### Fix
- cln feelimit unit ([#472](https://github.com/BoltzExchange/boltz-client/issues/472))
- ws subscribe race ([#471](https://github.com/BoltzExchange/boltz-client/issues/471))
- add lock around WebSocket connection ([#452](https://github.com/BoltzExchange/boltz-client/issues/452))

### Refactor
- rename password header to authorization ([#468](https://github.com/BoltzExchange/boltz-client/issues/468))
- nursery init ([#470](https://github.com/BoltzExchange/boltz-client/issues/470))
- move `Credentials` out of wallet implementation ([#455](https://github.com/BoltzExchange/boltz-client/issues/455))
- improve wallet interface ([#450](https://github.com/BoltzExchange/boltz-client/issues/450))


<a name="v2.6.1"></a>
## [v2.6.1] - 2025-05-26
### Feat
- handle unavailable backend gracefully on startup ([#444](https://github.com/BoltzExchange/boltz-client/issues/444))
- getpairs cli ([#439](https://github.com/BoltzExchange/boltz-client/issues/439))

### Fix
- save preimage for every swap ([#448](https://github.com/BoltzExchange/boltz-client/issues/448))
- save preimage of every successful swap ([#446](https://github.com/BoltzExchange/boltz-client/issues/446))
- allow readonly wallets for transactions rpc ([#445](https://github.com/BoltzExchange/boltz-client/issues/445))
- tenant command description ([#434](https://github.com/BoltzExchange/boltz-client/issues/434))
- reverse swap example ([#432](https://github.com/BoltzExchange/boltz-client/issues/432))

### Refactor
- simplify submarine swap tests ([#440](https://github.com/BoltzExchange/boltz-client/issues/440))


<a name="v2.6.0"></a>
## [v2.6.0] - 2025-04-30
### Feat
- minimal batched ([#425](https://github.com/BoltzExchange/boltz-client/issues/425))
- add `ExtraFees` in api requests ([#424](https://github.com/BoltzExchange/boltz-client/issues/424))

### Fix
- allow negative fees in stats ([#429](https://github.com/BoltzExchange/boltz-client/issues/429))


<a name="v2.5.1"></a>
## [v2.5.1] - 2025-04-14
### Fix
- allow negative service fees ([#412](https://github.com/BoltzExchange/boltz-client/issues/412))


<a name="v2.5.0"></a>
## [v2.5.0] - 2025-04-01
### Feat
- implement `IsTransactionConfirmed` for boltz ([#407](https://github.com/BoltzExchange/boltz-client/issues/407))
- `BumpTransaction` rpc ([#388](https://github.com/BoltzExchange/boltz-client/issues/388))
- use multi tx provider for liquid ([#392](https://github.com/BoltzExchange/boltz-client/issues/392))
- lazy websocket connection ([#393](https://github.com/BoltzExchange/boltz-client/issues/393))

### Fix
- bump transaction test ([#405](https://github.com/BoltzExchange/boltz-client/issues/405))
- remove invoice expiry check ([#397](https://github.com/BoltzExchange/boltz-client/issues/397))

### Refactor
- increase mainchain fee floor ([#400](https://github.com/BoltzExchange/boltz-client/issues/400))


<a name="v2.4.1"></a>
## [v2.4.1] - 2025-03-08
### Fix
- remove invoice expiry check ([#397](https://github.com/BoltzExchange/boltz-client/issues/397))


<a name="v2.4.0"></a>
## [v2.4.0] - 2025-02-17
### Feat
- enable discountct in gdk
- use discounted liquid transaction size

### Fix
- change `MaxInputs` back to 256
- do not return error when closing previous connection on reconnect ([#391](https://github.com/BoltzExchange/boltz-client/issues/391))
- bolt12 expiry date decoding


<a name="v2.3.9"></a>
## [v2.3.9] - 2025-02-13
### Fix
- do not return error when closing previous connection on reconnect ([#391](https://github.com/BoltzExchange/boltz-client/issues/391))


<a name="v2.3.8"></a>
## [v2.3.8] - 2025-02-11
### Feat
- `pro` config option ([#385](https://github.com/BoltzExchange/boltz-client/issues/385))

### Fix
- legacy `getpairs` request ([#389](https://github.com/BoltzExchange/boltz-client/issues/389))
- remove any version prefix before parsing ([#387](https://github.com/BoltzExchange/boltz-client/issues/387))


<a name="v2.3.7"></a>
## [v2.3.7] - 2025-01-22
### Fix
- update build package path ([#382](https://github.com/BoltzExchange/boltz-client/issues/382))


<a name="v2.3.6"></a>
## [v2.3.6] - 2025-01-22
### Feat
- get swap by payment hash ([#377](https://github.com/BoltzExchange/boltz-client/issues/377))

### Fix
- cln regtest ([#374](https://github.com/BoltzExchange/boltz-client/issues/374))
- docker image gdk path ([#373](https://github.com/BoltzExchange/boltz-client/issues/373))


<a name="v2.3.5"></a>
## [v2.3.5] - 2025-01-11
### Refactor
- swap tree init ([#371](https://github.com/BoltzExchange/boltz-client/issues/371))
- package structure ([#366](https://github.com/BoltzExchange/boltz-client/issues/366))


<a name="v2.3.4"></a>
## [v2.3.4] - 2024-12-17
### Feat
- multi tx provider

### Fix
- remove timeout block height from claimable swaps query
- avoid linking lightning library in cli binary ([#358](https://github.com/BoltzExchange/boltz-client/issues/358))


<a name="v2.3.3"></a>
## [v2.3.3] - 2024-12-09
### Fix
- dont allow lowball fee when paying invoice with magic routing hint


<a name="v2.3.2"></a>
## [v2.3.2] - 2024-12-06
### Fix
- check correct balance amount ([#351](https://github.com/BoltzExchange/boltz-client/issues/351))


<a name="v2.3.1"></a>
## [v2.3.1] - 2024-12-05
### Feat
- include `max_balance` in chain recommendations rpc ([#350](https://github.com/BoltzExchange/boltz-client/issues/350))
- include send fee in balance check ([#346](https://github.com/BoltzExchange/boltz-client/issues/346))
- autoswap execute rpc ([#342](https://github.com/BoltzExchange/boltz-client/issues/342))
- chain autoswap sweeps ([#341](https://github.com/BoltzExchange/boltz-client/issues/341))
- autoswap balance check ([#336](https://github.com/BoltzExchange/boltz-client/issues/336))

### Fix
- change default mempool liquid api to bull bitcoin ([#349](https://github.com/BoltzExchange/boltz-client/issues/349))
- run auto consolidation on startup ([#345](https://github.com/BoltzExchange/boltz-client/issues/345))
- decrease `MaxInputs` by 1 ([#343](https://github.com/BoltzExchange/boltz-client/issues/343))
- add gdk fee floor ([#344](https://github.com/BoltzExchange/boltz-client/issues/344))
- check for existing submarine swaps before creation ([#340](https://github.com/BoltzExchange/boltz-client/issues/340))
- check for same currency on existing credentials aswell ([#335](https://github.com/BoltzExchange/boltz-client/issues/335))
- include refunded swaps in `FailedSwapsQuery` ([#338](https://github.com/BoltzExchange/boltz-client/issues/338))

### Refactor
- check recommendations against previously accepted when executing ([#347](https://github.com/BoltzExchange/boltz-client/issues/347))
- increase default ln autoswap reserve to 10 percent ([#337](https://github.com/BoltzExchange/boltz-client/issues/337))


<a name="v2.2.3"></a>
## [v2.2.3] - 2024-12-04
### Fix
- change default mempool liquid api to bull bitcoin ([#349](https://github.com/BoltzExchange/boltz-client/issues/349))
- decrease `MaxInputs` by 1 ([#343](https://github.com/BoltzExchange/boltz-client/issues/343))
- add gdk fee floor ([#344](https://github.com/BoltzExchange/boltz-client/issues/344))


<a name="v2.3.0"></a>
## [v2.3.0] - 2024-11-29
### Feat
- include send fee in balance check ([#346](https://github.com/BoltzExchange/boltz-client/issues/346))
- autoswap execute rpc ([#342](https://github.com/BoltzExchange/boltz-client/issues/342))
- chain autoswap sweeps ([#341](https://github.com/BoltzExchange/boltz-client/issues/341))
- autoswap balance check ([#336](https://github.com/BoltzExchange/boltz-client/issues/336))

### Fix
- run auto consolidation on startup ([#345](https://github.com/BoltzExchange/boltz-client/issues/345))
- decrease `MaxInputs` by 1 ([#343](https://github.com/BoltzExchange/boltz-client/issues/343))
- add gdk fee floor ([#344](https://github.com/BoltzExchange/boltz-client/issues/344))
- check for existing submarine swaps before creation ([#340](https://github.com/BoltzExchange/boltz-client/issues/340))
- check for same currency on existing credentials aswell ([#335](https://github.com/BoltzExchange/boltz-client/issues/335))
- include refunded swaps in `FailedSwapsQuery` ([#338](https://github.com/BoltzExchange/boltz-client/issues/338))

### Refactor
- increase default ln autoswap reserve to 10 percent ([#337](https://github.com/BoltzExchange/boltz-client/issues/337))


<a name="v2.2.2"></a>
## [v2.2.2] - 2024-11-27
### Fix
- decrease `MaxInputs` by 1 ([#343](https://github.com/BoltzExchange/boltz-client/issues/343))
- add gdk fee floor ([#344](https://github.com/BoltzExchange/boltz-client/issues/344))


<a name="v2.2.1"></a>
## [v2.2.1] - 2024-11-15
### Fix
- respect `send_all` rpc param in `WalletSend` ([#333](https://github.com/BoltzExchange/boltz-client/issues/333))
- dont allow lowball in `WalletSend` ([#332](https://github.com/BoltzExchange/boltz-client/issues/332))


<a name="v2.2.0"></a>
## [v2.2.0] - 2024-11-08
### Feat
- amountless chain swaps ([#291](https://github.com/BoltzExchange/boltz-client/issues/291))
- `GetSendFee` rpc ([#325](https://github.com/BoltzExchange/boltz-client/issues/325))
- bolt12 support for submarine swaps ([#309](https://github.com/BoltzExchange/boltz-client/issues/309))

### Fix
- flaky direct payment test
- flaky recovery test
- make sure channel is being received on in remove call

### Refactor
- move `findVout` function to onchain package ([#321](https://github.com/BoltzExchange/boltz-client/issues/321))


<a name="v2.1.11"></a>
## [v2.1.11] - 2024-10-22
### Fix
- check if chosenOutput is nil when looking for direct payments ([#319](https://github.com/BoltzExchange/boltz-client/issues/319))


<a name="v2.1.10"></a>
## [v2.1.10] - 2024-10-19
### Feat
- support creating swaps with lnurls and lnaddresses ([#308](https://github.com/BoltzExchange/boltz-client/issues/308))

### Fix
- handle multiple direct payments to same address correctly ([#315](https://github.com/BoltzExchange/boltz-client/issues/315))
- force reconnect if boltz ws doesnt respond ([#317](https://github.com/BoltzExchange/boltz-client/issues/317))
- stop server during lightning connection loop aswell ([#314](https://github.com/BoltzExchange/boltz-client/issues/314))


<a name="v2.1.9"></a>
## [v2.1.9] - 2024-10-14
### Fix
- slightly increase gdk fee estimation ([#312](https://github.com/BoltzExchange/boltz-client/issues/312))
- race condition on channel forwarder removal ([#310](https://github.com/BoltzExchange/boltz-client/issues/310))


<a name="v2.1.8"></a>
## [v2.1.8] - 2024-10-13
### Feat
- auto consolidate ([#306](https://github.com/BoltzExchange/boltz-client/issues/306))

### Fix
- flaky wallet transaction tests ([#307](https://github.com/BoltzExchange/boltz-client/issues/307))
- remove unnecessary rescan ([#302](https://github.com/BoltzExchange/boltz-client/issues/302))


<a name="v2.1.7"></a>
## [v2.1.7] - 2024-10-04
### Feat
- subaccount core descriptors in rpc ([#293](https://github.com/BoltzExchange/boltz-client/issues/293))
- split core descriptors by new line ([#296](https://github.com/BoltzExchange/boltz-client/issues/296))
- `ListWalletTransactions` rpc ([#286](https://github.com/BoltzExchange/boltz-client/issues/286))
- custom reverse swap invoice expiry ([#290](https://github.com/BoltzExchange/boltz-client/issues/290))
- allow insecure lnd connection ([#288](https://github.com/BoltzExchange/boltz-client/issues/288))

### Fix
- add missing `AllowReadonly` flags for wallets ([#301](https://github.com/BoltzExchange/boltz-client/issues/301))
- change test order ([#299](https://github.com/BoltzExchange/boltz-client/issues/299))
- allow readonly wallets for receiving and chain swap destinations ([#294](https://github.com/BoltzExchange/boltz-client/issues/294))
- update internal blockHeight state before sending updates to subscribers ([#297](https://github.com/BoltzExchange/boltz-client/issues/297))
- apply regtest patch ([#298](https://github.com/BoltzExchange/boltz-client/issues/298))


<a name="v2.1.6"></a>
## [v2.1.6] - 2024-09-13
### Feat
- allow specifying `all` tenant
- add lock for `spentOutputs`
- listswaps pagination ([#280](https://github.com/BoltzExchange/boltz-client/issues/280))

### Fix
- block creation of `all` tenant ([#287](https://github.com/BoltzExchange/boltz-client/issues/287))
- dont fetch empty lockup txs
- track unspent tx outputs in gdk

### Refactor
- add constant for global tenant


<a name="v2.1.5"></a>
## [v2.1.5] - 2024-09-04
### Feat
- add keepalive policy to grpc server ([#279](https://github.com/BoltzExchange/boltz-client/issues/279))

### Fix
- only spend confirmed utxos ([#278](https://github.com/BoltzExchange/boltz-client/issues/278))


<a name="v2.1.4"></a>
## [v2.1.4] - 2024-08-27
### Fix
- dont allow lowball when paying submarine swap directly ([#277](https://github.com/BoltzExchange/boltz-client/issues/277))


<a name="v2.1.3"></a>
## [v2.1.3] - 2024-08-26
### Feat
- add description hash option for reverse swaps ([#271](https://github.com/BoltzExchange/boltz-client/issues/271))

### Fix
- gdk memory leak ([#276](https://github.com/BoltzExchange/boltz-client/issues/276))
- use zero conf when invoice expiry is less than block time ([#270](https://github.com/BoltzExchange/boltz-client/issues/270))
- revert tx broadcaster ([#273](https://github.com/BoltzExchange/boltz-client/issues/273))
- check for zero amount invoices ([#269](https://github.com/BoltzExchange/boltz-client/issues/269))

### Refactor
- make status logs less verbose ([#275](https://github.com/BoltzExchange/boltz-client/issues/275))
- dont log private info ([#274](https://github.com/BoltzExchange/boltz-client/issues/274))
- decrease log verbosity ([#267](https://github.com/BoltzExchange/boltz-client/issues/267))


<a name="v2.1.2"></a>
## [v2.1.2] - 2024-08-24
### Feat
- properly implement magic routing hints ([#264](https://github.com/BoltzExchange/boltz-client/issues/264))
- chain autoswap reserve balance ([#259](https://github.com/BoltzExchange/boltz-client/issues/259))
- `tenant` flag in cli ([#262](https://github.com/BoltzExchange/boltz-client/issues/262))
- send and receive rpcs for wallets ([#261](https://github.com/BoltzExchange/boltz-client/issues/261))
- resubscribe to swaps when reconnecting websocket ([#258](https://github.com/BoltzExchange/boltz-client/issues/258))
- regular pings for Boltz WebSocket ([#252](https://github.com/BoltzExchange/boltz-client/issues/252))
- check wallet balance before creating swaps to avoid unnecessary swap creation ([#244](https://github.com/BoltzExchange/boltz-client/issues/244))
- add `is_auto' to gRPC swap messages ([#247](https://github.com/BoltzExchange/boltz-client/issues/247))
- release helper in Makefile ([#246](https://github.com/BoltzExchange/boltz-client/issues/246))

### Fix
- check for duplicate wallet names ([#265](https://github.com/BoltzExchange/boltz-client/issues/265))
- pass referralId to server ([#263](https://github.com/BoltzExchange/boltz-client/issues/263))
- include fatal logs in log file ([#256](https://github.com/BoltzExchange/boltz-client/issues/256))
- check for nil balance when serializing subaccount ([#250](https://github.com/BoltzExchange/boltz-client/issues/250))
- deserialization of max 0-conf amount ([#248](https://github.com/BoltzExchange/boltz-client/issues/248))

### Refactor
- improve error handling ([#249](https://github.com/BoltzExchange/boltz-client/issues/249))
- convert remaining budget to uint64 ([#245](https://github.com/BoltzExchange/boltz-client/issues/245))
- remember last block height ([#251](https://github.com/BoltzExchange/boltz-client/issues/251))
- fetch pairs in parallel in `GetPairs` ([#254](https://github.com/BoltzExchange/boltz-client/issues/254))


<a name="v2.1.1"></a>
## [v2.1.1] - 2024-08-13
### Feat
- `ReferralId` config option ([#243](https://github.com/BoltzExchange/boltz-client/issues/243))
- add check against max zero conf amount
- lowball flags for lockup transactions of submarine and chain swaps
- rotating logs ([#238](https://github.com/BoltzExchange/boltz-client/issues/238))
- stats gRPC ([#222](https://github.com/BoltzExchange/boltz-client/issues/222))
- include lightning and wallet balances in autoswap recommendations ([#231](https://github.com/BoltzExchange/boltz-client/issues/231))
- use lowball fee on liquid ([#234](https://github.com/BoltzExchange/boltz-client/issues/234))
- handle shutdown signals gracefully ([#230](https://github.com/BoltzExchange/boltz-client/issues/230))
- add config option for cln server name ([#219](https://github.com/BoltzExchange/boltz-client/issues/219))
- improve rpc startup error states ([#216](https://github.com/BoltzExchange/boltz-client/issues/216))
- ClaimSwaps gRPC method ([#198](https://github.com/BoltzExchange/boltz-client/issues/198))
- custom reverse swap invoice description ([#208](https://github.com/BoltzExchange/boltz-client/issues/208))
- Liquid readonly wallets ([#203](https://github.com/BoltzExchange/boltz-client/issues/203))
- warning in autoswap setup if selected wallet has no balance ([#210](https://github.com/BoltzExchange/boltz-client/issues/210))
- download gdk dynamically ([#209](https://github.com/BoltzExchange/boltz-client/issues/209))
- remember time of reverse swap payment ([#207](https://github.com/BoltzExchange/boltz-client/issues/207))
- add more validators and `maxFeePercent` to autoswap setup ([#199](https://github.com/BoltzExchange/boltz-client/issues/199))

### Fix
- correct service fee calculation for submarine swaps ([#242](https://github.com/BoltzExchange/boltz-client/issues/242))
- static GDK artifact ([#239](https://github.com/BoltzExchange/boltz-client/issues/239))
- pass configured electrum url to gdk ([#235](https://github.com/BoltzExchange/boltz-client/issues/235))
- only return complete autoswap config if no tenant is specified ([#226](https://github.com/BoltzExchange/boltz-client/issues/226))
- dont error in `GetConfig` rpc when autoswap is not configured ([#220](https://github.com/BoltzExchange/boltz-client/issues/220))
- automatically try to resync wallet subaccounts when not found ([#215](https://github.com/BoltzExchange/boltz-client/issues/215))
- correct message when asking for wallet currency in autoswap setup ([#213](https://github.com/BoltzExchange/boltz-client/issues/213))
- check if wallet params are nil ([#212](https://github.com/BoltzExchange/boltz-client/issues/212))
- skip invoice status check if it does not come from own node ([#211](https://github.com/BoltzExchange/boltz-client/issues/211))
- do not check status for externally paid invoices ([#206](https://github.com/BoltzExchange/boltz-client/issues/206))
- make sure name is not empty when removing wallet in cli ([#197](https://github.com/BoltzExchange/boltz-client/issues/197))


<a name="v2.1.0"></a>
## [v2.1.0] - 2024-07-11
### Feat
- tenant checks on swapinfo and refundswap ([#190](https://github.com/BoltzExchange/boltz-client/issues/190))
- use boltz endpoint on liquid by default for lower fees ([#187](https://github.com/BoltzExchange/boltz-client/issues/187))
- minor autoswap improvements ([#183](https://github.com/BoltzExchange/boltz-client/issues/183))
- improve error message when wallet is not found ([#175](https://github.com/BoltzExchange/boltz-client/issues/175))
- make sure all gdk accounts are synced on startup ([#171](https://github.com/BoltzExchange/boltz-client/issues/171))
- chain autoswap ([#139](https://github.com/BoltzExchange/boltz-client/issues/139))
- global swapinfo stream in cli ([#177](https://github.com/BoltzExchange/boltz-client/issues/177))
- allow manual refunds to wallets ([#167](https://github.com/BoltzExchange/boltz-client/issues/167))
- test invalid boltz data ([#156](https://github.com/BoltzExchange/boltz-client/issues/156))
- request proxy for http and ws api ([#155](https://github.com/BoltzExchange/boltz-client/issues/155))
- magic routing hint support ([#146](https://github.com/BoltzExchange/boltz-client/issues/146))
- initial chainswaps ([#133](https://github.com/BoltzExchange/boltz-client/issues/133))
- use `row` interface to simplify db logic
- allow baking admin macaroons
- show refundable swaps in cli ([#132](https://github.com/BoltzExchange/boltz-client/issues/132))
- more fine grained permissions
- entity related rpc implementation
- allow specifying basic permissions in rpc
- entity parameter for listswaps and getwallets rpc
- macaroon based entity authentication
- standalone mode ([#123](https://github.com/BoltzExchange/boltz-client/issues/123))

### Fix
- make sure cli doesnt depend on gdk ([#200](https://github.com/BoltzExchange/boltz-client/issues/200))
- properly wait for sync in unlock test
- add space ([#193](https://github.com/BoltzExchange/boltz-client/issues/193))
- cli fees ([#191](https://github.com/BoltzExchange/boltz-client/issues/191))
- How do you want to specify min/max balance values? -> How do you… ([#181](https://github.com/BoltzExchange/boltz-client/issues/181))
- enforce empty wallet name ([#185](https://github.com/BoltzExchange/boltz-client/issues/185))
- ignore other threshold if only one swap type is allowed ([#182](https://github.com/BoltzExchange/boltz-client/issues/182))
- set swap to error state if paying internally fails ([#170](https://github.com/BoltzExchange/boltz-client/issues/170))
- correct wallet cli flag names ([#159](https://github.com/BoltzExchange/boltz-client/issues/159))
- round up fee by 1 ([#153](https://github.com/BoltzExchange/boltz-client/issues/153))
- register swap listeners when recovering pending swaps
- disable rbf when sending from internal wallets ([#142](https://github.com/BoltzExchange/boltz-client/issues/142))
- only refund immediately if coins were locked up ([#149](https://github.com/BoltzExchange/boltz-client/issues/149))
- changepassword help description ([#135](https://github.com/BoltzExchange/boltz-client/issues/135))
- permissions for `GetPairs` rpc
- proper error check in grpc client ([#131](https://github.com/BoltzExchange/boltz-client/issues/131))
- correct error check when cleaning up old tls cert and key

### Refactor
- rename entity to tenant ([#188](https://github.com/BoltzExchange/boltz-client/issues/188))
- package binaries using tar instead of zip ([#172](https://github.com/BoltzExchange/boltz-client/issues/172))
- improve wallets rpc ([#158](https://github.com/BoltzExchange/boltz-client/issues/158))
- do not implicitly use nil entity as admin default ([#154](https://github.com/BoltzExchange/boltz-client/issues/154))
- simplify internal onchain interfaces ([#147](https://github.com/BoltzExchange/boltz-client/issues/147))
- improve refundable swap queries ([#150](https://github.com/BoltzExchange/boltz-client/issues/150))
- add streamStatus in tests ([#144](https://github.com/BoltzExchange/boltz-client/issues/144))
- unify pair rpcs ([#143](https://github.com/BoltzExchange/boltz-client/issues/143))
- allow specifying entity as request header
- use ids instead of names for wallet and entity in rpc
- rename `SetId` to `SetupWallet`
- per-output addresses for transactions ([#130](https://github.com/BoltzExchange/boltz-client/issues/130))


<a name="v2.0.2"></a>
## [v2.0.2] - 2024-03-26
### Feat
- global swap info stream ([#124](https://github.com/BoltzExchange/boltz-client/issues/124))
- allow external pay for reverse swaps ([#121](https://github.com/BoltzExchange/boltz-client/issues/121))

### Fix
- allow null values for `Wallet` and `ExternalPay` in swap db ([#128](https://github.com/BoltzExchange/boltz-client/issues/128))


<a name="v2.0.1"></a>
## [v2.0.1] - 2024-03-19
### Feat
- noTls option and cors for rpcserver ([#102](https://github.com/BoltzExchange/boltz-client/issues/102))


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


[v2.10.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.9.1...v2.10.0
[v2.9.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.9.0...v2.9.1
[v2.9.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.9...v2.9.0
[v2.8.9]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.8...v2.8.9
[v2.8.8]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.7...v2.8.8
[v2.8.7]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.6...v2.8.7
[v2.8.6]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.5...v2.8.6
[v2.8.5]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.4...v2.8.5
[v2.8.4]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.3...v2.8.4
[v2.8.3]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.2...v2.8.3
[v2.8.2]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.1...v2.8.2
[v2.8.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.8.0...v2.8.1
[v2.8.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.7.2...v2.8.0
[v2.7.2]: https://github.com/BoltzExchange/boltz-client/compare/v2.7.1...v2.7.2
[v2.7.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.7.0...v2.7.1
[v2.7.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.6.1...v2.7.0
[v2.6.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.6.0...v2.6.1
[v2.6.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.5.1...v2.6.0
[v2.5.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.5.0...v2.5.1
[v2.5.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.4.1...v2.5.0
[v2.4.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.4.0...v2.4.1
[v2.4.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.9...v2.4.0
[v2.3.9]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.8...v2.3.9
[v2.3.8]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.7...v2.3.8
[v2.3.7]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.6...v2.3.7
[v2.3.6]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.5...v2.3.6
[v2.3.5]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.4...v2.3.5
[v2.3.4]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.3...v2.3.4
[v2.3.3]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.2...v2.3.3
[v2.3.2]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.1...v2.3.2
[v2.3.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.2.3...v2.3.1
[v2.2.3]: https://github.com/BoltzExchange/boltz-client/compare/v2.3.0...v2.2.3
[v2.3.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.2.2...v2.3.0
[v2.2.2]: https://github.com/BoltzExchange/boltz-client/compare/v2.2.1...v2.2.2
[v2.2.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.2.0...v2.2.1
[v2.2.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.11...v2.2.0
[v2.1.11]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.10...v2.1.11
[v2.1.10]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.9...v2.1.10
[v2.1.9]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.8...v2.1.9
[v2.1.8]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.7...v2.1.8
[v2.1.7]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.6...v2.1.7
[v2.1.6]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.5...v2.1.6
[v2.1.5]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.4...v2.1.5
[v2.1.4]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.3...v2.1.4
[v2.1.3]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.2...v2.1.3
[v2.1.2]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.1...v2.1.2
[v2.1.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.1.0...v2.1.1
[v2.1.0]: https://github.com/BoltzExchange/boltz-client/compare/v2.0.2...v2.1.0
[v2.0.2]: https://github.com/BoltzExchange/boltz-client/compare/v2.0.1...v2.0.2
[v2.0.1]: https://github.com/BoltzExchange/boltz-client/compare/v2.0.0...v2.0.1
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
