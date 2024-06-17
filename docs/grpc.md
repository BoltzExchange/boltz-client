# gRPC Documentation

This page was automatically generated.

Paths for the REST proxy of the gRPC interface can be found [here](https://github.com/BoltzExchange/boltz-client/blob/master/boltzrpc/rest-annotations.yaml).





## boltzrpc.Boltz



### Methods
#### GetInfo

Gets general information about the daemon like the chain of the lightning node it is connected to and the IDs of pending swaps.

| Request | Response |
| ------- | -------- |
| [`GetInfoRequest`](#getinforequest) | [`GetInfoResponse`](#getinforesponse) |

#### GetServiceInfo

Fetches the latest limits and fees from the Boltz backend API it is connected to.

| Request | Response |
| ------- | -------- |
| [`GetServiceInfoRequest`](#getserviceinforequest) | [`GetServiceInfoResponse`](#getserviceinforesponse) |

#### GetPairInfo

Fetches information about a specific pair for a chain swap.

| Request | Response |
| ------- | -------- |
| [`GetPairInfoRequest`](#getpairinforequest) | [`PairInfo`](#pairinfo) |

#### GetPairs

Fetches all available pairs for submarine and reverse swaps.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#.google.protobuf.empty) | [`GetPairsResponse`](#getpairsresponse) |

#### ListSwaps

Returns a list of all swaps, reverse swaps, and chain swaps in the database.

| Request | Response |
| ------- | -------- |
| [`ListSwapsRequest`](#listswapsrequest) | [`ListSwapsResponse`](#listswapsresponse) |

#### RefundSwap

Refund a failed swap manually. This is only required when no refund address has been set or the swap does not have an associated wallet.

| Request | Response |
| ------- | -------- |
| [`RefundSwapRequest`](#refundswaprequest) | [`GetSwapInfoResponse`](#getswapinforesponse) |

#### GetSwapInfo

Gets all available information about a swap from the database.

| Request | Response |
| ------- | -------- |
| [`GetSwapInfoRequest`](#getswapinforequest) | [`GetSwapInfoResponse`](#getswapinforesponse) |

#### GetSwapInfoStream

Returns the entire history of the swap if is still pending and streams updates in real time. If the swap id is empty or "*" updates for all swaps will be streamed.

| Request | Response |
| ------- | -------- |
| [`GetSwapInfoRequest`](#getswapinforequest) | [`GetSwapInfoResponse`](#getswapinforesponse) stream |

#### Deposit

This is a wrapper for channel creation swaps. The daemon only returns the ID, timeout block height and lockup address. The Boltz backend takes care of the rest. When an amount of onchain coins that is in the limits is sent to the address before the timeout block height, the daemon creates a new lightning invoice, sends it to the Boltz backend which will try to pay it and if that is not possible, create a new channel to make the swap succeed.

| Request | Response |
| ------- | -------- |
| [`DepositRequest`](#depositrequest) | [`DepositResponse`](#depositresponse) |

#### CreateSwap

Creates a new swap from onchain to lightning.

| Request | Response |
| ------- | -------- |
| [`CreateSwapRequest`](#createswaprequest) | [`CreateSwapResponse`](#createswapresponse) |

#### CreateChannel

Create a new swap from onchain to a new lightning channel. The daemon will only accept the invoice payment if the HTLCs is coming trough a new channel channel opened by Boltz.

| Request | Response |
| ------- | -------- |
| [`CreateChannelRequest`](#createchannelrequest) | [`CreateSwapResponse`](#createswapresponse) |

#### CreateReverseSwap

Creates a new reverse swap from lightning to onchain. If `accept_zero_conf` is set to true in the request, the daemon will not wait until the lockup transaction from Boltz is confirmed in a block, but will claim it instantly.

| Request | Response |
| ------- | -------- |
| [`CreateReverseSwapRequest`](#createreverseswaprequest) | [`CreateReverseSwapResponse`](#createreverseswapresponse) |

#### CreateChainSwap

Creates a new chain swap from one chain to another. If `accept_zero_conf` is set to true in the request, the daemon will not wait until the lockup transaction from Boltz is confirmed in a block, but will claim it instantly.

| Request | Response |
| ------- | -------- |
| [`CreateChainSwapRequest`](#createchainswaprequest) | [`ChainSwapInfo`](#chainswapinfo) |

#### CreateWallet

Creates a new liquid wallet and returns the mnemonic.

| Request | Response |
| ------- | -------- |
| [`CreateWalletRequest`](#createwalletrequest) | [`CreateWalletResponse`](#createwalletresponse) |

#### ImportWallet

Imports an existing wallet.

| Request | Response |
| ------- | -------- |
| [`ImportWalletRequest`](#importwalletrequest) | [`Wallet`](#wallet) |

#### SetSubaccount

Sets the subaccount of a wallet. Not supported for readonly wallets.

| Request | Response |
| ------- | -------- |
| [`SetSubaccountRequest`](#setsubaccountrequest) | [`Subaccount`](#subaccount) |

#### GetSubaccounts

Returns all subaccounts for a given wallet. Not supported for readonly wallets.

| Request | Response |
| ------- | -------- |
| [`GetSubaccountsRequest`](#getsubaccountsrequest) | [`GetSubaccountsResponse`](#getsubaccountsresponse) |

#### GetWallets

Returns all available wallets.

| Request | Response |
| ------- | -------- |
| [`GetWalletsRequest`](#getwalletsrequest) | [`Wallets`](#wallets) |

#### GetWallet

Returns the current balance and subaccount of a wallet.

| Request | Response |
| ------- | -------- |
| [`GetWalletRequest`](#getwalletrequest) | [`Wallet`](#wallet) |

#### GetWalletCredentials

Returns the credentials of a wallet. The password will be required if the wallet is encrypted.

| Request | Response |
| ------- | -------- |
| [`GetWalletCredentialsRequest`](#getwalletcredentialsrequest) | [`WalletCredentials`](#walletcredentials) |

#### RemoveWallet

Removes a wallet.

| Request | Response |
| ------- | -------- |
| [`RemoveWalletRequest`](#removewalletrequest) | [`RemoveWalletResponse`](#removewalletresponse) |

#### Stop

Gracefully stops the daemon.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#.google.protobuf.empty) | [`.google.protobuf.Empty`](#.google.protobuf.empty) |

#### Unlock

Unlocks the server. This will be required on startup if there are any encrypted wallets.

| Request | Response |
| ------- | -------- |
| [`UnlockRequest`](#unlockrequest) | [`.google.protobuf.Empty`](#.google.protobuf.empty) |

#### VerifyWalletPassword

Check if the password is correct.

| Request | Response |
| ------- | -------- |
| [`VerifyWalletPasswordRequest`](#verifywalletpasswordrequest) | [`VerifyWalletPasswordResponse`](#verifywalletpasswordresponse) |

#### ChangeWalletPassword

Changes the password for wallet encryption.

| Request | Response |
| ------- | -------- |
| [`ChangeWalletPasswordRequest`](#changewalletpasswordrequest) | [`.google.protobuf.Empty`](#.google.protobuf.empty) |

#### CreateEntity

Creates a new entity which can be used to bake restricted macaroons.

| Request | Response |
| ------- | -------- |
| [`CreateEntityRequest`](#createentityrequest) | [`Entity`](#entity) |

#### ListEntities

Returns all entities.

| Request | Response |
| ------- | -------- |
| [`ListEntitiesRequest`](#listentitiesrequest) | [`ListEntitiesResponse`](#listentitiesresponse) |

#### GetEntity

Get a specifiy entity.

| Request | Response |
| ------- | -------- |
| [`GetEntityRequest`](#getentityrequest) | [`Entity`](#entity) |

#### BakeMacaroon

Bakes a new macaroon with the specified permissions. The macaroon can also be restricted to a specific entity. In this case, - any swap or wallet created with the returned macaroon will belong to this entity and can not be accessed by other entities. - the lightning node connected to the daemon can not be used to pay or create invoices for swaps.

| Request | Response |
| ------- | -------- |
| [`BakeMacaroonRequest`](#bakemacaroonrequest) | [`BakeMacaroonResponse`](#bakemacaroonresponse) |




### Messages

#### BakeMacaroonRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `entity_id` | [`uint64`](#uint64) | optional |  |
| `permissions` | [`MacaroonPermissions`](#macaroonpermissions) | repeated |  |





#### BakeMacaroonResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `macaroon` | [`string`](#string) |  |  |





#### Balance




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `confirmed` | [`uint64`](#uint64) |  |  |
| `unconfirmed` | [`uint64`](#uint64) |  |  |





#### BlockHeights




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `btc` | [`uint32`](#uint32) |  |  |
| `liquid` | [`uint32`](#uint32) | optional |  |





#### Budget




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `remaining` | [`int64`](#int64) |  |  |
| `start_date` | [`int64`](#int64) |  |  |
| `end_date` | [`int64`](#int64) |  |  |





#### ChainSwapData




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `currency` | [`Currency`](#currency) |  |  |
| `private_key` | [`string`](#string) |  |  |
| `their_public_key` | [`string`](#string) |  |  |
| `amount` | [`uint64`](#uint64) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `lockup_transaction_id` | [`string`](#string) | optional |  |
| `transaction_id` | [`string`](#string) | optional |  |
| `wallet_id` | [`uint64`](#uint64) | optional |  |
| `address` | [`string`](#string) | optional |  |
| `blinding_key` | [`string`](#string) | optional |  |
| `lockup_address` | [`string`](#string) |  |  |





#### ChainSwapInfo




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `pair` | [`Pair`](#pair) |  |  |
| `state` | [`SwapState`](#swapstate) |  |  |
| `error` | [`string`](#string) |  |  |
| `status` | [`string`](#string) |  |  |
| `preimage` | [`string`](#string) |  |  |
| `is_auto` | [`bool`](#bool) |  |  |
| `service_fee` | [`uint64`](#uint64) | optional |  |
| `service_fee_percent` | [`float`](#float) |  |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `entity_id` | [`uint64`](#uint64) |  |  |
| `from_data` | [`ChainSwapData`](#chainswapdata) |  |  |
| `to_data` | [`ChainSwapData`](#chainswapdata) |  |  |





#### ChangeWalletPasswordRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `old` | [`string`](#string) |  |  |
| `new` | [`string`](#string) |  |  |





#### ChannelCreationInfo

Channel creations are an optional extension to a submarine swap in the data types of boltz-client.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap_id` | [`string`](#string) |  | ID of the swap to which this channel channel belongs |
| `status` | [`string`](#string) |  |  |
| `inbound_liquidity` | [`uint32`](#uint32) |  |  |
| `private` | [`bool`](#bool) |  |  |
| `funding_transaction_id` | [`string`](#string) |  |  |
| `funding_transaction_vout` | [`uint32`](#uint32) |  |  |





#### ChannelId




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `cln` | [`string`](#string) |  | cln style: 832347x2473x1 |
| `lnd` | [`uint64`](#uint64) |  | lnd style: 915175205006540801 |





#### CombinedChannelSwapInfo




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`SwapInfo`](#swapinfo) |  |  |
| `channel_creation` | [`ChannelCreationInfo`](#channelcreationinfo) |  |  |





#### CreateChainSwapRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  | Amount of satoshis to swap. It is the amount expected to be sent to the lockup address. |
| `pair` | [`Pair`](#pair) |  |  |
| `to_address` | [`string`](#string) | optional | Address where funds will be swept to if the swap succeeds |
| `refund_address` | [`string`](#string) | optional | Address where the coins should be refunded to if the swap fails. |
| `from_wallet_id` | [`uint64`](#uint64) | optional | Wallet from which the swap should be paid from. Ignored if `external_pay` is set to true. If the swap fails, funds will be refunded to this wallet as well. |
| `to_wallet_id` | [`uint64`](#uint64) | optional | Wallet where the the funds will go if the swap succeeds. |
| `accept_zero_conf` | [`bool`](#bool) | optional | Whether the daemon should broadcast the claim transaction immediately after the lockup transaction is in the mempool. Should only be used for smaller amounts as it involves trust in Boltz. |
| `external_pay` | [`bool`](#bool) | optional | If set, the daemon will not pay the swap from an internal wallet. |





#### CreateChannelRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |
| `inbound_liquidity` | [`uint32`](#uint32) |  | Percentage of inbound liquidity the channel that is opened should have. 25 by default. |
| `private` | [`bool`](#bool) |  |  |





#### CreateEntityRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |





#### CreateReverseSwapRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  | amount of satoshis to swap |
| `address` | [`string`](#string) |  | If no value is set, the daemon will query a new address from the lightning node |
| `accept_zero_conf` | [`bool`](#bool) |  | Whether the daemon should broadcast the claim transaction immediately after the lockup transaction is in the mempool. Should only be used for smaller amounts as it involves trust in boltz. |
| `pair` | [`Pair`](#pair) |  |  |
| `chan_ids` | [`string`](#string) | repeated | a list of channel ids which are allowed for paying the invoice. can be in either cln or lnd style. |
| `wallet_id` | [`uint64`](#uint64) | optional | wallet from which the onchain address should be generated - only considered if `address` is not set |
| `return_immediately` | [`bool`](#bool) | optional | Whether the daemon should return immediately after creating the swap or wait until the swap is successful or failed. It will always return immediately if `accept_zero_conf` is not set. |
| `external_pay` | [`bool`](#bool) | optional | If set, the daemon will not pay the invoice of the swap and return the invoice to be paid. This implicitly sets `return_immediately` to true. |





#### CreateReverseSwapResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `lockup_address` | [`string`](#string) |  |  |
| `routing_fee_milli_sat` | [`uint64`](#uint64) | optional | Only populated when zero-conf is accepted and return_immediately is set to false |
| `claim_transaction_id` | [`string`](#string) | optional | Only populated when zero-conf is accepted and return_immediately is set to false |
| `invoice` | [`string`](#string) | optional | Invoice to be paid. Only populated when `external_pay` is set to true |





#### CreateSwapRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  |  |
| `pair` | [`Pair`](#pair) |  | not yet supported repeated string chan_ids = 3; |
| `send_from_internal` | [`bool`](#bool) |  | the daemon will pay the swap using the onchain wallet specified in the `wallet` field or any wallet otherwise. |
| `refund_address` | [`string`](#string) | optional | address where the coins should go if the swap fails. Refunds will go to any of the daemons wallets otherwise. |
| `wallet_id` | [`uint64`](#uint64) | optional | wallet to pay swap from. only used if `send_from_internal` is set to true |
| `invoice` | [`string`](#string) | optional | invoice to use for the swap. if not set, the daemon will get a new invoice from the lightning node |





#### CreateSwapResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |
| `expected_amount` | [`uint64`](#uint64) |  |  |
| `bip21` | [`string`](#string) |  |  |
| `tx_id` | [`string`](#string) |  | lockup transaction id. Only populated when `send_from_internal` was specified in the request |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `timeout_hours` | [`float`](#float) |  |  |





#### CreateWalletRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `params` | [`WalletParams`](#walletparams) |  |  |





#### CreateWalletResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `mnemonic` | [`string`](#string) |  |  |
| `wallet` | [`Wallet`](#wallet) |  |  |





#### DepositRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `inbound_liquidity` | [`uint32`](#uint32) |  | Percentage of inbound liquidity the channel that is opened in case the invoice cannot be paid should have. 25 by default. |





#### DepositResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |





#### Entity




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`uint64`](#uint64) |  |  |
| `name` | [`string`](#string) |  |  |





#### Fees




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner` | [`MinerFees`](#minerfees) |  |  |





#### GetEntityRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |





#### GetInfoRequest







#### GetInfoResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `version` | [`string`](#string) |  |  |
| `node` | [`string`](#string) |  |  |
| `network` | [`string`](#string) |  |  |
| `node_pubkey` | [`string`](#string) |  |  |
| `auto_swap_status` | [`string`](#string) |  | one of: running, disabled, error |
| `block_heights` | [`BlockHeights`](#blockheights) |  | mapping of the currency to the latest block height. |
| `refundable_swaps` | [`string`](#string) | repeated | swaps that need a manual interaction to refund |
| `entity` | [`Entity`](#entity) | optional | the currently authenticated entity |
| `symbol` | [`string`](#string) |  | **Deprecated.**  |
| `lnd_pubkey` | [`string`](#string) |  | **Deprecated.**  |
| `block_height` | [`uint32`](#uint32) |  | **Deprecated.**  |
| `pending_swaps` | [`string`](#string) | repeated | **Deprecated.**  |
| `pending_reverse_swaps` | [`string`](#string) | repeated | **Deprecated.**  |





#### GetPairInfoRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `type` | [`SwapType`](#swaptype) |  |  |
| `pair` | [`Pair`](#pair) |  |  |





#### GetPairsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `submarine` | [`PairInfo`](#pairinfo) | repeated |  |
| `reverse` | [`PairInfo`](#pairinfo) | repeated |  |
| `chain` | [`PairInfo`](#pairinfo) | repeated |  |





#### GetServiceInfoRequest







#### GetServiceInfoResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `fees` | [`Fees`](#fees) |  |  |
| `limits` | [`Limits`](#limits) |  |  |





#### GetSubaccountsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `wallet_id` | [`uint64`](#uint64) |  |  |





#### GetSubaccountsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `current` | [`uint64`](#uint64) | optional |  |
| `subaccounts` | [`Subaccount`](#subaccount) | repeated |  |





#### GetSwapInfoRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |





#### GetSwapInfoResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`SwapInfo`](#swapinfo) |  |  |
| `channel_creation` | [`ChannelCreationInfo`](#channelcreationinfo) |  |  |
| `reverse_swap` | [`ReverseSwapInfo`](#reverseswapinfo) |  |  |
| `chain_swap` | [`ChainSwapInfo`](#chainswapinfo) |  |  |





#### GetWalletCredentialsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`uint64`](#uint64) |  |  |
| `password` | [`string`](#string) | optional |  |





#### GetWalletRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) | optional |  |
| `id` | [`uint64`](#uint64) | optional |  |





#### GetWalletsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `currency` | [`Currency`](#currency) | optional |  |
| `include_readonly` | [`bool`](#bool) | optional |  |





#### ImportWalletRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `credentials` | [`WalletCredentials`](#walletcredentials) |  |  |
| `params` | [`WalletParams`](#walletparams) |  |  |





#### ImportWalletResponse







#### LightningChannel




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`ChannelId`](#channelid) |  |  |
| `capacity` | [`uint64`](#uint64) |  |  |
| `local_sat` | [`uint64`](#uint64) |  |  |
| `remote_sat` | [`uint64`](#uint64) |  |  |
| `peer_id` | [`string`](#string) |  |  |





#### Limits




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `minimal` | [`uint64`](#uint64) |  |  |
| `maximal` | [`uint64`](#uint64) |  |  |
| `maximal_zero_conf_amount` | [`uint64`](#uint64) |  |  |





#### ListEntitiesRequest







#### ListEntitiesResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `entities` | [`Entity`](#entity) | repeated |  |





#### ListSwapsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `from` | [`Currency`](#currency) | optional |  |
| `to` | [`Currency`](#currency) | optional |  |
| `is_auto` | [`bool`](#bool) | optional |  |
| `state` | [`SwapState`](#swapstate) | optional |  |





#### ListSwapsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swaps` | [`SwapInfo`](#swapinfo) | repeated |  |
| `channel_creations` | [`CombinedChannelSwapInfo`](#combinedchannelswapinfo) | repeated |  |
| `reverse_swaps` | [`ReverseSwapInfo`](#reverseswapinfo) | repeated |  |
| `chain_swaps` | [`ChainSwapInfo`](#chainswapinfo) | repeated |  |





#### MacaroonPermissions




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `action` | [`MacaroonAction`](#macaroonaction) |  |  |





#### MinerFees




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `normal` | [`uint32`](#uint32) |  |  |
| `reverse` | [`uint32`](#uint32) |  |  |





#### Pair




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `from` | [`Currency`](#currency) |  |  |
| `to` | [`Currency`](#currency) |  |  |





#### PairInfo




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `pair` | [`Pair`](#pair) |  |  |
| `fees` | [`SwapFees`](#swapfees) |  |  |
| `limits` | [`Limits`](#limits) |  |  |
| `hash` | [`string`](#string) |  |  |





#### RefundSwapRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |





#### RemoveWalletRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`uint64`](#uint64) |  |  |





#### RemoveWalletResponse







#### ReverseSwapInfo




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `state` | [`SwapState`](#swapstate) |  |  |
| `error` | [`string`](#string) |  |  |
| `status` | [`string`](#string) |  | Latest status message of the Boltz backend |
| `private_key` | [`string`](#string) |  |  |
| `preimage` | [`string`](#string) |  |  |
| `redeem_script` | [`string`](#string) |  |  |
| `invoice` | [`string`](#string) |  |  |
| `claim_address` | [`string`](#string) |  |  |
| `onchain_amount` | [`int64`](#int64) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `lockup_transaction_id` | [`string`](#string) |  |  |
| `claim_transaction_id` | [`string`](#string) |  |  |
| `pair` | [`Pair`](#pair) |  |  |
| `chan_ids` | [`ChannelId`](#channelid) | repeated |  |
| `blinding_key` | [`string`](#string) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `service_fee` | [`uint64`](#uint64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `routing_fee_msat` | [`uint64`](#uint64) | optional |  |
| `external_pay` | [`bool`](#bool) |  |  |
| `entity_id` | [`uint64`](#uint64) |  |  |





#### SetSubaccountRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `wallet_id` | [`uint64`](#uint64) |  |  |
| `subaccount` | [`uint64`](#uint64) | optional | The subaccount to use. If not set, a new one will be created. |





#### Subaccount




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `balance` | [`Balance`](#balance) |  |  |
| `pointer` | [`uint64`](#uint64) |  |  |
| `type` | [`string`](#string) |  |  |





#### SwapFees




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner_fees` | [`uint64`](#uint64) |  |  |





#### SwapInfo




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `pair` | [`Pair`](#pair) |  |  |
| `state` | [`SwapState`](#swapstate) |  |  |
| `error` | [`string`](#string) |  |  |
| `status` | [`string`](#string) |  | Latest status message of the Boltz backend |
| `private_key` | [`string`](#string) |  |  |
| `preimage` | [`string`](#string) |  |  |
| `redeem_script` | [`string`](#string) |  |  |
| `invoice` | [`string`](#string) |  |  |
| `lockup_address` | [`string`](#string) |  |  |
| `expected_amount` | [`uint64`](#uint64) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `lockup_transaction_id` | [`string`](#string) |  |  |
| `refund_transaction_id` | [`string`](#string) |  | If the swap times out or fails for some other reason, the damon will automatically refund the coins sent to the `lockup_address` back to the configured wallet or the address specified in the `refund_address` field. |
| `refund_address` | [`string`](#string) | optional |  |
| `chan_ids` | [`ChannelId`](#channelid) | repeated |  |
| `blinding_key` | [`string`](#string) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `service_fee` | [`uint64`](#uint64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `wallet_id` | [`uint64`](#uint64) | optional | internal wallet which was used to pay the swap |
| `entity_id` | [`uint64`](#uint64) |  |  |





#### SwapStats




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total_fees` | [`uint64`](#uint64) |  |  |
| `total_amount` | [`uint64`](#uint64) |  |  |
| `avg_fees` | [`uint64`](#uint64) |  |  |
| `avg_amount` | [`uint64`](#uint64) |  |  |
| `count` | [`uint64`](#uint64) |  |  |





#### UnlockRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `password` | [`string`](#string) |  |  |





#### VerifyWalletPasswordRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `password` | [`string`](#string) |  |  |





#### VerifyWalletPasswordResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `correct` | [`bool`](#bool) |  |  |





#### Wallet




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`uint64`](#uint64) |  |  |
| `name` | [`string`](#string) |  |  |
| `currency` | [`Currency`](#currency) |  |  |
| `readonly` | [`bool`](#bool) |  |  |
| `balance` | [`Balance`](#balance) |  |  |
| `entity_id` | [`uint64`](#uint64) |  |  |





#### WalletCredentials




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `mnemonic` | [`string`](#string) | optional | only one of these is allowed to be present |
| `xpub` | [`string`](#string) | optional |  |
| `core_descriptor` | [`string`](#string) | optional |  |
| `subaccount` | [`uint64`](#uint64) | optional | only used in combination with mnemonic |





#### WalletParams




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `currency` | [`Currency`](#currency) |  |  |
| `password` | [`string`](#string) | optional | the password to encrypt the wallet with. If there are existing encrypted wallets, the same password has to be used. |





#### Wallets




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `wallets` | [`Wallet`](#wallet) | repeated |  |






### Enums



#### Currency


| Name | Number | Description |
| ---- | ------ | ----------- |
| BTC | 0 |  |
| LBTC | 1 |  |



#### MacaroonAction


| Name | Number | Description |
| ---- | ------ | ----------- |
| READ | 0 |  |
| WRITE | 1 |  |



#### SwapState


| Name | Number | Description |
| ---- | ------ | ----------- |
| PENDING | 0 |  |
| SUCCESSFUL | 1 |  |
| ERROR | 2 | Unknown client error. Check the error field of the message for more information |
| SERVER_ERROR | 3 | Unknown server error. Check the status field of the message for more information |
| REFUNDED | 4 | Client refunded locked coins after the HTLC timed out |
| ABANDONED | 5 | Client noticed that the HTLC timed out but didn't find any outputs to refund |



#### SwapType


| Name | Number | Description |
| ---- | ------ | ----------- |
| SUBMARINE | 0 |  |
| REVERSE | 1 |  |
| CHAIN | 2 |  |






## autoswaprpc.AutoSwap



### Methods
#### GetRecommendations

Returns a list of swaps which are currently recommended by autoswap. Also works when autoswap is not running.

| Request | Response |
| ------- | -------- |
| [`GetRecommendationsRequest`](#getrecommendationsrequest) | [`GetRecommendationsResponse`](#getrecommendationsresponse) |

#### GetStatus

Returns the current budget of autoswap and some relevant stats.

| Request | Response |
| ------- | -------- |
| [`GetStatusRequest`](#getstatusrequest) | [`GetStatusResponse`](#getstatusresponse) |

#### UpdateLightningConfig

Updates the chain configuration entirely or partially. Autoswap will reload the configuration after this call.

| Request | Response |
| ------- | -------- |
| [`UpdateLightningConfigRequest`](#updatelightningconfigrequest) | [`Config`](#config) |

#### UpdateChainConfig

Updates the lightning configuration entirely or partially. Autoswap will reload the configuration after this call.

| Request | Response |
| ------- | -------- |
| [`UpdateChainConfigRequest`](#updatechainconfigrequest) | [`Config`](#config) |

#### GetConfig

Returns the currently used configurationencoded as json. If a key is specfied, only the value of that key will be returned.

| Request | Response |
| ------- | -------- |
| [`GetConfigRequest`](#getconfigrequest) | [`Config`](#config) |

#### ReloadConfig

Reloads the configuration from disk.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#.google.protobuf.empty) | [`Config`](#config) |




### Messages

#### Budget




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `remaining` | [`int64`](#int64) |  |  |
| `start_date` | [`int64`](#int64) |  |  |
| `end_date` | [`int64`](#int64) |  |  |
| `stats` | [`boltzrpc.SwapStats`](#boltzrpc.swapstats) | optional |  |





#### ChainConfig




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `enabled` | [`bool`](#bool) |  |  |
| `from_wallet` | [`string`](#string) |  |  |
| `to_wallet` | [`string`](#string) |  |  |
| `to_address` | [`string`](#string) |  |  |
| `max_balance` | [`uint64`](#uint64) |  |  |
| `max_fee_percent` | [`float`](#float) |  |  |
| `budget` | [`uint64`](#uint64) |  |  |
| `budget_interval` | [`uint64`](#uint64) |  |  |
| `entity` | [`string`](#string) | optional |  |





#### ChainRecommendation




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  |  |
| `fee_estimate` | [`uint64`](#uint64) |  |  |
| `dismissed_reasons` | [`string`](#string) | repeated | any reasons why the recommendation is not being executed |





#### Config




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `chain` | [`ChainConfig`](#chainconfig) | repeated |  |
| `lightning` | [`LightningConfig`](#lightningconfig) | repeated |  |





#### GetConfigRequest







#### GetRecommendationsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `no_dismissed` | [`bool`](#bool) | optional | Do not return any dismissed recommendations |





#### GetRecommendationsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `lightning` | [`LightningRecommendation`](#lightningrecommendation) | repeated |  |
| `chain` | [`ChainRecommendation`](#chainrecommendation) | repeated |  |





#### GetStatusRequest







#### GetStatusResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `lightning` | [`Status`](#status) | optional |  |
| `chain` | [`Status`](#status) | optional |  |
| `error` | [`string`](#string) | optional |  |





#### LightningConfig




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `enabled` | [`bool`](#bool) |  |  |
| `channel_poll_interval` | [`uint64`](#uint64) |  |  |
| `static_address` | [`string`](#string) |  |  |
| `max_balance` | [`uint64`](#uint64) |  |  |
| `min_balance` | [`uint64`](#uint64) |  |  |
| `max_balance_percent` | [`float`](#float) |  |  |
| `min_balance_percent` | [`float`](#float) |  |  |
| `max_fee_percent` | [`float`](#float) |  |  |
| `accept_zero_conf` | [`bool`](#bool) |  |  |
| `failure_backoff` | [`uint64`](#uint64) |  |  |
| `budget` | [`uint64`](#uint64) |  |  |
| `budget_interval` | [`uint64`](#uint64) |  |  |
| `currency` | [`boltzrpc.Currency`](#boltzrpc.currency) |  |  |
| `swap_type` | [`string`](#string) |  |  |
| `per_channel` | [`bool`](#bool) |  |  |
| `wallet` | [`string`](#string) |  |  |
| `max_swap_amount` | [`uint64`](#uint64) |  |  |
| `entity` | [`string`](#string) | optional |  |





#### LightningRecommendation




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  |  |
| `fee_estimate` | [`uint64`](#uint64) |  |  |
| `dismissed_reasons` | [`string`](#string) | repeated | any reasons why the recommendation is not being executed |
| `type` | [`string`](#string) |  |  |
| `channel` | [`boltzrpc.LightningChannel`](#boltzrpc.lightningchannel) |  |  |





#### Status




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `running` | [`bool`](#bool) |  |  |
| `error` | [`string`](#string) | optional |  |
| `budget` | [`Budget`](#budget) | optional |  |
| `description` | [`string`](#string) |  |  |





#### UpdateChainConfigRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `config` | [`ChainConfig`](#chainconfig) |  |  |
| `field_mask` | [`google.protobuf.FieldMask`](#google.protobuf.fieldmask) | optional |  |
| `reset` | [`bool`](#bool) | optional |  |





#### UpdateLightningConfigRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `config` | [`LightningConfig`](#lightningconfig) | optional |  |
| `field_mask` | [`google.protobuf.FieldMask`](#google.protobuf.fieldmask) |  |  |
| `reset` | [`bool`](#bool) | optional |  |






### Enums




## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <div id="double" />`double` |  | `double` | `double` | `float` | `float64` | `double` | `float` | `Float` |
| <div id="float" />`float` |  | `float` | `float` | `float` | `float32` | `float` | `float` | `Float` |
| <div id="int32" />`int32` | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | `int32` | `int` | `int` | `int32` | `int` | `integer` | `Bignum or Fixnum (as required)` |
| <div id="int64" />`int64` | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | `int64` | `long` | `int/long` | `int64` | `long` | `integer/string` | `Bignum` |
| <div id="uint32" />`uint32` | Uses variable-length encoding. | `uint32` | `int` | `int/long` | `uint32` | `uint` | `integer` | `Bignum or Fixnum (as required)` |
| <div id="uint64" />`uint64` | Uses variable-length encoding. | `uint64` | `long` | `int/long` | `uint64` | `ulong` | `integer/string` | `Bignum or Fixnum (as required)` |
| <div id="sint32" />`sint32` | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | `int32` | `int` | `int` | `int32` | `int` | `integer` | `Bignum or Fixnum (as required)` |
| <div id="sint64" />`sint64` | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | `int64` | `long` | `int/long` | `int64` | `long` | `integer/string` | `Bignum` |
| <div id="fixed32" />`fixed32` | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | `uint32` | `int` | `int` | `uint32` | `uint` | `integer` | `Bignum or Fixnum (as required)` |
| <div id="fixed64" />`fixed64` | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | `uint64` | `long` | `int/long` | `uint64` | `ulong` | `integer/string` | `Bignum` |
| <div id="sfixed32" />`sfixed32` | Always four bytes. | `int32` | `int` | `int` | `int32` | `int` | `integer` | `Bignum or Fixnum (as required)` |
| <div id="sfixed64" />`sfixed64` | Always eight bytes. | `int64` | `long` | `int/long` | `int64` | `long` | `integer/string` | `Bignum` |
| <div id="bool" />`bool` |  | `bool` | `boolean` | `boolean` | `bool` | `bool` | `boolean` | `TrueClass/FalseClass` |
| <div id="string" />`string` | A string must always contain UTF-8 encoded or 7-bit ASCII text. | `string` | `String` | `str/unicode` | `string` | `string` | `string` | `String (UTF-8)` |
| <div id="bytes" />`bytes` | May contain any arbitrary sequence of bytes. | `string` | `ByteString` | `str` | `[]byte` | `ByteString` | `string` | `String (ASCII-8BIT)` |

