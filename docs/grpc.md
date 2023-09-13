# gRPC Documentation




This page was automatically generated based on the protobuf file `boltzrpc.proto`.

Paths for the REST proxy of the gRPC interface can be found [here](https://github.com/BoltzExchange/boltz-lnd/blob/master/boltzrpc/rest-annotations.yaml).

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

### Messages

#### ChannelCreationInfo

Channel creations are an optional extension to a submarine swap in the data types of boltz-lnd.

| Field                      | Type                       | Label | Description                                          |
| -------------------------- | -------------------------- | ----- | ---------------------------------------------------- |
| `swap_id`                  | [`string`](grpc.md#string) |       | ID of the swap to which this channel channel belongs |
| `status`                   | [`string`](grpc.md#string) |       |                                                      |
| `inbound_liquidity`        | [`uint32`](grpc.md#uint32) |       |                                                      |
| `private`                  | [`bool`](grpc.md#bool)     |       |                                                      |
| `funding_transaction_id`   | [`string`](grpc.md#string) |       |                                                      |
| `funding_transaction_vout` | [`uint32`](grpc.md#uint32) |       |                                                      |

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

| Field              | Type                       | Label | Description                                                            |
| ------------------ | -------------------------- | ----- | ---------------------------------------------------------------------- |
| `amount`           | [`int64`](grpc.md#int64)   |       |                                                                        |
| `address`          | [`string`](grpc.md#string) |       | If no value is set, the daemon will query a new P2WKH address from LND |
| `accept_zero_conf` | [`bool`](grpc.md#bool)     |       |                                                                        |

#### CreateReverseSwapResponse

| Field                   | Type                       | Label | Description                            |
| ----------------------- | -------------------------- | ----- | -------------------------------------- |
| `id`                    | [`string`](grpc.md#string) |       |                                        |
| `lockup_address`        | [`string`](grpc.md#string) |       |                                        |
| `routing_fee_milli_sat` | [`uint32`](grpc.md#uint32) |       |                                        |
| `claim_transaction_id`  | [`string`](grpc.md#string) |       | Only populated when 0-conf is accepted |

#### CreateSwapRequest

| Field    | Type                     | Label | Description |
| -------- | ------------------------ | ----- | ----------- |
| `amount` | [`int64`](grpc.md#int64) |       |             |

#### CreateSwapResponse

| Field             | Type                       | Label | Description |
| ----------------- | -------------------------- | ----- | ----------- |
| `id`              | [`string`](grpc.md#string) |       |             |
| `address`         | [`string`](grpc.md#string) |       |             |
| `expected_amount` | [`int64`](grpc.md#int64)   |       |             |
| `bip21`           | [`string`](grpc.md#string) |       |             |

#### DepositRequest

| Field               | Type                       | Label | Description                                                                                                               |
| ------------------- | -------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------- |
| `inbound_liquidity` | [`uint32`](grpc.md#uint32) |       | Percentage of inbound liquidity the channel that is opened in case the invoice cannot be paid should have. 25 by default. |

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

#### GetInfoRequest

#### GetInfoResponse

| Field                   | Type                       | Label    | Description |
| ----------------------- | -------------------------- | -------- | ----------- |
| `symbol`                | [`string`](grpc.md#string) |          |             |
| `network`               | [`string`](grpc.md#string) |          |             |
| `lnd_pubkey`            | [`string`](grpc.md#string) |          |             |
| `block_height`          | [`uint32`](grpc.md#uint32) |          |             |
| `pending_swaps`         | [`string`](grpc.md#string) | repeated |             |
| `pending_reverse_swaps` | [`string`](grpc.md#string) | repeated |             |

#### GetServiceInfoRequest

#### GetServiceInfoResponse

| Field    | Type                                | Label | Description |
| -------- | ----------------------------------- | ----- | ----------- |
| `fees`   | [`Fees`](grpc.md#boltzrpc.Fees)     |       |             |
| `limits` | [`Limits`](grpc.md#boltzrpc.Limits) |       |             |

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

#### Limits

| Field     | Type                     | Label | Description |
| --------- | ------------------------ | ----- | ----------- |
| `minimal` | [`int64`](grpc.md#int64) |       |             |
| `maximal` | [`int64`](grpc.md#int64) |       |             |

#### ListSwapsRequest

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

#### ReverseSwapInfo

| Field                   | Type                                      | Label | Description                                |
| ----------------------- | ----------------------------------------- | ----- | ------------------------------------------ |
| `id`                    | [`string`](grpc.md#string)                |       |                                            |
| `state`                 | [`SwapState`](grpc.md#boltzrpc.SwapState) |       |                                            |
| `error`                 | [`string`](grpc.md#string)                |       |                                            |
| `status`                | [`string`](grpc.md#string)                |       | Latest status message of the Boltz backend |
| `private_key`           | [`string`](grpc.md#string)                |       |                                            |
| `preimage`              | [`string`](grpc.md#string)                |       |                                            |
| `redeem_script`         | [`string`](grpc.md#string)                |       |                                            |
| `invoice`               | [`string`](grpc.md#string)                |       |                                            |
| `claim_address`         | [`string`](grpc.md#string)                |       |                                            |
| `onchain_amount`        | [`int64`](grpc.md#int64)                  |       |                                            |
| `timeout_block_height`  | [`uint32`](grpc.md#uint32)                |       |                                            |
| `lockup_transaction_id` | [`string`](grpc.md#string)                |       |                                            |
| `claim_transaction_id`  | [`string`](grpc.md#string)                |       |                                            |

#### SwapInfo

| Field                   | Type                                      | Label | Description                                                                                                                                                                                                 |
| ----------------------- | ----------------------------------------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`                    | [`string`](grpc.md#string)                |       |                                                                                                                                                                                                             |
| `state`                 | [`SwapState`](grpc.md#boltzrpc.SwapState) |       |                                                                                                                                                                                                             |
| `error`                 | [`string`](grpc.md#string)                |       |                                                                                                                                                                                                             |
| `status`                | [`string`](grpc.md#string)                |       | Latest status message of the Boltz backend                                                                                                                                                                  |
| `private_key`           | [`string`](grpc.md#string)                |       |                                                                                                                                                                                                             |
| `preimage`              | [`string`](grpc.md#string)                |       |                                                                                                                                                                                                             |
| `redeem_script`         | [`string`](grpc.md#string)                |       |                                                                                                                                                                                                             |
| `invoice`               | [`string`](grpc.md#string)                |       |                                                                                                                                                                                                             |
| `lockup_address`        | [`string`](grpc.md#string)                |       |                                                                                                                                                                                                             |
| `expected_amount`       | [`int64`](grpc.md#int64)                  |       |                                                                                                                                                                                                             |
| `timeout_block_height`  | [`uint32`](grpc.md#uint32)                |       |                                                                                                                                                                                                             |
| `lockup_transaction_id` | [`string`](grpc.md#string)                |       |                                                                                                                                                                                                             |
| `refund_transaction_id` | [`string`](grpc.md#string)                |       | If the swap times out or fails for some other reason, the damon will automatically refund the coins sent to the `lockup_address` back to the LND wallet and save the refund transaction id to the database. |

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
