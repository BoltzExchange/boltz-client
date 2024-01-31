# 🤖 gRPC API

This page was automatically generated based on the protobuf file `boltzrpc.proto`.

Paths for the REST proxy of the gRPC interface can be found [here](https://github.com/BoltzExchange/boltz-client/blob/master/boltzrpc/rest-annotations.yaml).

## boltzrpc.Boltz

### Methods

#### GetInfo

Gets general information about the daemon like the chain of the LND node it is connected to and the IDs of pending swaps.

| Request                                             | Response                                              |
| --------------------------------------------------- | ----------------------------------------------------- |
| [`GetInfoRequest`](grpc.md#boltzrpc.GetInfoRequest) | [`GetInfoResponse`](grpc.md#boltzrpc.GetInfoResponse) |

#### GetServiceInfo

Fetches the latest limits and fees from the Boltz backend API it is connected to.

| Request                                                           | Response                                                            |
| ----------------------------------------------------------------- | ------------------------------------------------------------------- |
| [`GetServiceInfoRequest`](grpc.md#boltzrpc.GetServiceInfoRequest) | [`GetServiceInfoResponse`](grpc.md#boltzrpc.GetServiceInfoResponse) |

#### GetFeeEstimation

Fetches the latest limits and fees from the Boltz backend API it is connected to.

| Request                                                               | Response                                                                |
| --------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| [`GetFeeEstimationRequest`](grpc.md#boltzrpc.GetFeeEstimationRequest) | [`GetFeeEstimationResponse`](grpc.md#boltzrpc.GetFeeEstimationResponse) |

#### ListSwaps

Returns a list of all swaps, reverse swaps and channel creations in the database.

| Request                                                 | Response                                                  |
| ------------------------------------------------------- | --------------------------------------------------------- |
| [`ListSwapsRequest`](grpc.md#boltzrpc.ListSwapsRequest) | [`ListSwapsResponse`](grpc.md#boltzrpc.ListSwapsResponse) |

#### GetSwapInfo

Gets all available information about a swap from the database.

| Request                                                     | Response                                                      |
| ----------------------------------------------------------- | ------------------------------------------------------------- |
| [`GetSwapInfoRequest`](grpc.md#boltzrpc.GetSwapInfoRequest) | [`GetSwapInfoResponse`](grpc.md#boltzrpc.GetSwapInfoResponse) |

#### GetSwapInfoStream

Returns the entire history of the swap if is still pending and streams updates in real time.

| Request                                                     | Response                                                             |
| ----------------------------------------------------------- | -------------------------------------------------------------------- |
| [`GetSwapInfoRequest`](grpc.md#boltzrpc.GetSwapInfoRequest) | [`GetSwapInfoResponse`](grpc.md#boltzrpc.GetSwapInfoResponse) stream |

#### Deposit

This is a wrapper for channel creation swaps. The daemon only returns the ID, timeout block height and lockup address. The Boltz backend takes care of the rest. When an amount of onchain coins that is in the limits is sent to the address before the timeout block height, the daemon creates a new lightning invoice, sends it to the Boltz backend which will try to pay it and if that is not possible, create a new channel to make the swap succeed.

| Request                                             | Response                                              |
| --------------------------------------------------- | ----------------------------------------------------- |
| [`DepositRequest`](grpc.md#boltzrpc.DepositRequest) | [`DepositResponse`](grpc.md#boltzrpc.DepositResponse) |

#### CreateSwap

Creates a new swap from onchain to lightning.

| Request                                                   | Response                                                    |
| --------------------------------------------------------- | ----------------------------------------------------------- |
| [`CreateSwapRequest`](grpc.md#boltzrpc.CreateSwapRequest) | [`CreateSwapResponse`](grpc.md#boltzrpc.CreateSwapResponse) |

#### CreateChannel

Create a new swap from onchain to a new lightning channel. The daemon will only accept the invoice payment if the HTLCs is coming trough a new channel channel opened by Boltz.

| Request                                                         | Response                                                    |
| --------------------------------------------------------------- | ----------------------------------------------------------- |
| [`CreateChannelRequest`](grpc.md#boltzrpc.CreateChannelRequest) | [`CreateSwapResponse`](grpc.md#boltzrpc.CreateSwapResponse) |

#### CreateReverseSwap

Creates a new reverse swap from lightning to onchain. If `accept_zero_conf` is set to true in the request, the daemon will not wait until the lockup transaction from Boltz is confirmed in a block, but will claim it instantly.

| Request                                                                 | Response                                                                  |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| [`CreateReverseSwapRequest`](grpc.md#boltzrpc.CreateReverseSwapRequest) | [`CreateReverseSwapResponse`](grpc.md#boltzrpc.CreateReverseSwapResponse) |

#### CreateWallet

Creates a new liquid wallet and returns the mnemonic.

| Request                                                       | Response                                                  |
| ------------------------------------------------------------- | --------------------------------------------------------- |
| [`CreateWalletRequest`](grpc.md#boltzrpc.CreateWalletRequest) | [`WalletCredentials`](grpc.md#boltzrpc.WalletCredentials) |

#### ImportWallet

Imports a liquid wallet from a mnemonic.

| Request                                                       | Response                            |
| ------------------------------------------------------------- | ----------------------------------- |
| [`ImportWalletRequest`](grpc.md#boltzrpc.ImportWalletRequest) | [`Wallet`](grpc.md#boltzrpc.Wallet) |

#### SetSubaccount

Sets the subaccount of the liquid wallet which will be used by the daemon.

| Request                                                         | Response                                    |
| --------------------------------------------------------------- | ------------------------------------------- |
| [`SetSubaccountRequest`](grpc.md#boltzrpc.SetSubaccountRequest) | [`Subaccount`](grpc.md#boltzrpc.Subaccount) |

#### GetSubaccounts

Returns a list of all subaccounts of the liquid wallet.

| Request                                     | Response                                                            |
| ------------------------------------------- | ------------------------------------------------------------------- |
| [`WalletInfo`](grpc.md#boltzrpc.WalletInfo) | [`GetSubaccountsResponse`](grpc.md#boltzrpc.GetSubaccountsResponse) |

#### GetWallets

Returns the current balance and subaccount of the liquid wallet.

| Request                                                   | Response                              |
| --------------------------------------------------------- | ------------------------------------- |
| [`GetWalletsRequest`](grpc.md#boltzrpc.GetWalletsRequest) | [`Wallets`](grpc.md#boltzrpc.Wallets) |

#### GetWallet

Returns the current balance and subaccount of the liquid wallet.

| Request                                                 | Response                            |
| ------------------------------------------------------- | ----------------------------------- |
| [`GetWalletRequest`](grpc.md#boltzrpc.GetWalletRequest) | [`Wallet`](grpc.md#boltzrpc.Wallet) |

#### GetWalletCredentials

Returns the mnemonic of the liquid wallet.

| Request                                                                       | Response                                                  |
| ----------------------------------------------------------------------------- | --------------------------------------------------------- |
| [`GetWalletCredentialsRequest`](grpc.md#boltzrpc.GetWalletCredentialsRequest) | [`WalletCredentials`](grpc.md#boltzrpc.WalletCredentials) |

#### RemoveWallet

Removes the liquid wallet from the daemon.

| Request                                                       | Response                                                        |
| ------------------------------------------------------------- | --------------------------------------------------------------- |
| [`RemoveWalletRequest`](grpc.md#boltzrpc.RemoveWalletRequest) | [`RemoveWalletResponse`](grpc.md#boltzrpc.RemoveWalletResponse) |

#### Stop

Stops the server.

| Request                                                   | Response                                                  |
| --------------------------------------------------------- | --------------------------------------------------------- |
| [`.google.protobuf.Empty`](grpc.md#google.protobuf.Empty) | [`.google.protobuf.Empty`](grpc.md#google.protobuf.Empty) |

#### Unlock

Unlocks the server.

| Request                                           | Response                                                  |
| ------------------------------------------------- | --------------------------------------------------------- |
| [`UnlockRequest`](grpc.md#boltzrpc.UnlockRequest) | [`.google.protobuf.Empty`](grpc.md#google.protobuf.Empty) |

#### VerifyWalletPassword

Check if the password is correct.

| Request                                                                       | Response                                                                        |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| [`VerifyWalletPasswordRequest`](grpc.md#boltzrpc.VerifyWalletPasswordRequest) | [`VerifyWalletPasswordResponse`](grpc.md#boltzrpc.VerifyWalletPasswordResponse) |

#### ChangeWalletPassword

Changes the password for wallet encryption.

| Request                                                                       | Response                                                  |
| ----------------------------------------------------------------------------- | --------------------------------------------------------- |
| [`ChangeWalletPasswordRequest`](grpc.md#boltzrpc.ChangeWalletPasswordRequest) | [`.google.protobuf.Empty`](grpc.md#google.protobuf.Empty) |

### Messages

#### Account

| Field     | Type                                  | Label | Description |
| --------- | ------------------------------------- | ----- | ----------- |
| `name`    | [`string`](grpc.md#string)            |       |             |
| `type`    | [`string`](grpc.md#string)            |       |             |
| `pointer` | [`uint64`](grpc.md#uint64)            |       |             |
| `balance` | [`Balance`](grpc.md#boltzrpc.Balance) |       |             |

#### Balance

| Field         | Type                       | Label | Description |
| ------------- | -------------------------- | ----- | ----------- |
| `total`       | [`uint64`](grpc.md#uint64) |       |             |
| `confirmed`   | [`uint64`](grpc.md#uint64) |       |             |
| `unconfirmed` | [`uint64`](grpc.md#uint64) |       |             |

#### Budget

| Field        | Type                       | Label | Description |
| ------------ | -------------------------- | ----- | ----------- |
| `total`      | [`uint64`](grpc.md#uint64) |       |             |
| `remaining`  | [`int64`](grpc.md#int64)   |       |             |
| `start_date` | [`int64`](grpc.md#int64)   |       |             |
| `end_date`   | [`int64`](grpc.md#int64)   |       |             |

#### ChangeWalletPasswordRequest

| Field | Type                       | Label | Description |
| ----- | -------------------------- | ----- | ----------- |
| `old` | [`string`](grpc.md#string) |       |             |
| `new` | [`string`](grpc.md#string) |       |             |

#### ChannelCreationInfo

Channel creations are an optional extension to a submarine swap in the data types of boltz-client.

| Field                      | Type                       | Label | Description                                          |
| -------------------------- | -------------------------- | ----- | ---------------------------------------------------- |
| `swap_id`                  | [`string`](grpc.md#string) |       | ID of the swap to which this channel channel belongs |
| `status`                   | [`string`](grpc.md#string) |       |                                                      |
| `inbound_liquidity`        | [`uint32`](grpc.md#uint32) |       |                                                      |
| `private`                  | [`bool`](grpc.md#bool)     |       |                                                      |
| `funding_transaction_id`   | [`string`](grpc.md#string) |       |                                                      |
| `funding_transaction_vout` | [`uint32`](grpc.md#uint32) |       |                                                      |

#### ChannelId

| Field | Type                       | Label | Description |
| ----- | -------------------------- | ----- | ----------- |
| `cln` | [`string`](grpc.md#string) |       |             |
| `lnd` | [`uint64`](grpc.md#uint64) |       |             |

#### CombinedChannelSwapInfo

| Field              | Type                                                          | Label | Description |
| ------------------ | ------------------------------------------------------------- | ----- | ----------- |
| `swap`             | [`SwapInfo`](grpc.md#boltzrpc.SwapInfo)                       |       |             |
| `channel_creation` | [`ChannelCreationInfo`](grpc.md#boltzrpc.ChannelCreationInfo) |       |             |

#### CreateChannelRequest

| Field               | Type                       | Label | Description                                                                            |
| ------------------- | -------------------------- | ----- | -------------------------------------------------------------------------------------- |
| `amount`            | [`int64`](grpc.md#int64)   |       |                                                                                        |
| `inbound_liquidity` | [`uint32`](grpc.md#uint32) |       | Percentage of inbound liquidity the channel that is opened should have. 25 by default. |
| `private`           | [`bool`](grpc.md#bool)     |       |                                                                                        |

#### CreateReverseSwapRequest

| Field              | Type                       | Label    | Description                                                            |
| ------------------ | -------------------------- | -------- | ---------------------------------------------------------------------- |
| `amount`           | [`int64`](grpc.md#int64)   |          |                                                                        |
| `address`          | [`string`](grpc.md#string) |          | If no value is set, the daemon will query a new P2WKH address from LND |
| `accept_zero_conf` | [`bool`](grpc.md#bool)     |          |                                                                        |
| `pair_id`          | [`string`](grpc.md#string) |          |                                                                        |
| `chan_ids`         | [`string`](grpc.md#string) | repeated |                                                                        |
| `wallet`           | [`string`](grpc.md#string) | optional |                                                                        |

#### CreateReverseSwapResponse

| Field                   | Type                       | Label | Description                                            |
| ----------------------- | -------------------------- | ----- | ------------------------------------------------------ |
| `id`                    | [`string`](grpc.md#string) |       |                                                        |
| `lockup_address`        | [`string`](grpc.md#string) |       |                                                        |
| `routing_fee_milli_sat` | [`uint32`](grpc.md#uint32) |       | **Deprecated.**                                        |
| `claim_transaction_id`  | [`string`](grpc.md#string) |       | **Deprecated.** Only populated when 0-conf is accepted |

#### CreateSwapRequest

| Field            | Type                       | Label    | Description |
| ---------------- | -------------------------- | -------- | ----------- |
| `amount`         | [`int64`](grpc.md#int64)   |          |             |
| `pair_id`        | [`string`](grpc.md#string) |          |             |
| `chan_ids`       | [`string`](grpc.md#string) | repeated |             |
| `auto_send`      | [`bool`](grpc.md#bool)     |          |             |
| `refund_address` | [`string`](grpc.md#string) |          |             |
| `wallet`         | [`string`](grpc.md#string) | optional |             |

#### CreateSwapResponse

| Field                  | Type                       | Label | Description |
| ---------------------- | -------------------------- | ----- | ----------- |
| `id`                   | [`string`](grpc.md#string) |       |             |
| `address`              | [`string`](grpc.md#string) |       |             |
| `expected_amount`      | [`int64`](grpc.md#int64)   |       |             |
| `bip21`                | [`string`](grpc.md#string) |       |             |
| `tx_id`                | [`string`](grpc.md#string) |       |             |
| `timeout_block_height` | [`uint32`](grpc.md#uint32) |       |             |

#### CreateWalletRequest

| Field      | Type                                        | Label    | Description |
| ---------- | ------------------------------------------- | -------- | ----------- |
| `info`     | [`WalletInfo`](grpc.md#boltzrpc.WalletInfo) |          |             |
| `password` | [`string`](grpc.md#string)                  | optional |             |

#### DepositRequest

| Field               | Type                       | Label | Description                                                                                                               |
| ------------------- | -------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------- |
| `inbound_liquidity` | [`uint32`](grpc.md#uint32) |       | Percentage of inbound liquidity the channel that is opened in case the invoice cannot be paid should have. 25 by default. |
| `pairId`            | [`string`](grpc.md#string) |       |                                                                                                                           |

#### DepositResponse

| Field                  | Type                       | Label | Description |
| ---------------------- | -------------------------- | ----- | ----------- |
| `id`                   | [`string`](grpc.md#string) |       |             |
| `address`              | [`string`](grpc.md#string) |       |             |
| `timeout_block_height` | [`uint32`](grpc.md#uint32) |       |             |

#### Fees

| Field        | Type                                      | Label | Description |
| ------------ | ----------------------------------------- | ----- | ----------- |
| `percentage` | [`float`](grpc.md#float)                  |       |             |
| `miner`      | [`MinerFees`](grpc.md#boltzrpc.MinerFees) |       |             |

#### GetFeeEstimationRequest

| Field       | Type                       | Label | Description |
| ----------- | -------------------------- | ----- | ----------- |
| `amount`    | [`uint64`](grpc.md#uint64) |       |             |
| `swap_type` | [`string`](grpc.md#string) |       |             |
| `pair_id`   | [`string`](grpc.md#string) |       |             |

#### GetFeeEstimationResponse

| Field    | Type                                | Label | Description |
| -------- | ----------------------------------- | ----- | ----------- |
| `fees`   | [`Fees`](grpc.md#boltzrpc.Fees)     |       |             |
| `limits` | [`Limits`](grpc.md#boltzrpc.Limits) |       |             |

#### GetInfoRequest

#### GetInfoResponse

| Field                   | Type                                                                                      | Label    | Description     |
| ----------------------- | ----------------------------------------------------------------------------------------- | -------- | --------------- |
| `version`               | [`string`](grpc.md#string)                                                                |          |                 |
| `node`                  | [`string`](grpc.md#string)                                                                |          |                 |
| `network`               | [`string`](grpc.md#string)                                                                |          |                 |
| `node_pubkey`           | [`string`](grpc.md#string)                                                                |          |                 |
| `auto_swap_status`      | [`string`](grpc.md#string)                                                                |          |                 |
| `block_heights`         | [`GetInfoResponse.BlockHeightsEntry`](grpc.md#boltzrpc.GetInfoResponse.BlockHeightsEntry) | repeated |                 |
| `symbol`                | [`string`](grpc.md#string)                                                                |          | **Deprecated.** |
| `lnd_pubkey`            | [`string`](grpc.md#string)                                                                |          | **Deprecated.** |
| `block_height`          | [`uint32`](grpc.md#uint32)                                                                |          | **Deprecated.** |
| `pending_swaps`         | [`string`](grpc.md#string)                                                                | repeated | **Deprecated.** |
| `pending_reverse_swaps` | [`string`](grpc.md#string)                                                                | repeated | **Deprecated.** |

#### GetInfoResponse.BlockHeightsEntry

| Field   | Type                       | Label | Description |
| ------- | -------------------------- | ----- | ----------- |
| `key`   | [`string`](grpc.md#string) |       |             |
| `value` | [`uint32`](grpc.md#uint32) |       |             |

#### GetServiceInfoRequest

| Field     | Type                       | Label | Description |
| --------- | -------------------------- | ----- | ----------- |
| `pair_id` | [`string`](grpc.md#string) |       |             |

#### GetServiceInfoResponse

| Field    | Type                                | Label | Description |
| -------- | ----------------------------------- | ----- | ----------- |
| `fees`   | [`Fees`](grpc.md#boltzrpc.Fees)     |       |             |
| `limits` | [`Limits`](grpc.md#boltzrpc.Limits) |       |             |

#### GetSubaccountsRequest

#### GetSubaccountsResponse

| Field         | Type                                        | Label    | Description |
| ------------- | ------------------------------------------- | -------- | ----------- |
| `current`     | [`uint64`](grpc.md#uint64)                  | optional |             |
| `subaccounts` | [`Subaccount`](grpc.md#boltzrpc.Subaccount) | repeated |             |

#### GetSwapInfoRequest

| Field | Type                       | Label | Description |
| ----- | -------------------------- | ----- | ----------- |
| `id`  | [`string`](grpc.md#string) |       |             |

#### GetSwapInfoResponse

| Field              | Type                                                          | Label | Description |
| ------------------ | ------------------------------------------------------------- | ----- | ----------- |
| `swap`             | [`SwapInfo`](grpc.md#boltzrpc.SwapInfo)                       |       |             |
| `channel_creation` | [`ChannelCreationInfo`](grpc.md#boltzrpc.ChannelCreationInfo) |       |             |
| `reverse_swap`     | [`ReverseSwapInfo`](grpc.md#boltzrpc.ReverseSwapInfo)         |       |             |

#### GetSwapRecommendationsRequest

#### GetSwapRecommendationsResponse

| Field   | Type                                                        | Label    | Description |
| ------- | ----------------------------------------------------------- | -------- | ----------- |
| `swaps` | [`SwapRecommendation`](grpc.md#boltzrpc.SwapRecommendation) | repeated |             |

#### GetWalletCredentialsRequest

| Field      | Type                       | Label    | Description |
| ---------- | -------------------------- | -------- | ----------- |
| `name`     | [`string`](grpc.md#string) |          |             |
| `password` | [`string`](grpc.md#string) | optional |             |

#### GetWalletRequest

| Field  | Type                       | Label | Description |
| ------ | -------------------------- | ----- | ----------- |
| `name` | [`string`](grpc.md#string) |       |             |

#### GetWalletsRequest

| Field              | Type                       | Label    | Description |
| ------------------ | -------------------------- | -------- | ----------- |
| `currency`         | [`string`](grpc.md#string) | optional |             |
| `include_readonly` | [`bool`](grpc.md#bool)     | optional |             |

#### ImportWalletRequest

| Field         | Type                                                      | Label    | Description |
| ------------- | --------------------------------------------------------- | -------- | ----------- |
| `credentials` | [`WalletCredentials`](grpc.md#boltzrpc.WalletCredentials) |          |             |
| `info`        | [`WalletInfo`](grpc.md#boltzrpc.WalletInfo)               |          |             |
| `password`    | [`string`](grpc.md#string)                                | optional |             |

#### ImportWalletResponse

#### LightningChannel

| Field        | Type                                      | Label | Description |
| ------------ | ----------------------------------------- | ----- | ----------- |
| `id`         | [`ChannelId`](grpc.md#boltzrpc.ChannelId) |       |             |
| `capacity`   | [`uint64`](grpc.md#uint64)                |       |             |
| `local_sat`  | [`uint64`](grpc.md#uint64)                |       |             |
| `remote_sat` | [`uint64`](grpc.md#uint64)                |       |             |
| `peer_id`    | [`string`](grpc.md#string)                |       |             |

#### Limits

| Field     | Type                     | Label | Description |
| --------- | ------------------------ | ----- | ----------- |
| `minimal` | [`int64`](grpc.md#int64) |       |             |
| `maximal` | [`int64`](grpc.md#int64) |       |             |

#### ListSwapsRequest

| Field     | Type                                      | Label    | Description |
| --------- | ----------------------------------------- | -------- | ----------- |
| `pair_id` | [`string`](grpc.md#string)                | optional |             |
| `is_auto` | [`bool`](grpc.md#bool)                    | optional |             |
| `state`   | [`SwapState`](grpc.md#boltzrpc.SwapState) | optional |             |

#### ListSwapsResponse

| Field               | Type                                                                  | Label    | Description |
| ------------------- | --------------------------------------------------------------------- | -------- | ----------- |
| `swaps`             | [`SwapInfo`](grpc.md#boltzrpc.SwapInfo)                               | repeated |             |
| `channel_creations` | [`CombinedChannelSwapInfo`](grpc.md#boltzrpc.CombinedChannelSwapInfo) | repeated |             |
| `reverse_swaps`     | [`ReverseSwapInfo`](grpc.md#boltzrpc.ReverseSwapInfo)                 | repeated |             |

#### MinerFees

| Field     | Type                       | Label | Description |
| --------- | -------------------------- | ----- | ----------- |
| `normal`  | [`uint32`](grpc.md#uint32) |       |             |
| `reverse` | [`uint32`](grpc.md#uint32) |       |             |

#### RemoveWalletRequest

| Field  | Type                       | Label | Description |
| ------ | -------------------------- | ----- | ----------- |
| `name` | [`string`](grpc.md#string) |       |             |

#### RemoveWalletResponse

#### ReverseSwapInfo

| Field                   | Type                                      | Label    | Description                                |
| ----------------------- | ----------------------------------------- | -------- | ------------------------------------------ |
| `id`                    | [`string`](grpc.md#string)                |          |                                            |
| `state`                 | [`SwapState`](grpc.md#boltzrpc.SwapState) |          |                                            |
| `error`                 | [`string`](grpc.md#string)                |          |                                            |
| `status`                | [`string`](grpc.md#string)                |          | Latest status message of the Boltz backend |
| `private_key`           | [`string`](grpc.md#string)                |          |                                            |
| `preimage`              | [`string`](grpc.md#string)                |          |                                            |
| `redeem_script`         | [`string`](grpc.md#string)                |          |                                            |
| `invoice`               | [`string`](grpc.md#string)                |          |                                            |
| `claim_address`         | [`string`](grpc.md#string)                |          |                                            |
| `onchain_amount`        | [`int64`](grpc.md#int64)                  |          |                                            |
| `timeout_block_height`  | [`uint32`](grpc.md#uint32)                |          |                                            |
| `lockup_transaction_id` | [`string`](grpc.md#string)                |          |                                            |
| `claim_transaction_id`  | [`string`](grpc.md#string)                |          |                                            |
| `pair_id`               | [`string`](grpc.md#string)                |          |                                            |
| `chan_ids`              | [`ChannelId`](grpc.md#boltzrpc.ChannelId) | repeated |                                            |
| `blinding_key`          | [`string`](grpc.md#string)                | optional |                                            |
| `created_at`            | [`int64`](grpc.md#int64)                  |          |                                            |
| `service_fee`           | [`uint64`](grpc.md#uint64)                | optional |                                            |
| `onchain_fee`           | [`uint64`](grpc.md#uint64)                | optional |                                            |
| `routing_fee_msat`      | [`uint64`](grpc.md#uint64)                | optional |                                            |

#### SetSubaccountRequest

| Field        | Type                       | Label    | Description                                                   |
| ------------ | -------------------------- | -------- | ------------------------------------------------------------- |
| `name`       | [`string`](grpc.md#string) |          |                                                               |
| `subaccount` | [`uint64`](grpc.md#uint64) | optional | The subaccount to use. If not set, a new one will be created. |

#### Subaccount

| Field     | Type                                  | Label | Description |
| --------- | ------------------------------------- | ----- | ----------- |
| `balance` | [`Balance`](grpc.md#boltzrpc.Balance) |       |             |
| `pointer` | [`uint64`](grpc.md#uint64)            |       |             |
| `type`    | [`string`](grpc.md#string)            |       |             |

#### SwapInfo

| Field                   | Type                                      | Label    | Description                                                                                                                                                                                                            |
| ----------------------- | ----------------------------------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`                    | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `pair_id`               | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `state`                 | [`SwapState`](grpc.md#boltzrpc.SwapState) |          |                                                                                                                                                                                                                        |
| `error`                 | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `status`                | [`string`](grpc.md#string)                |          | Latest status message of the Boltz backend                                                                                                                                                                             |
| `private_key`           | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `preimage`              | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `redeem_script`         | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `invoice`               | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `lockup_address`        | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `expected_amount`       | [`int64`](grpc.md#int64)                  |          |                                                                                                                                                                                                                        |
| `timeout_block_height`  | [`uint32`](grpc.md#uint32)                |          |                                                                                                                                                                                                                        |
| `lockup_transaction_id` | [`string`](grpc.md#string)                |          |                                                                                                                                                                                                                        |
| `refund_transaction_id` | [`string`](grpc.md#string)                |          | If the swap times out or fails for some other reason, the damon will automatically refund the coins sent to the `lockup_address` back to the configured wallet or the address specified in the `refund_address` field. |
| `refund_address`        | [`string`](grpc.md#string)                | optional |                                                                                                                                                                                                                        |
| `chan_ids`              | [`ChannelId`](grpc.md#boltzrpc.ChannelId) | repeated |                                                                                                                                                                                                                        |
| `blinding_key`          | [`string`](grpc.md#string)                | optional |                                                                                                                                                                                                                        |
| `created_at`            | [`int64`](grpc.md#int64)                  |          |                                                                                                                                                                                                                        |
| `service_fee`           | [`uint64`](grpc.md#uint64)                | optional |                                                                                                                                                                                                                        |
| `onchain_fee`           | [`uint64`](grpc.md#uint64)                | optional |                                                                                                                                                                                                                        |
| `auto_send`             | [`bool`](grpc.md#bool)                    |          |                                                                                                                                                                                                                        |

#### SwapRecommendation

| Field               | Type                                                    | Label    | Description |
| ------------------- | ------------------------------------------------------- | -------- | ----------- |
| `type`              | [`string`](grpc.md#string)                              |          |             |
| `amount`            | [`uint64`](grpc.md#uint64)                              |          |             |
| `channel`           | [`LightningChannel`](grpc.md#boltzrpc.LightningChannel) |          |             |
| `fee_estimate`      | [`uint64`](grpc.md#uint64)                              |          |             |
| `dismissed_reasons` | [`string`](grpc.md#string)                              | repeated |             |

#### SwapStats

| Field          | Type                       | Label | Description |
| -------------- | -------------------------- | ----- | ----------- |
| `total_fees`   | [`uint64`](grpc.md#uint64) |       |             |
| `total_amount` | [`uint64`](grpc.md#uint64) |       |             |
| `avg_fees`     | [`uint64`](grpc.md#uint64) |       |             |
| `avg_amount`   | [`uint64`](grpc.md#uint64) |       |             |
| `count`        | [`uint64`](grpc.md#uint64) |       |             |

#### UnlockRequest

| Field      | Type                       | Label | Description |
| ---------- | -------------------------- | ----- | ----------- |
| `password` | [`string`](grpc.md#string) |       |             |

#### VerifyWalletPasswordRequest

| Field      | Type                       | Label | Description |
| ---------- | -------------------------- | ----- | ----------- |
| `password` | [`string`](grpc.md#string) |       |             |

#### VerifyWalletPasswordResponse

| Field     | Type                   | Label | Description |
| --------- | ---------------------- | ----- | ----------- |
| `correct` | [`bool`](grpc.md#bool) |       |             |

#### Wallet

| Field      | Type                                  | Label | Description |
| ---------- | ------------------------------------- | ----- | ----------- |
| `name`     | [`string`](grpc.md#string)            |       |             |
| `currency` | [`string`](grpc.md#string)            |       |             |
| `readonly` | [`bool`](grpc.md#bool)                |       |             |
| `balance`  | [`Balance`](grpc.md#boltzrpc.Balance) |       |             |

#### WalletCredentials

| Field             | Type                       | Label    | Description                                |
| ----------------- | -------------------------- | -------- | ------------------------------------------ |
| `mnemonic`        | [`string`](grpc.md#string) | optional | only one of these is allowed to be present |
| `xpub`            | [`string`](grpc.md#string) | optional |                                            |
| `core_descriptor` | [`string`](grpc.md#string) | optional |                                            |
| `subaccount`      | [`uint64`](grpc.md#uint64) | optional | only used in combination with mnemonic     |

#### WalletInfo

| Field      | Type                       | Label | Description |
| ---------- | -------------------------- | ----- | ----------- |
| `name`     | [`string`](grpc.md#string) |       |             |
| `currency` | [`string`](grpc.md#string) |       |             |

#### Wallets

| Field     | Type                                | Label    | Description |
| --------- | ----------------------------------- | -------- | ----------- |
| `wallets` | [`Wallet`](grpc.md#boltzrpc.Wallet) | repeated |             |

### Enums

#### SwapState

| Name          | Number | Description                                                                      |
| ------------- | ------ | -------------------------------------------------------------------------------- |
| PENDING       | 0      |                                                                                  |
| SUCCESSFUL    | 1      |                                                                                  |
| ERROR         | 2      | Unknown client error. Check the error field of the message for more information  |
| SERVER\_ERROR | 3      | Unknown server error. Check the status field of the message for more information |
| REFUNDED      | 4      | Client refunded locked coins after the HTLC timed out                            |
| ABANDONED     | 5      | Client noticed that the HTLC timed out but didn't find any outputs to refund     |

This page was automatically generated based on the protobuf file `autoswaprpc/autoswap.proto`.

Paths for the REST proxy of the gRPC interface can be found [here](https://github.com/BoltzExchange/boltz-client/blob/master/boltzrpc/rest-annotations.yaml).

## autoswaprpc.AutoSwap

### Methods

#### GetSwapRecommendations

Returns a list of swaps which are currently recommended by the autoswapper. Also works when the autoswapper is not running.

| Request                                                                              | Response                                                                               |
| ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------- |
| [`GetSwapRecommendationsRequest`](grpc.md#autoswaprpc.GetSwapRecommendationsRequest) | [`GetSwapRecommendationsResponse`](grpc.md#autoswaprpc.GetSwapRecommendationsResponse) |

#### GetStatus

Returns the current budget of the autoswapper and some relevant stats.

| Request                                                    | Response                                                     |
| ---------------------------------------------------------- | ------------------------------------------------------------ |
| [`GetStatusRequest`](grpc.md#autoswaprpc.GetStatusRequest) | [`GetStatusResponse`](grpc.md#autoswaprpc.GetStatusResponse) |

#### ResetConfig

Resets the configuration to default values.

| Request                                                   | Response                               |
| --------------------------------------------------------- | -------------------------------------- |
| [`.google.protobuf.Empty`](grpc.md#google.protobuf.Empty) | [`Config`](grpc.md#autoswaprpc.Config) |

#### SetConfig

Allows setting multiple json-encoded config values at once. The autoswapper will reload the configuration after this call.

| Request                                | Response                               |
| -------------------------------------- | -------------------------------------- |
| [`Config`](grpc.md#autoswaprpc.Config) | [`Config`](grpc.md#autoswaprpc.Config) |

#### SetConfigValue

Allows setting a specific value in the configuration. The autoswapper will reload the configuration after this call.

| Request                                                              | Response                               |
| -------------------------------------------------------------------- | -------------------------------------- |
| [`SetConfigValueRequest`](grpc.md#autoswaprpc.SetConfigValueRequest) | [`Config`](grpc.md#autoswaprpc.Config) |

#### GetConfig

Returns the currently used configurationencoded as json. If a key is specfied, only the value of that key will be returned.

| Request                                                    | Response                               |
| ---------------------------------------------------------- | -------------------------------------- |
| [`GetConfigRequest`](grpc.md#autoswaprpc.GetConfigRequest) | [`Config`](grpc.md#autoswaprpc.Config) |

#### ReloadConfig

Reloads the configuration from disk.

| Request                                                   | Response                               |
| --------------------------------------------------------- | -------------------------------------- |
| [`.google.protobuf.Empty`](grpc.md#google.protobuf.Empty) | [`Config`](grpc.md#autoswaprpc.Config) |

### Messages

#### Budget

| Field        | Type                       | Label | Description |
| ------------ | -------------------------- | ----- | ----------- |
| `total`      | [`uint64`](grpc.md#uint64) |       |             |
| `remaining`  | [`int64`](grpc.md#int64)   |       |             |
| `start_date` | [`int64`](grpc.md#int64)   |       |             |
| `end_date`   | [`int64`](grpc.md#int64)   |       |             |

#### Config

| Field  | Type                       | Label | Description |
| ------ | -------------------------- | ----- | ----------- |
| `json` | [`string`](grpc.md#string) |       |             |

#### GetConfigRequest

| Field | Type                       | Label    | Description |
| ----- | -------------------------- | -------- | ----------- |
| `key` | [`string`](grpc.md#string) | optional |             |

#### GetStatusRequest

#### GetStatusResponse

| Field      | Type                                               | Label    | Description |
| ---------- | -------------------------------------------------- | -------- | ----------- |
| `running`  | [`bool`](grpc.md#bool)                             |          |             |
| `strategy` | [`string`](grpc.md#string)                         |          |             |
| `error`    | [`string`](grpc.md#string)                         |          |             |
| `stats`    | [`boltzrpc.SwapStats`](grpc.md#boltzrpc.SwapStats) | optional |             |
| `budget`   | [`Budget`](grpc.md#autoswaprpc.Budget)             | optional |             |

#### GetSwapRecommendationsRequest

| Field          | Type                   | Label    | Description                                 |
| -------------- | ---------------------- | -------- | ------------------------------------------- |
| `no_dismissed` | [`bool`](grpc.md#bool) | optional | Do not return any dismissed recommendations |

#### GetSwapRecommendationsResponse

| Field   | Type                                                           | Label    | Description |
| ------- | -------------------------------------------------------------- | -------- | ----------- |
| `swaps` | [`SwapRecommendation`](grpc.md#autoswaprpc.SwapRecommendation) | repeated |             |

#### SetConfigValueRequest

| Field   | Type                       | Label | Description |
| ------- | -------------------------- | ----- | ----------- |
| `key`   | [`string`](grpc.md#string) |       |             |
| `value` | [`string`](grpc.md#string) |       |             |

#### SwapRecommendation

| Field               | Type                                                             | Label    | Description |
| ------------------- | ---------------------------------------------------------------- | -------- | ----------- |
| `type`              | [`string`](grpc.md#string)                                       |          |             |
| `amount`            | [`uint64`](grpc.md#uint64)                                       |          |             |
| `channel`           | [`boltzrpc.LightningChannel`](grpc.md#boltzrpc.LightningChannel) |          |             |
| `fee_estimate`      | [`uint64`](grpc.md#uint64)                                       |          |             |
| `dismissed_reasons` | [`string`](grpc.md#string)                                       | repeated |             |

### Enums

## Scalar Value Types

| .proto Type | Notes                                                                                                                                           | C++      | Java         | Python        | Go        | C#           | PHP              | Ruby                             |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | -------- | ------------ | ------------- | --------- | ------------ | ---------------- | -------------------------------- |
| `double`    |                                                                                                                                                 | `double` | `double`     | `float`       | `float64` | `double`     | `float`          | `Float`                          |
| `float`     |                                                                                                                                                 | `float`  | `float`      | `float`       | `float32` | `float`      | `float`          | `Float`                          |
| `int32`     | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | `int32`  | `int`        | `int`         | `int32`   | `int`        | `integer`        | `Bignum or Fixnum (as required)` |
| `int64`     | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | `int64`  | `long`       | `int/long`    | `int64`   | `long`       | `integer/string` | `Bignum`                         |
| `uint32`    | Uses variable-length encoding.                                                                                                                  | `uint32` | `int`        | `int/long`    | `uint32`  | `uint`       | `integer`        | `Bignum or Fixnum (as required)` |
| `uint64`    | Uses variable-length encoding.                                                                                                                  | `uint64` | `long`       | `int/long`    | `uint64`  | `ulong`      | `integer/string` | `Bignum or Fixnum (as required)` |
| `sint32`    | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s.                            | `int32`  | `int`        | `int`         | `int32`   | `int`        | `integer`        | `Bignum or Fixnum (as required)` |
| `sint64`    | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s.                            | `int64`  | `long`       | `int/long`    | `int64`   | `long`       | `integer/string` | `Bignum`                         |
| `fixed32`   | Always four bytes. More efficient than uint32 if values are often greater than 2^28.                                                            | `uint32` | `int`        | `int`         | `uint32`  | `uint`       | `integer`        | `Bignum or Fixnum (as required)` |
| `fixed64`   | Always eight bytes. More efficient than uint64 if values are often greater than 2^56.                                                           | `uint64` | `long`       | `int/long`    | `uint64`  | `ulong`      | `integer/string` | `Bignum`                         |
| `sfixed32`  | Always four bytes.                                                                                                                              | `int32`  | `int`        | `int`         | `int32`   | `int`        | `integer`        | `Bignum or Fixnum (as required)` |
| `sfixed64`  | Always eight bytes.                                                                                                                             | `int64`  | `long`       | `int/long`    | `int64`   | `long`       | `integer/string` | `Bignum`                         |
| `bool`      |                                                                                                                                                 | `bool`   | `boolean`    | `boolean`     | `bool`    | `bool`       | `boolean`        | `TrueClass/FalseClass`           |
| `string`    | A string must always contain UTF-8 encoded or 7-bit ASCII text.                                                                                 | `string` | `String`     | `str/unicode` | `string`  | `string`     | `string`         | `String (UTF-8)`                 |
| `bytes`     | May contain any arbitrary sequence of bytes.                                                                                                    | `string` | `ByteString` | `str`         | `[]byte`  | `ByteString` | `string`         | `String (ASCII-8BIT)`            |
