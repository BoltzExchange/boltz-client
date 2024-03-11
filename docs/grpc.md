# gRPC Documentation




This page was automatically generated based on the protobuf file `boltzrpc.proto`.

Paths for the REST proxy of the gRPC interface can be found [here](https://github.com/BoltzExchange/boltz-client/blob/master/boltzrpc/rest-annotations.yaml).


## boltzrpc.Boltz
<a name="boltzrpc-Boltz"></a>


### Methods
#### GetInfo

Gets general information about the daemon like the chain of the LND node it is connected to and the IDs of pending swaps.

| Request | Response |
| ------- | -------- |
| [`GetInfoRequest`](#boltzrpc-GetInfoRequest) | [`GetInfoResponse`](#boltzrpc-GetInfoResponse) |

#### GetServiceInfo

Fetches the latest limits and fees from the Boltz backend API it is connected to.

| Request | Response |
| ------- | -------- |
| [`GetServiceInfoRequest`](#boltzrpc-GetServiceInfoRequest) | [`GetServiceInfoResponse`](#boltzrpc-GetServiceInfoResponse) |

#### GetSubmarinePair

Fetches information about a specific pair for a submarine swap.

| Request | Response |
| ------- | -------- |
| [`Pair`](#boltzrpc-Pair) | [`SubmarinePair`](#boltzrpc-SubmarinePair) |

#### GetReversePair

Fetches information about a specific pair for a reverse swap.

| Request | Response |
| ------- | -------- |
| [`Pair`](#boltzrpc-Pair) | [`ReversePair`](#boltzrpc-ReversePair) |

#### GetPairs

Fetches all available pairs for submarine and reverse swaps.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#google-protobuf-Empty) | [`GetPairsResponse`](#boltzrpc-GetPairsResponse) |

#### GetFeeEstimation

Fetches the latest limits and fees from the Boltz backend API it is connected to.

| Request | Response |
| ------- | -------- |
| [`GetFeeEstimationRequest`](#boltzrpc-GetFeeEstimationRequest) | [`GetFeeEstimationResponse`](#boltzrpc-GetFeeEstimationResponse) |

#### ListSwaps

Returns a list of all swaps, reverse swaps and channel creations in the database.

| Request | Response |
| ------- | -------- |
| [`ListSwapsRequest`](#boltzrpc-ListSwapsRequest) | [`ListSwapsResponse`](#boltzrpc-ListSwapsResponse) |

#### RefundSwap

Refund a failed swap manually. This is only required when no refund address has been set or the daemon has no wallet for the currency.

| Request | Response |
| ------- | -------- |
| [`RefundSwapRequest`](#boltzrpc-RefundSwapRequest) | [`GetSwapInfoResponse`](#boltzrpc-GetSwapInfoResponse) |

#### GetSwapInfo

Gets all available information about a swap from the database.

| Request | Response |
| ------- | -------- |
| [`GetSwapInfoRequest`](#boltzrpc-GetSwapInfoRequest) | [`GetSwapInfoResponse`](#boltzrpc-GetSwapInfoResponse) |

#### GetSwapInfoStream

Returns the entire history of the swap if is still pending and streams updates in real time.

| Request | Response |
| ------- | -------- |
| [`GetSwapInfoRequest`](#boltzrpc-GetSwapInfoRequest) | [`GetSwapInfoResponse`](#boltzrpc-GetSwapInfoResponse) stream |

#### Deposit

This is a wrapper for channel creation swaps. The daemon only returns the ID, timeout block height and lockup address. The Boltz backend takes care of the rest. When an amount of onchain coins that is in the limits is sent to the address before the timeout block height, the daemon creates a new lightning invoice, sends it to the Boltz backend which will try to pay it and if that is not possible, create a new channel to make the swap succeed.

| Request | Response |
| ------- | -------- |
| [`DepositRequest`](#boltzrpc-DepositRequest) | [`DepositResponse`](#boltzrpc-DepositResponse) |

#### CreateSwap

Creates a new swap from onchain to lightning.

| Request | Response |
| ------- | -------- |
| [`CreateSwapRequest`](#boltzrpc-CreateSwapRequest) | [`CreateSwapResponse`](#boltzrpc-CreateSwapResponse) |

#### CreateChannel

Create a new swap from onchain to a new lightning channel. The daemon will only accept the invoice payment if the HTLCs is coming trough a new channel channel opened by Boltz.

| Request | Response |
| ------- | -------- |
| [`CreateChannelRequest`](#boltzrpc-CreateChannelRequest) | [`CreateSwapResponse`](#boltzrpc-CreateSwapResponse) |

#### CreateReverseSwap

Creates a new reverse swap from lightning to onchain. If `accept_zero_conf` is set to true in the request, the daemon will not wait until the lockup transaction from Boltz is confirmed in a block, but will claim it instantly.

| Request | Response |
| ------- | -------- |
| [`CreateReverseSwapRequest`](#boltzrpc-CreateReverseSwapRequest) | [`CreateReverseSwapResponse`](#boltzrpc-CreateReverseSwapResponse) |

#### CreateWallet

Creates a new liquid wallet and returns the mnemonic.

| Request | Response |
| ------- | -------- |
| [`CreateWalletRequest`](#boltzrpc-CreateWalletRequest) | [`WalletCredentials`](#boltzrpc-WalletCredentials) |

#### ImportWallet

Imports a liquid wallet from a mnemonic.

| Request | Response |
| ------- | -------- |
| [`ImportWalletRequest`](#boltzrpc-ImportWalletRequest) | [`Wallet`](#boltzrpc-Wallet) |

#### SetSubaccount

Sets the subaccount of the liquid wallet which will be used by the daemon.

| Request | Response |
| ------- | -------- |
| [`SetSubaccountRequest`](#boltzrpc-SetSubaccountRequest) | [`Subaccount`](#boltzrpc-Subaccount) |

#### GetSubaccounts

Returns a list of all subaccounts of the liquid wallet.

| Request | Response |
| ------- | -------- |
| [`WalletInfo`](#boltzrpc-WalletInfo) | [`GetSubaccountsResponse`](#boltzrpc-GetSubaccountsResponse) |

#### GetWallets

Returns the current balance and subaccount of the liquid wallet.

| Request | Response |
| ------- | -------- |
| [`GetWalletsRequest`](#boltzrpc-GetWalletsRequest) | [`Wallets`](#boltzrpc-Wallets) |

#### GetWallet

Returns the current balance and subaccount of the liquid wallet.

| Request | Response |
| ------- | -------- |
| [`GetWalletRequest`](#boltzrpc-GetWalletRequest) | [`Wallet`](#boltzrpc-Wallet) |

#### GetWalletCredentials

Returns the mnemonic of the liquid wallet.

| Request | Response |
| ------- | -------- |
| [`GetWalletCredentialsRequest`](#boltzrpc-GetWalletCredentialsRequest) | [`WalletCredentials`](#boltzrpc-WalletCredentials) |

#### RemoveWallet

Removes the liquid wallet from the daemon.

| Request | Response |
| ------- | -------- |
| [`RemoveWalletRequest`](#boltzrpc-RemoveWalletRequest) | [`RemoveWalletResponse`](#boltzrpc-RemoveWalletResponse) |

#### Stop

Stops the server.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#google-protobuf-Empty) | [`.google.protobuf.Empty`](#google-protobuf-Empty) |

#### Unlock

Unlocks the server.

| Request | Response |
| ------- | -------- |
| [`UnlockRequest`](#boltzrpc-UnlockRequest) | [`.google.protobuf.Empty`](#google-protobuf-Empty) |

#### VerifyWalletPassword

Check if the password is correct.

| Request | Response |
| ------- | -------- |
| [`VerifyWalletPasswordRequest`](#boltzrpc-VerifyWalletPasswordRequest) | [`VerifyWalletPasswordResponse`](#boltzrpc-VerifyWalletPasswordResponse) |

#### ChangeWalletPassword

Changes the password for wallet encryption.

| Request | Response |
| ------- | -------- |
| [`ChangeWalletPasswordRequest`](#boltzrpc-ChangeWalletPasswordRequest) | [`.google.protobuf.Empty`](#google-protobuf-Empty) |




### Messages

#### Account
<a name="boltzrpc-Account"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `type` | [`string`](#string) |  |  |
| `pointer` | [`uint64`](#uint64) |  |  |
| `balance` | [`Balance`](#Balance) |  |  |





#### Balance
<a name="boltzrpc-Balance"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `confirmed` | [`uint64`](#uint64) |  |  |
| `unconfirmed` | [`uint64`](#uint64) |  |  |





#### Budget
<a name="boltzrpc-Budget"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `remaining` | [`int64`](#int64) |  |  |
| `start_date` | [`int64`](#int64) |  |  |
| `end_date` | [`int64`](#int64) |  |  |





#### ChangeWalletPasswordRequest
<a name="boltzrpc-ChangeWalletPasswordRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `old` | [`string`](#string) |  |  |
| `new` | [`string`](#string) |  |  |





#### ChannelCreationInfo
<a name="boltzrpc-ChannelCreationInfo"></a>
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
<a name="boltzrpc-ChannelId"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `cln` | [`string`](#string) |  |  |
| `lnd` | [`uint64`](#uint64) |  |  |





#### CombinedChannelSwapInfo
<a name="boltzrpc-CombinedChannelSwapInfo"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`SwapInfo`](#SwapInfo) |  |  |
| `channel_creation` | [`ChannelCreationInfo`](#ChannelCreationInfo) |  |  |





#### CreateChannelRequest
<a name="boltzrpc-CreateChannelRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |
| `inbound_liquidity` | [`uint32`](#uint32) |  | Percentage of inbound liquidity the channel that is opened should have. 25 by default. |
| `private` | [`bool`](#bool) |  |  |





#### CreateReverseSwapRequest
<a name="boltzrpc-CreateReverseSwapRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |
| `address` | [`string`](#string) |  | If no value is set, the daemon will query a new P2WKH address from LND |
| `accept_zero_conf` | [`bool`](#bool) |  |  |
| `pair` | [`Pair`](#Pair) |  |  |
| `chan_ids` | [`string`](#string) | repeated |  |
| `wallet` | [`string`](#string) | optional |  |
| `return_immediately` | [`bool`](#bool) |  | Whether the daemon should return immediately after creating the swap or wait until the swap is successful or failed. It will always return immediately if `accept_zero_conf` is not set. |





#### CreateReverseSwapResponse
<a name="boltzrpc-CreateReverseSwapResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `lockup_address` | [`string`](#string) |  |  |
| `routing_fee_milli_sat` | [`uint64`](#uint64) | optional | Only populated when zero-conf is accepted and return_immediately is set to false |
| `claim_transaction_id` | [`string`](#string) | optional | Only populated when zero-conf is accepted and return_immediately is set to false |





#### CreateSwapRequest
<a name="boltzrpc-CreateSwapRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |
| `pair` | [`Pair`](#Pair) |  |  |
| `chan_ids` | [`string`](#string) | repeated |  |
| `auto_send` | [`bool`](#bool) |  |  |
| `refund_address` | [`string`](#string) |  |  |
| `wallet` | [`string`](#string) | optional |  |
| `invoice` | [`string`](#string) | optional |  |





#### CreateSwapResponse
<a name="boltzrpc-CreateSwapResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |
| `expected_amount` | [`int64`](#int64) |  |  |
| `bip21` | [`string`](#string) |  |  |
| `tx_id` | [`string`](#string) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `timeout_hours` | [`float`](#float) |  |  |





#### CreateWalletRequest
<a name="boltzrpc-CreateWalletRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `info` | [`WalletInfo`](#WalletInfo) |  |  |
| `password` | [`string`](#string) | optional |  |





#### DepositRequest
<a name="boltzrpc-DepositRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `inbound_liquidity` | [`uint32`](#uint32) |  | Percentage of inbound liquidity the channel that is opened in case the invoice cannot be paid should have. 25 by default. |





#### DepositResponse
<a name="boltzrpc-DepositResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |





#### Fees
<a name="boltzrpc-Fees"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner` | [`MinerFees`](#MinerFees) |  |  |





#### GetFeeEstimationRequest
<a name="boltzrpc-GetFeeEstimationRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  |  |
| `swap_type` | [`string`](#string) |  |  |
| `pair` | [`Pair`](#Pair) |  |  |





#### GetFeeEstimationResponse
<a name="boltzrpc-GetFeeEstimationResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `fees` | [`Fees`](#Fees) |  |  |
| `limits` | [`Limits`](#Limits) |  |  |





#### GetInfoRequest
<a name="boltzrpc-GetInfoRequest"></a>






#### GetInfoResponse
<a name="boltzrpc-GetInfoResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `version` | [`string`](#string) |  |  |
| `node` | [`string`](#string) |  |  |
| `network` | [`string`](#string) |  |  |
| `node_pubkey` | [`string`](#string) |  |  |
| `auto_swap_status` | [`string`](#string) |  |  |
| `block_heights` | [`GetInfoResponse.BlockHeightsEntry`](#GetInfoResponse-BlockHeightsEntry) | repeated |  |
| `symbol` | [`string`](#string) |  | **Deprecated.**  |
| `lnd_pubkey` | [`string`](#string) |  | **Deprecated.**  |
| `block_height` | [`uint32`](#uint32) |  | **Deprecated.**  |
| `pending_swaps` | [`string`](#string) | repeated | **Deprecated.**  |
| `pending_reverse_swaps` | [`string`](#string) | repeated | **Deprecated.**  |





#### GetInfoResponse.BlockHeightsEntry
<a name="boltzrpc-GetInfoResponse-BlockHeightsEntry"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `key` | [`string`](#string) |  |  |
| `value` | [`uint32`](#uint32) |  |  |





#### GetPairsResponse
<a name="boltzrpc-GetPairsResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `submarine` | [`SubmarinePair`](#SubmarinePair) | repeated |  |
| `reverse` | [`ReversePair`](#ReversePair) | repeated |  |





#### GetServiceInfoRequest
<a name="boltzrpc-GetServiceInfoRequest"></a>






#### GetServiceInfoResponse
<a name="boltzrpc-GetServiceInfoResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `fees` | [`Fees`](#Fees) |  |  |
| `limits` | [`Limits`](#Limits) |  |  |





#### GetSubaccountsRequest
<a name="boltzrpc-GetSubaccountsRequest"></a>






#### GetSubaccountsResponse
<a name="boltzrpc-GetSubaccountsResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `current` | [`uint64`](#uint64) | optional |  |
| `subaccounts` | [`Subaccount`](#Subaccount) | repeated |  |





#### GetSwapInfoRequest
<a name="boltzrpc-GetSwapInfoRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |





#### GetSwapInfoResponse
<a name="boltzrpc-GetSwapInfoResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`SwapInfo`](#SwapInfo) |  |  |
| `channel_creation` | [`ChannelCreationInfo`](#ChannelCreationInfo) |  |  |
| `reverse_swap` | [`ReverseSwapInfo`](#ReverseSwapInfo) |  |  |





#### GetSwapRecommendationsRequest
<a name="boltzrpc-GetSwapRecommendationsRequest"></a>






#### GetSwapRecommendationsResponse
<a name="boltzrpc-GetSwapRecommendationsResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swaps` | [`SwapRecommendation`](#SwapRecommendation) | repeated |  |





#### GetWalletCredentialsRequest
<a name="boltzrpc-GetWalletCredentialsRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `password` | [`string`](#string) | optional |  |





#### GetWalletRequest
<a name="boltzrpc-GetWalletRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |





#### GetWalletsRequest
<a name="boltzrpc-GetWalletsRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `currency` | [`Currency`](#Currency) | optional |  |
| `include_readonly` | [`bool`](#bool) | optional |  |





#### ImportWalletRequest
<a name="boltzrpc-ImportWalletRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `credentials` | [`WalletCredentials`](#WalletCredentials) |  |  |
| `info` | [`WalletInfo`](#WalletInfo) |  |  |
| `password` | [`string`](#string) | optional |  |





#### ImportWalletResponse
<a name="boltzrpc-ImportWalletResponse"></a>






#### LightningChannel
<a name="boltzrpc-LightningChannel"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`ChannelId`](#ChannelId) |  |  |
| `capacity` | [`uint64`](#uint64) |  |  |
| `local_sat` | [`uint64`](#uint64) |  |  |
| `remote_sat` | [`uint64`](#uint64) |  |  |
| `peer_id` | [`string`](#string) |  |  |





#### Limits
<a name="boltzrpc-Limits"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `minimal` | [`uint64`](#uint64) |  |  |
| `maximal` | [`uint64`](#uint64) |  |  |
| `maximal_zero_conf_amount` | [`uint64`](#uint64) |  |  |





#### ListSwapsRequest
<a name="boltzrpc-ListSwapsRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `from` | [`Currency`](#Currency) | optional |  |
| `to` | [`Currency`](#Currency) | optional |  |
| `is_auto` | [`bool`](#bool) | optional |  |
| `state` | [`SwapState`](#SwapState) | optional |  |





#### ListSwapsResponse
<a name="boltzrpc-ListSwapsResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swaps` | [`SwapInfo`](#SwapInfo) | repeated |  |
| `channel_creations` | [`CombinedChannelSwapInfo`](#CombinedChannelSwapInfo) | repeated |  |
| `reverse_swaps` | [`ReverseSwapInfo`](#ReverseSwapInfo) | repeated |  |





#### MinerFees
<a name="boltzrpc-MinerFees"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `normal` | [`uint32`](#uint32) |  |  |
| `reverse` | [`uint32`](#uint32) |  |  |





#### Pair
<a name="boltzrpc-Pair"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `from` | [`Currency`](#Currency) |  |  |
| `to` | [`Currency`](#Currency) |  |  |





#### RefundSwapRequest
<a name="boltzrpc-RefundSwapRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |





#### RemoveWalletRequest
<a name="boltzrpc-RemoveWalletRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |





#### RemoveWalletResponse
<a name="boltzrpc-RemoveWalletResponse"></a>






#### ReversePair
<a name="boltzrpc-ReversePair"></a>
Reverse Pair


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `pair` | [`Pair`](#Pair) |  |  |
| `hash` | [`string`](#string) |  |  |
| `rate` | [`float`](#float) |  |  |
| `limits` | [`Limits`](#Limits) |  |  |
| `fees` | [`ReversePair.Fees`](#ReversePair-Fees) |  |  |





#### ReversePair.Fees
<a name="boltzrpc-ReversePair-Fees"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner_fees` | [`ReversePair.Fees.MinerFees`](#ReversePair-Fees-MinerFees) |  |  |





#### ReversePair.Fees.MinerFees
<a name="boltzrpc-ReversePair-Fees-MinerFees"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `lockup` | [`uint64`](#uint64) |  |  |
| `claim` | [`uint64`](#uint64) |  |  |





#### ReverseSwapInfo
<a name="boltzrpc-ReverseSwapInfo"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `state` | [`SwapState`](#SwapState) |  |  |
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
| `pair` | [`Pair`](#Pair) |  |  |
| `chan_ids` | [`ChannelId`](#ChannelId) | repeated |  |
| `blinding_key` | [`string`](#string) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `service_fee` | [`uint64`](#uint64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `routing_fee_msat` | [`uint64`](#uint64) | optional |  |





#### SetSubaccountRequest
<a name="boltzrpc-SetSubaccountRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `subaccount` | [`uint64`](#uint64) | optional | The subaccount to use. If not set, a new one will be created. |





#### Subaccount
<a name="boltzrpc-Subaccount"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `balance` | [`Balance`](#Balance) |  |  |
| `pointer` | [`uint64`](#uint64) |  |  |
| `type` | [`string`](#string) |  |  |





#### SubmarinePair
<a name="boltzrpc-SubmarinePair"></a>
Submarine Pair


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `pair` | [`Pair`](#Pair) |  |  |
| `hash` | [`string`](#string) |  |  |
| `rate` | [`float`](#float) |  |  |
| `limits` | [`Limits`](#Limits) |  |  |
| `fees` | [`SubmarinePair.Fees`](#SubmarinePair-Fees) |  |  |





#### SubmarinePair.Fees
<a name="boltzrpc-SubmarinePair-Fees"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner_fees` | [`uint64`](#uint64) |  |  |





#### SwapInfo
<a name="boltzrpc-SwapInfo"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `pair` | [`Pair`](#Pair) |  |  |
| `state` | [`SwapState`](#SwapState) |  |  |
| `error` | [`string`](#string) |  |  |
| `status` | [`string`](#string) |  | Latest status message of the Boltz backend |
| `private_key` | [`string`](#string) |  |  |
| `preimage` | [`string`](#string) |  |  |
| `redeem_script` | [`string`](#string) |  |  |
| `invoice` | [`string`](#string) |  |  |
| `lockup_address` | [`string`](#string) |  |  |
| `expected_amount` | [`int64`](#int64) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `lockup_transaction_id` | [`string`](#string) |  |  |
| `refund_transaction_id` | [`string`](#string) |  | If the swap times out or fails for some other reason, the damon will automatically refund the coins sent to the `lockup_address` back to the configured wallet or the address specified in the `refund_address` field. |
| `refund_address` | [`string`](#string) | optional |  |
| `chan_ids` | [`ChannelId`](#ChannelId) | repeated |  |
| `blinding_key` | [`string`](#string) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `service_fee` | [`uint64`](#uint64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `auto_send` | [`bool`](#bool) |  |  |





#### SwapRecommendation
<a name="boltzrpc-SwapRecommendation"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `type` | [`string`](#string) |  |  |
| `amount` | [`uint64`](#uint64) |  |  |
| `channel` | [`LightningChannel`](#LightningChannel) |  |  |
| `fee_estimate` | [`uint64`](#uint64) |  |  |
| `dismissed_reasons` | [`string`](#string) | repeated |  |





#### SwapStats
<a name="boltzrpc-SwapStats"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total_fees` | [`uint64`](#uint64) |  |  |
| `total_amount` | [`uint64`](#uint64) |  |  |
| `avg_fees` | [`uint64`](#uint64) |  |  |
| `avg_amount` | [`uint64`](#uint64) |  |  |
| `count` | [`uint64`](#uint64) |  |  |





#### UnlockRequest
<a name="boltzrpc-UnlockRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `password` | [`string`](#string) |  |  |





#### VerifyWalletPasswordRequest
<a name="boltzrpc-VerifyWalletPasswordRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `password` | [`string`](#string) |  |  |





#### VerifyWalletPasswordResponse
<a name="boltzrpc-VerifyWalletPasswordResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `correct` | [`bool`](#bool) |  |  |





#### Wallet
<a name="boltzrpc-Wallet"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `currency` | [`Currency`](#Currency) |  |  |
| `readonly` | [`bool`](#bool) |  |  |
| `balance` | [`Balance`](#Balance) |  |  |





#### WalletCredentials
<a name="boltzrpc-WalletCredentials"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `mnemonic` | [`string`](#string) | optional | only one of these is allowed to be present |
| `xpub` | [`string`](#string) | optional |  |
| `core_descriptor` | [`string`](#string) | optional |  |
| `subaccount` | [`uint64`](#uint64) | optional | only used in combination with mnemonic |





#### WalletInfo
<a name="boltzrpc-WalletInfo"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `currency` | [`Currency`](#Currency) |  |  |





#### Wallets
<a name="boltzrpc-Wallets"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `wallets` | [`Wallet`](#Wallet) | repeated |  |






### Enums


<a name="boltzrpc-Currency"></a>

#### Currency


| Name | Number | Description |
| ---- | ------ | ----------- |
| BTC | 0 |  |
| LBTC | 1 |  |


<a name="boltzrpc-SwapState"></a>

#### SwapState


| Name | Number | Description |
| ---- | ------ | ----------- |
| PENDING | 0 |  |
| SUCCESSFUL | 1 |  |
| ERROR | 2 | Unknown client error. Check the error field of the message for more information |
| SERVER_ERROR | 3 | Unknown server error. Check the status field of the message for more information |
| REFUNDED | 4 | Client refunded locked coins after the HTLC timed out |
| ABANDONED | 5 | Client noticed that the HTLC timed out but didn't find any outputs to refund |





This page was automatically generated based on the protobuf file `autoswaprpc/autoswap.proto`.

Paths for the REST proxy of the gRPC interface can be found [here](https://github.com/BoltzExchange/boltz-client/blob/master/boltzrpc/rest-annotations.yaml).


## autoswaprpc.AutoSwap
<a name="autoswaprpc-AutoSwap"></a>


### Methods
#### GetSwapRecommendations

Returns a list of swaps which are currently recommended by the autoswapper. Also works when the autoswapper is not running.

| Request | Response |
| ------- | -------- |
| [`GetSwapRecommendationsRequest`](#autoswaprpc-GetSwapRecommendationsRequest) | [`GetSwapRecommendationsResponse`](#autoswaprpc-GetSwapRecommendationsResponse) |

#### GetStatus

Returns the current budget of the autoswapper and some relevant stats.

| Request | Response |
| ------- | -------- |
| [`GetStatusRequest`](#autoswaprpc-GetStatusRequest) | [`GetStatusResponse`](#autoswaprpc-GetStatusResponse) |

#### ResetConfig

Resets the configuration to default values.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#google-protobuf-Empty) | [`Config`](#autoswaprpc-Config) |

#### SetConfig

Allows setting multiple json-encoded config values at once. The autoswapper will reload the configuration after this call.

| Request | Response |
| ------- | -------- |
| [`Config`](#autoswaprpc-Config) | [`Config`](#autoswaprpc-Config) |

#### SetConfigValue

Allows setting a specific value in the configuration. The autoswapper will reload the configuration after this call.

| Request | Response |
| ------- | -------- |
| [`SetConfigValueRequest`](#autoswaprpc-SetConfigValueRequest) | [`Config`](#autoswaprpc-Config) |

#### GetConfig

Returns the currently used configurationencoded as json. If a key is specfied, only the value of that key will be returned.

| Request | Response |
| ------- | -------- |
| [`GetConfigRequest`](#autoswaprpc-GetConfigRequest) | [`Config`](#autoswaprpc-Config) |

#### ReloadConfig

Reloads the configuration from disk.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#google-protobuf-Empty) | [`Config`](#autoswaprpc-Config) |




### Messages

#### Budget
<a name="autoswaprpc-Budget"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `remaining` | [`int64`](#int64) |  |  |
| `start_date` | [`int64`](#int64) |  |  |
| `end_date` | [`int64`](#int64) |  |  |





#### Config
<a name="autoswaprpc-Config"></a>



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
| `currency` | [`boltzrpc.Currency`](#boltzrpc-Currency) |  |  |
| `swap_type` | [`string`](#string) |  |  |
| `per_channel` | [`bool`](#bool) |  |  |
| `wallet` | [`string`](#string) |  |  |
| `max_swap_amount` | [`uint64`](#uint64) |  |  |





#### GetConfigRequest
<a name="autoswaprpc-GetConfigRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `key` | [`string`](#string) | optional |  |





#### GetStatusRequest
<a name="autoswaprpc-GetStatusRequest"></a>






#### GetStatusResponse
<a name="autoswaprpc-GetStatusResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `running` | [`bool`](#bool) |  |  |
| `strategy` | [`string`](#string) |  |  |
| `error` | [`string`](#string) |  |  |
| `stats` | [`boltzrpc.SwapStats`](#boltzrpc-SwapStats) | optional |  |
| `budget` | [`Budget`](#Budget) | optional |  |





#### GetSwapRecommendationsRequest
<a name="autoswaprpc-GetSwapRecommendationsRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `no_dismissed` | [`bool`](#bool) | optional | Do not return any dismissed recommendations |





#### GetSwapRecommendationsResponse
<a name="autoswaprpc-GetSwapRecommendationsResponse"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swaps` | [`SwapRecommendation`](#SwapRecommendation) | repeated |  |





#### SetConfigValueRequest
<a name="autoswaprpc-SetConfigValueRequest"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `key` | [`string`](#string) |  |  |
| `value` | [`string`](#string) |  |  |





#### SwapRecommendation
<a name="autoswaprpc-SwapRecommendation"></a>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `type` | [`string`](#string) |  |  |
| `amount` | [`uint64`](#uint64) |  |  |
| `channel` | [`boltzrpc.LightningChannel`](#boltzrpc-LightningChannel) |  |  |
| `fee_estimate` | [`uint64`](#uint64) |  |  |
| `dismissed_reasons` | [`string`](#string) | repeated |  |






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

