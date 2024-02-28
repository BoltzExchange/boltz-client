# gRPC Documentation




This page was automatically generated based on the protobuf file `boltzrpc.proto`.

Paths for the REST proxy of the gRPC interface can be found [here](https://github.com/BoltzExchange/boltz-client/blob/master/boltzrpc/rest-annotations.yaml).


## boltzrpc.Boltz


### Methods
#### GetInfo

Gets general information about the daemon like the chain of the LND node it is connected to and the IDs of pending swaps.

| Request | Response |
| ------- | -------- |
| [`GetInfoRequest`](#boltzrpc.GetInfoRequest) | [`GetInfoResponse`](#boltzrpc.GetInfoResponse) |

#### GetServiceInfo

Fetches the latest limits and fees from the Boltz backend API it is connected to.

| Request | Response |
| ------- | -------- |
| [`GetServiceInfoRequest`](#boltzrpc.GetServiceInfoRequest) | [`GetServiceInfoResponse`](#boltzrpc.GetServiceInfoResponse) |

#### GetSubmarinePair

Fetches information about a specific pair for a submarine swap.

| Request | Response |
| ------- | -------- |
| [`Pair`](#boltzrpc.Pair) | [`SubmarinePair`](#boltzrpc.SubmarinePair) |

#### GetReversePair

Fetches information about a specific pair for a reverse swap.

| Request | Response |
| ------- | -------- |
| [`Pair`](#boltzrpc.Pair) | [`ReversePair`](#boltzrpc.ReversePair) |

#### GetPairs

Fetches all available pairs for submarine and reverse swaps.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#google.protobuf.Empty) | [`GetPairsResponse`](#boltzrpc.GetPairsResponse) |

#### GetFeeEstimation

Fetches the latest limits and fees from the Boltz backend API it is connected to.

| Request | Response |
| ------- | -------- |
| [`GetFeeEstimationRequest`](#boltzrpc.GetFeeEstimationRequest) | [`GetFeeEstimationResponse`](#boltzrpc.GetFeeEstimationResponse) |

#### ListSwaps

Returns a list of all swaps, reverse swaps and channel creations in the database.

| Request | Response |
| ------- | -------- |
| [`ListSwapsRequest`](#boltzrpc.ListSwapsRequest) | [`ListSwapsResponse`](#boltzrpc.ListSwapsResponse) |

#### GetSwapInfo

Gets all available information about a swap from the database.

| Request | Response |
| ------- | -------- |
| [`GetSwapInfoRequest`](#boltzrpc.GetSwapInfoRequest) | [`GetSwapInfoResponse`](#boltzrpc.GetSwapInfoResponse) |

#### GetSwapInfoStream

Returns the entire history of the swap if is still pending and streams updates in real time.

| Request | Response |
| ------- | -------- |
| [`GetSwapInfoRequest`](#boltzrpc.GetSwapInfoRequest) | [`GetSwapInfoResponse`](#boltzrpc.GetSwapInfoResponse) stream |

#### Deposit

This is a wrapper for channel creation swaps. The daemon only returns the ID, timeout block height and lockup address. The Boltz backend takes care of the rest. When an amount of onchain coins that is in the limits is sent to the address before the timeout block height, the daemon creates a new lightning invoice, sends it to the Boltz backend which will try to pay it and if that is not possible, create a new channel to make the swap succeed.

| Request | Response |
| ------- | -------- |
| [`DepositRequest`](#boltzrpc.DepositRequest) | [`DepositResponse`](#boltzrpc.DepositResponse) |

#### CreateSwap

Creates a new swap from onchain to lightning.

| Request | Response |
| ------- | -------- |
| [`CreateSwapRequest`](#boltzrpc.CreateSwapRequest) | [`CreateSwapResponse`](#boltzrpc.CreateSwapResponse) |

#### CreateChannel

Create a new swap from onchain to a new lightning channel. The daemon will only accept the invoice payment if the HTLCs is coming trough a new channel channel opened by Boltz.

| Request | Response |
| ------- | -------- |
| [`CreateChannelRequest`](#boltzrpc.CreateChannelRequest) | [`CreateSwapResponse`](#boltzrpc.CreateSwapResponse) |

#### CreateReverseSwap

Creates a new reverse swap from lightning to onchain. If `accept_zero_conf` is set to true in the request, the daemon will not wait until the lockup transaction from Boltz is confirmed in a block, but will claim it instantly.

| Request | Response |
| ------- | -------- |
| [`CreateReverseSwapRequest`](#boltzrpc.CreateReverseSwapRequest) | [`CreateReverseSwapResponse`](#boltzrpc.CreateReverseSwapResponse) |

#### CreateWallet

Creates a new liquid wallet and returns the mnemonic.

| Request | Response |
| ------- | -------- |
| [`CreateWalletRequest`](#boltzrpc.CreateWalletRequest) | [`WalletCredentials`](#boltzrpc.WalletCredentials) |

#### ImportWallet

Imports a liquid wallet from a mnemonic.

| Request | Response |
| ------- | -------- |
| [`ImportWalletRequest`](#boltzrpc.ImportWalletRequest) | [`Wallet`](#boltzrpc.Wallet) |

#### SetSubaccount

Sets the subaccount of the liquid wallet which will be used by the daemon.

| Request | Response |
| ------- | -------- |
| [`SetSubaccountRequest`](#boltzrpc.SetSubaccountRequest) | [`Subaccount`](#boltzrpc.Subaccount) |

#### GetSubaccounts

Returns a list of all subaccounts of the liquid wallet.

| Request | Response |
| ------- | -------- |
| [`WalletInfo`](#boltzrpc.WalletInfo) | [`GetSubaccountsResponse`](#boltzrpc.GetSubaccountsResponse) |

#### GetWallets

Returns the current balance and subaccount of the liquid wallet.

| Request | Response |
| ------- | -------- |
| [`GetWalletsRequest`](#boltzrpc.GetWalletsRequest) | [`Wallets`](#boltzrpc.Wallets) |

#### GetWallet

Returns the current balance and subaccount of the liquid wallet.

| Request | Response |
| ------- | -------- |
| [`GetWalletRequest`](#boltzrpc.GetWalletRequest) | [`Wallet`](#boltzrpc.Wallet) |

#### GetWalletCredentials

Returns the mnemonic of the liquid wallet.

| Request | Response |
| ------- | -------- |
| [`GetWalletCredentialsRequest`](#boltzrpc.GetWalletCredentialsRequest) | [`WalletCredentials`](#boltzrpc.WalletCredentials) |

#### RemoveWallet

Removes the liquid wallet from the daemon.

| Request | Response |
| ------- | -------- |
| [`RemoveWalletRequest`](#boltzrpc.RemoveWalletRequest) | [`RemoveWalletResponse`](#boltzrpc.RemoveWalletResponse) |

#### Stop

Stops the server.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#google.protobuf.Empty) | [`.google.protobuf.Empty`](#google.protobuf.Empty) |

#### Unlock

Unlocks the server.

| Request | Response |
| ------- | -------- |
| [`UnlockRequest`](#boltzrpc.UnlockRequest) | [`.google.protobuf.Empty`](#google.protobuf.Empty) |

#### VerifyWalletPassword

Check if the password is correct.

| Request | Response |
| ------- | -------- |
| [`VerifyWalletPasswordRequest`](#boltzrpc.VerifyWalletPasswordRequest) | [`VerifyWalletPasswordResponse`](#boltzrpc.VerifyWalletPasswordResponse) |

#### ChangeWalletPassword

Changes the password for wallet encryption.

| Request | Response |
| ------- | -------- |
| [`ChangeWalletPasswordRequest`](#boltzrpc.ChangeWalletPasswordRequest) | [`.google.protobuf.Empty`](#google.protobuf.Empty) |




### Messages

#### <div id="boltzrpc.Account">Account</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `type` | [`string`](#string) |  |  |
| `pointer` | [`uint64`](#uint64) |  |  |
| `balance` | [`Balance`](#boltzrpc.Balance) |  |  |





#### <div id="boltzrpc.Balance">Balance</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `confirmed` | [`uint64`](#uint64) |  |  |
| `unconfirmed` | [`uint64`](#uint64) |  |  |





#### <div id="boltzrpc.Budget">Budget</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `remaining` | [`int64`](#int64) |  |  |
| `start_date` | [`int64`](#int64) |  |  |
| `end_date` | [`int64`](#int64) |  |  |





#### <div id="boltzrpc.ChangeWalletPasswordRequest">ChangeWalletPasswordRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `old` | [`string`](#string) |  |  |
| `new` | [`string`](#string) |  |  |





#### <div id="boltzrpc.ChannelCreationInfo">ChannelCreationInfo</div>
Channel creations are an optional extension to a submarine swap in the data types of boltz-client.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap_id` | [`string`](#string) |  | ID of the swap to which this channel channel belongs |
| `status` | [`string`](#string) |  |  |
| `inbound_liquidity` | [`uint32`](#uint32) |  |  |
| `private` | [`bool`](#bool) |  |  |
| `funding_transaction_id` | [`string`](#string) |  |  |
| `funding_transaction_vout` | [`uint32`](#uint32) |  |  |





#### <div id="boltzrpc.ChannelId">ChannelId</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `cln` | [`string`](#string) |  |  |
| `lnd` | [`uint64`](#uint64) |  |  |





#### <div id="boltzrpc.CombinedChannelSwapInfo">CombinedChannelSwapInfo</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`SwapInfo`](#boltzrpc.SwapInfo) |  |  |
| `channel_creation` | [`ChannelCreationInfo`](#boltzrpc.ChannelCreationInfo) |  |  |





#### <div id="boltzrpc.CreateChannelRequest">CreateChannelRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |
| `inbound_liquidity` | [`uint32`](#uint32) |  | Percentage of inbound liquidity the channel that is opened should have. 25 by default. |
| `private` | [`bool`](#bool) |  |  |





#### <div id="boltzrpc.CreateReverseSwapRequest">CreateReverseSwapRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |
| `address` | [`string`](#string) |  | If no value is set, the daemon will query a new P2WKH address from LND |
| `accept_zero_conf` | [`bool`](#bool) |  |  |
| `pair` | [`Pair`](#boltzrpc.Pair) |  |  |
| `chan_ids` | [`string`](#string) | repeated |  |
| `wallet` | [`string`](#string) | optional |  |





#### <div id="boltzrpc.CreateReverseSwapResponse">CreateReverseSwapResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `lockup_address` | [`string`](#string) |  |  |
| `routing_fee_milli_sat` | [`uint32`](#uint32) |  | **Deprecated.**  |
| `claim_transaction_id` | [`string`](#string) |  | **Deprecated.** Only populated when 0-conf is accepted |





#### <div id="boltzrpc.CreateSwapRequest">CreateSwapRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |
| `pair` | [`Pair`](#boltzrpc.Pair) |  |  |
| `chan_ids` | [`string`](#string) | repeated |  |
| `auto_send` | [`bool`](#bool) |  |  |
| `refund_address` | [`string`](#string) |  |  |
| `wallet` | [`string`](#string) | optional |  |
| `invoice` | [`string`](#string) | optional |  |





#### <div id="boltzrpc.CreateSwapResponse">CreateSwapResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |
| `expected_amount` | [`int64`](#int64) |  |  |
| `bip21` | [`string`](#string) |  |  |
| `tx_id` | [`string`](#string) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `timeout_hours` | [`float`](#float) |  |  |





#### <div id="boltzrpc.CreateWalletRequest">CreateWalletRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `info` | [`WalletInfo`](#boltzrpc.WalletInfo) |  |  |
| `password` | [`string`](#string) | optional |  |





#### <div id="boltzrpc.DepositRequest">DepositRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `inbound_liquidity` | [`uint32`](#uint32) |  | Percentage of inbound liquidity the channel that is opened in case the invoice cannot be paid should have. 25 by default. |





#### <div id="boltzrpc.DepositResponse">DepositResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |





#### <div id="boltzrpc.Fees">Fees</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner` | [`MinerFees`](#boltzrpc.MinerFees) |  |  |





#### <div id="boltzrpc.GetFeeEstimationRequest">GetFeeEstimationRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  |  |
| `swap_type` | [`string`](#string) |  |  |
| `pair` | [`Pair`](#boltzrpc.Pair) |  |  |





#### <div id="boltzrpc.GetFeeEstimationResponse">GetFeeEstimationResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `fees` | [`Fees`](#boltzrpc.Fees) |  |  |
| `limits` | [`Limits`](#boltzrpc.Limits) |  |  |





#### <div id="boltzrpc.GetInfoRequest">GetInfoRequest</div>






#### <div id="boltzrpc.GetInfoResponse">GetInfoResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `version` | [`string`](#string) |  |  |
| `node` | [`string`](#string) |  |  |
| `network` | [`string`](#string) |  |  |
| `node_pubkey` | [`string`](#string) |  |  |
| `auto_swap_status` | [`string`](#string) |  |  |
| `block_heights` | [`GetInfoResponse.BlockHeightsEntry`](#boltzrpc.GetInfoResponse.BlockHeightsEntry) | repeated |  |
| `symbol` | [`string`](#string) |  | **Deprecated.**  |
| `lnd_pubkey` | [`string`](#string) |  | **Deprecated.**  |
| `block_height` | [`uint32`](#uint32) |  | **Deprecated.**  |
| `pending_swaps` | [`string`](#string) | repeated | **Deprecated.**  |
| `pending_reverse_swaps` | [`string`](#string) | repeated | **Deprecated.**  |





#### <div id="boltzrpc.GetInfoResponse.BlockHeightsEntry">GetInfoResponse.BlockHeightsEntry</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `key` | [`string`](#string) |  |  |
| `value` | [`uint32`](#uint32) |  |  |





#### <div id="boltzrpc.GetPairsResponse">GetPairsResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `submarine` | [`SubmarinePair`](#boltzrpc.SubmarinePair) | repeated |  |
| `reverse` | [`ReversePair`](#boltzrpc.ReversePair) | repeated |  |





#### <div id="boltzrpc.GetServiceInfoRequest">GetServiceInfoRequest</div>






#### <div id="boltzrpc.GetServiceInfoResponse">GetServiceInfoResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `fees` | [`Fees`](#boltzrpc.Fees) |  |  |
| `limits` | [`Limits`](#boltzrpc.Limits) |  |  |





#### <div id="boltzrpc.GetSubaccountsRequest">GetSubaccountsRequest</div>






#### <div id="boltzrpc.GetSubaccountsResponse">GetSubaccountsResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `current` | [`uint64`](#uint64) | optional |  |
| `subaccounts` | [`Subaccount`](#boltzrpc.Subaccount) | repeated |  |





#### <div id="boltzrpc.GetSwapInfoRequest">GetSwapInfoRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |





#### <div id="boltzrpc.GetSwapInfoResponse">GetSwapInfoResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`SwapInfo`](#boltzrpc.SwapInfo) |  |  |
| `channel_creation` | [`ChannelCreationInfo`](#boltzrpc.ChannelCreationInfo) |  |  |
| `reverse_swap` | [`ReverseSwapInfo`](#boltzrpc.ReverseSwapInfo) |  |  |





#### <div id="boltzrpc.GetSwapRecommendationsRequest">GetSwapRecommendationsRequest</div>






#### <div id="boltzrpc.GetSwapRecommendationsResponse">GetSwapRecommendationsResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swaps` | [`SwapRecommendation`](#boltzrpc.SwapRecommendation) | repeated |  |





#### <div id="boltzrpc.GetWalletCredentialsRequest">GetWalletCredentialsRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `password` | [`string`](#string) | optional |  |





#### <div id="boltzrpc.GetWalletRequest">GetWalletRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |





#### <div id="boltzrpc.GetWalletsRequest">GetWalletsRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `currency` | [`Currency`](#boltzrpc.Currency) | optional |  |
| `include_readonly` | [`bool`](#bool) | optional |  |





#### <div id="boltzrpc.ImportWalletRequest">ImportWalletRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `credentials` | [`WalletCredentials`](#boltzrpc.WalletCredentials) |  |  |
| `info` | [`WalletInfo`](#boltzrpc.WalletInfo) |  |  |
| `password` | [`string`](#string) | optional |  |





#### <div id="boltzrpc.ImportWalletResponse">ImportWalletResponse</div>






#### <div id="boltzrpc.LightningChannel">LightningChannel</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`ChannelId`](#boltzrpc.ChannelId) |  |  |
| `capacity` | [`uint64`](#uint64) |  |  |
| `local_sat` | [`uint64`](#uint64) |  |  |
| `remote_sat` | [`uint64`](#uint64) |  |  |
| `peer_id` | [`string`](#string) |  |  |





#### <div id="boltzrpc.Limits">Limits</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `minimal` | [`uint64`](#uint64) |  |  |
| `maximal` | [`uint64`](#uint64) |  |  |
| `maximal_zero_conf_amount` | [`uint64`](#uint64) |  |  |





#### <div id="boltzrpc.ListSwapsRequest">ListSwapsRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `from` | [`Currency`](#boltzrpc.Currency) | optional |  |
| `to` | [`Currency`](#boltzrpc.Currency) | optional |  |
| `is_auto` | [`bool`](#bool) | optional |  |
| `state` | [`SwapState`](#boltzrpc.SwapState) | optional |  |





#### <div id="boltzrpc.ListSwapsResponse">ListSwapsResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swaps` | [`SwapInfo`](#boltzrpc.SwapInfo) | repeated |  |
| `channel_creations` | [`CombinedChannelSwapInfo`](#boltzrpc.CombinedChannelSwapInfo) | repeated |  |
| `reverse_swaps` | [`ReverseSwapInfo`](#boltzrpc.ReverseSwapInfo) | repeated |  |





#### <div id="boltzrpc.MinerFees">MinerFees</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `normal` | [`uint32`](#uint32) |  |  |
| `reverse` | [`uint32`](#uint32) |  |  |





#### <div id="boltzrpc.Pair">Pair</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `from` | [`Currency`](#boltzrpc.Currency) |  |  |
| `to` | [`Currency`](#boltzrpc.Currency) |  |  |





#### <div id="boltzrpc.RemoveWalletRequest">RemoveWalletRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |





#### <div id="boltzrpc.RemoveWalletResponse">RemoveWalletResponse</div>






#### <div id="boltzrpc.ReversePair">ReversePair</div>
Reverse Pair


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `pair` | [`Pair`](#boltzrpc.Pair) |  |  |
| `hash` | [`string`](#string) |  |  |
| `rate` | [`float`](#float) |  |  |
| `limits` | [`Limits`](#boltzrpc.Limits) |  |  |
| `fees` | [`ReversePair.Fees`](#boltzrpc.ReversePair.Fees) |  |  |





#### <div id="boltzrpc.ReversePair.Fees">ReversePair.Fees</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner_fees` | [`ReversePair.Fees.MinerFees`](#boltzrpc.ReversePair.Fees.MinerFees) |  |  |





#### <div id="boltzrpc.ReversePair.Fees.MinerFees">ReversePair.Fees.MinerFees</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `lockup` | [`uint64`](#uint64) |  |  |
| `claim` | [`uint64`](#uint64) |  |  |





#### <div id="boltzrpc.ReverseSwapInfo">ReverseSwapInfo</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `state` | [`SwapState`](#boltzrpc.SwapState) |  |  |
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
| `pair` | [`Pair`](#boltzrpc.Pair) |  |  |
| `chan_ids` | [`ChannelId`](#boltzrpc.ChannelId) | repeated |  |
| `blinding_key` | [`string`](#string) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `service_fee` | [`uint64`](#uint64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `routing_fee_msat` | [`uint64`](#uint64) | optional |  |





#### <div id="boltzrpc.SetSubaccountRequest">SetSubaccountRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `subaccount` | [`uint64`](#uint64) | optional | The subaccount to use. If not set, a new one will be created. |





#### <div id="boltzrpc.Subaccount">Subaccount</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `balance` | [`Balance`](#boltzrpc.Balance) |  |  |
| `pointer` | [`uint64`](#uint64) |  |  |
| `type` | [`string`](#string) |  |  |





#### <div id="boltzrpc.SubmarinePair">SubmarinePair</div>
Submarine Pair


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `pair` | [`Pair`](#boltzrpc.Pair) |  |  |
| `hash` | [`string`](#string) |  |  |
| `rate` | [`float`](#float) |  |  |
| `limits` | [`Limits`](#boltzrpc.Limits) |  |  |
| `fees` | [`SubmarinePair.Fees`](#boltzrpc.SubmarinePair.Fees) |  |  |





#### <div id="boltzrpc.SubmarinePair.Fees">SubmarinePair.Fees</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner_fees` | [`uint64`](#uint64) |  |  |





#### <div id="boltzrpc.SwapInfo">SwapInfo</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `pair` | [`Pair`](#boltzrpc.Pair) |  |  |
| `state` | [`SwapState`](#boltzrpc.SwapState) |  |  |
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
| `chan_ids` | [`ChannelId`](#boltzrpc.ChannelId) | repeated |  |
| `blinding_key` | [`string`](#string) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `service_fee` | [`uint64`](#uint64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `auto_send` | [`bool`](#bool) |  |  |





#### <div id="boltzrpc.SwapRecommendation">SwapRecommendation</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `type` | [`string`](#string) |  |  |
| `amount` | [`uint64`](#uint64) |  |  |
| `channel` | [`LightningChannel`](#boltzrpc.LightningChannel) |  |  |
| `fee_estimate` | [`uint64`](#uint64) |  |  |
| `dismissed_reasons` | [`string`](#string) | repeated |  |





#### <div id="boltzrpc.SwapStats">SwapStats</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total_fees` | [`uint64`](#uint64) |  |  |
| `total_amount` | [`uint64`](#uint64) |  |  |
| `avg_fees` | [`uint64`](#uint64) |  |  |
| `avg_amount` | [`uint64`](#uint64) |  |  |
| `count` | [`uint64`](#uint64) |  |  |





#### <div id="boltzrpc.UnlockRequest">UnlockRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `password` | [`string`](#string) |  |  |





#### <div id="boltzrpc.VerifyWalletPasswordRequest">VerifyWalletPasswordRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `password` | [`string`](#string) |  |  |





#### <div id="boltzrpc.VerifyWalletPasswordResponse">VerifyWalletPasswordResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `correct` | [`bool`](#bool) |  |  |





#### <div id="boltzrpc.Wallet">Wallet</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `currency` | [`Currency`](#boltzrpc.Currency) |  |  |
| `readonly` | [`bool`](#bool) |  |  |
| `balance` | [`Balance`](#boltzrpc.Balance) |  |  |





#### <div id="boltzrpc.WalletCredentials">WalletCredentials</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `mnemonic` | [`string`](#string) | optional | only one of these is allowed to be present |
| `xpub` | [`string`](#string) | optional |  |
| `core_descriptor` | [`string`](#string) | optional |  |
| `subaccount` | [`uint64`](#uint64) | optional | only used in combination with mnemonic |





#### <div id="boltzrpc.WalletInfo">WalletInfo</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `currency` | [`Currency`](#boltzrpc.Currency) |  |  |





#### <div id="boltzrpc.Wallets">Wallets</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `wallets` | [`Wallet`](#boltzrpc.Wallet) | repeated |  |






### Enums


<a name="boltzrpc.Currency"></a>

#### Currency


| Name | Number | Description |
| ---- | ------ | ----------- |
| BTC | 0 |  |
| LBTC | 1 |  |


<a name="boltzrpc.SwapState"></a>

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


### Methods
#### GetSwapRecommendations

Returns a list of swaps which are currently recommended by the autoswapper. Also works when the autoswapper is not running.

| Request | Response |
| ------- | -------- |
| [`GetSwapRecommendationsRequest`](#autoswaprpc.GetSwapRecommendationsRequest) | [`GetSwapRecommendationsResponse`](#autoswaprpc.GetSwapRecommendationsResponse) |

#### GetStatus

Returns the current budget of the autoswapper and some relevant stats.

| Request | Response |
| ------- | -------- |
| [`GetStatusRequest`](#autoswaprpc.GetStatusRequest) | [`GetStatusResponse`](#autoswaprpc.GetStatusResponse) |

#### ResetConfig

Resets the configuration to default values.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#google.protobuf.Empty) | [`Config`](#autoswaprpc.Config) |

#### SetConfig

Allows setting multiple json-encoded config values at once. The autoswapper will reload the configuration after this call.

| Request | Response |
| ------- | -------- |
| [`Config`](#autoswaprpc.Config) | [`Config`](#autoswaprpc.Config) |

#### SetConfigValue

Allows setting a specific value in the configuration. The autoswapper will reload the configuration after this call.

| Request | Response |
| ------- | -------- |
| [`SetConfigValueRequest`](#autoswaprpc.SetConfigValueRequest) | [`Config`](#autoswaprpc.Config) |

#### GetConfig

Returns the currently used configurationencoded as json. If a key is specfied, only the value of that key will be returned.

| Request | Response |
| ------- | -------- |
| [`GetConfigRequest`](#autoswaprpc.GetConfigRequest) | [`Config`](#autoswaprpc.Config) |

#### ReloadConfig

Reloads the configuration from disk.

| Request | Response |
| ------- | -------- |
| [`.google.protobuf.Empty`](#google.protobuf.Empty) | [`Config`](#autoswaprpc.Config) |




### Messages

#### <div id="autoswaprpc.Budget">Budget</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total` | [`uint64`](#uint64) |  |  |
| `remaining` | [`int64`](#int64) |  |  |
| `start_date` | [`int64`](#int64) |  |  |
| `end_date` | [`int64`](#int64) |  |  |





#### <div id="autoswaprpc.Config">Config</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `json` | [`string`](#string) |  |  |





#### <div id="autoswaprpc.GetConfigRequest">GetConfigRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `key` | [`string`](#string) | optional |  |





#### <div id="autoswaprpc.GetStatusRequest">GetStatusRequest</div>






#### <div id="autoswaprpc.GetStatusResponse">GetStatusResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `running` | [`bool`](#bool) |  |  |
| `strategy` | [`string`](#string) |  |  |
| `error` | [`string`](#string) |  |  |
| `stats` | [`boltzrpc.SwapStats`](#boltzrpc.SwapStats) | optional |  |
| `budget` | [`Budget`](#autoswaprpc.Budget) | optional |  |





#### <div id="autoswaprpc.GetSwapRecommendationsRequest">GetSwapRecommendationsRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `no_dismissed` | [`bool`](#bool) | optional | Do not return any dismissed recommendations |





#### <div id="autoswaprpc.GetSwapRecommendationsResponse">GetSwapRecommendationsResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swaps` | [`SwapRecommendation`](#autoswaprpc.SwapRecommendation) | repeated |  |





#### <div id="autoswaprpc.SetConfigValueRequest">SetConfigValueRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `key` | [`string`](#string) |  |  |
| `value` | [`string`](#string) |  |  |





#### <div id="autoswaprpc.SwapRecommendation">SwapRecommendation</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `type` | [`string`](#string) |  |  |
| `amount` | [`uint64`](#uint64) |  |  |
| `channel` | [`boltzrpc.LightningChannel`](#boltzrpc.LightningChannel) |  |  |
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

