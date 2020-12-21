# gRPC Documentation




This page was automatically generated based on the protobuf file `boltzrpc.proto`.


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




### Messages

#### <div id="boltzrpc.ChannelCreationInfo">ChannelCreationInfo</div>
Channel creations are an optional extension to a submarine swap in the data types of boltz-lnd.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap_id` | [`string`](#string) |  | ID of the swap to which this channel channel belongs |
| `status` | [`string`](#string) |  |  |
| `inbound_liquidity` | [`uint32`](#uint32) |  |  |
| `private` | [`bool`](#bool) |  |  |
| `funding_transaction_id` | [`string`](#string) |  |  |
| `funding_transaction_vout` | [`uint32`](#uint32) |  |  |





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





#### <div id="boltzrpc.CreateReverseSwapResponse">CreateReverseSwapResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `lockup_address` | [`string`](#string) |  |  |
| `routing_fee_milli_sat` | [`uint32`](#uint32) |  |  |
| `claim_transaction_id` | [`string`](#string) |  | Only populated when 0-conf is accepted |





#### <div id="boltzrpc.CreateSwapRequest">CreateSwapRequest</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |





#### <div id="boltzrpc.CreateSwapResponse">CreateSwapResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `address` | [`string`](#string) |  |  |
| `expected_amount` | [`int64`](#int64) |  |  |
| `bip21` | [`string`](#string) |  |  |





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





#### <div id="boltzrpc.GetInfoRequest">GetInfoRequest</div>






#### <div id="boltzrpc.GetInfoResponse">GetInfoResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `symbol` | [`string`](#string) |  |  |
| `network` | [`string`](#string) |  |  |
| `lnd_pubkey` | [`string`](#string) |  |  |
| `block_height` | [`uint32`](#uint32) |  |  |
| `pending_swaps` | [`string`](#string) | repeated |  |
| `pending_reverse_swaps` | [`string`](#string) | repeated |  |





#### <div id="boltzrpc.GetServiceInfoRequest">GetServiceInfoRequest</div>






#### <div id="boltzrpc.GetServiceInfoResponse">GetServiceInfoResponse</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `fees` | [`Fees`](#boltzrpc.Fees) |  |  |
| `limits` | [`Limits`](#boltzrpc.Limits) |  |  |





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





#### <div id="boltzrpc.Limits">Limits</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `minimal` | [`int64`](#int64) |  |  |
| `maximal` | [`int64`](#int64) |  |  |





#### <div id="boltzrpc.ListSwapsRequest">ListSwapsRequest</div>






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





#### <div id="boltzrpc.ReverseSwapInfo">ReverseSwapInfo</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `status` | [`string`](#string) |  |  |
| `private_key` | [`string`](#string) |  |  |
| `preimage` | [`string`](#string) |  |  |
| `redeem_script` | [`string`](#string) |  |  |
| `invoice` | [`string`](#string) |  |  |
| `claim_address` | [`string`](#string) |  |  |
| `onchain_amount` | [`int64`](#int64) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `lockup_transaction_id` | [`string`](#string) |  |  |
| `claim_transaction_id` | [`string`](#string) |  |  |





#### <div id="boltzrpc.SwapInfo">SwapInfo</div>



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `status` | [`string`](#string) |  |  |
| `private_key` | [`string`](#string) |  |  |
| `preimage` | [`string`](#string) |  |  |
| `redeem_script` | [`string`](#string) |  |  |
| `invoice` | [`string`](#string) |  |  |
| `lockup_address` | [`string`](#string) |  |  |
| `expected_amount` | [`int64`](#int64) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `lockup_transaction_id` | [`string`](#string) |  |  |
| `refund_transaction_id` | [`string`](#string) |  | If the swap times out or fails for some other reason, the damon will automatically refund the coins sent to the `lockup_address` back to the LND wallet and save the refund transaction id to the database. |






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

