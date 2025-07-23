# gRPC Documentation

This page was automatically generated.

## Authorization

The gRPC API supports two authorization methods.

### Macaroons

Macaroons are cryptographic bearer tokens that offer fine-grained access control and serve as the default authorization method for the gRPC. The daemon automatically generates two macaroons at startup.

- Admin macaroon (`admin.macaroon`) - grants full access to all RPCs
- Readonly macaroon (`readonly.macaroon`) - grants read-only access to all RPCs

The macaroons are stored in the `macaroons` folder in the data directory by default. Their locations can be overridden using the configuration options:

- `rpc.adminmacaroonpath`
- `rpc.readonlymacaroonpath`

When using macaroon authentication, include the macaroon in your request metadata:

- For gRPC: Use the `macaroon` key in the request metadata
- For REST proxy: Use the `Grpc-Metadata-Macaroon` header

### Password

The client supports simple password authentication as an alternative to macaroons.

To enable password authentication:

1. Set a password using the `rpc.password` flag at startup
2. Or configure it in the [config](configuration.md)

Note: When password authentication is enabled, macaroon authentication is automatically disabled and vice-versa.

To use password authentication:

- For gRPC: Include the password in the `authorization` key of the request metadata
- For REST proxy: Use the `Authorization` header

Note: It is recommended to use macaroon authentication when possible as it provides more granular access control.

Paths for the REST proxy of the gRPC interface can be found [here](https://github.com/BoltzExchange/boltz-client/blob/master/pkg/boltzrpc/rest-annotations.yaml).





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

#### GetStats

Returns stats of all swaps, reverse swaps, and chain swaps in the database.

| Request | Response |
| ------- | -------- |
| [`GetStatsRequest`](#getstatsrequest) | [`GetStatsResponse`](#getstatsresponse) |

#### RefundSwap

Refund a failed swap manually. This is only required when no refund address has been set and the swap does not have an associated wallet.

| Request | Response |
| ------- | -------- |
| [`RefundSwapRequest`](#refundswaprequest) | [`GetSwapInfoResponse`](#getswapinforesponse) |

#### ClaimSwaps

Claim swaps manually. This is only required when no claim address has been set and the swap does not have an associated wallet.

| Request | Response |
| ------- | -------- |
| [`ClaimSwapsRequest`](#claimswapsrequest) | [`ClaimSwapsResponse`](#claimswapsresponse) |

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

#### GetWalletSendFee

Calculates the fee for an equivalent `WalletSend` request. If `address` is left empty, a dummy swap address will be used, allowing for a fee estimation of a swap lockup transaction.

| Request | Response |
| ------- | -------- |
| [`WalletSendRequest`](#walletsendrequest) | [`WalletSendFee`](#walletsendfee) |

#### ListWalletTransactions

Returns recent transactions from a wallet.

| Request | Response |
| ------- | -------- |
| [`ListWalletTransactionsRequest`](#listwallettransactionsrequest) | [`ListWalletTransactionsResponse`](#listwallettransactionsresponse) |

#### BumpTransaction

Increase the fee of a transaction using RBF. The transaction has to belong to one of the clients wallets.

| Request | Response |
| ------- | -------- |
| [`BumpTransactionRequest`](#bumptransactionrequest) | [`BumpTransactionResponse`](#bumptransactionresponse) |

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

#### WalletSend

Send coins from a wallet. Only the confirmed balance can be spent.

| Request | Response |
| ------- | -------- |
| [`WalletSendRequest`](#walletsendrequest) | [`WalletSendResponse`](#walletsendresponse) |

#### WalletReceive

Get a new address of the wallet.

| Request | Response |
| ------- | -------- |
| [`WalletReceiveRequest`](#walletreceiverequest) | [`WalletReceiveResponse`](#walletreceiveresponse) |

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

#### CreateTenant

Creates a new tenant which can be used to bake restricted macaroons.

| Request | Response |
| ------- | -------- |
| [`CreateTenantRequest`](#createtenantrequest) | [`Tenant`](#tenant) |

#### ListTenants

Returns all tenants.

| Request | Response |
| ------- | -------- |
| [`ListTenantsRequest`](#listtenantsrequest) | [`ListTenantsResponse`](#listtenantsresponse) |

#### GetTenant

Get a specifiy tenant.

| Request | Response |
| ------- | -------- |
| [`GetTenantRequest`](#gettenantrequest) | [`Tenant`](#tenant) |

#### BakeMacaroon

Bakes a new macaroon with the specified permissions. The macaroon can also be restricted to a specific tenant. In this case, - any swap or wallet created with the returned macaroon will belong to this tenant and can not be accessed by other tenants. - the lightning node connected to the daemon can not be used to pay or create invoices for swaps.

| Request | Response |
| ------- | -------- |
| [`BakeMacaroonRequest`](#bakemacaroonrequest) | [`BakeMacaroonResponse`](#bakemacaroonresponse) |

#### GetSwapMnemonic

Returns mnemonic used for the key derivation of swaps, which can be used to restore swap information in the case of data loss.

| Request | Response |
| ------- | -------- |
| [`GetSwapMnemonicRequest`](#getswapmnemonicrequest) | [`GetSwapMnemonicResponse`](#getswapmnemonicresponse) |

#### SetSwapMnemonic

Sets the mnemonic used for key derivation of swaps. An existing mnemonic can be used, or a new one can be generated.

| Request | Response |
| ------- | -------- |
| [`SetSwapMnemonicRequest`](#setswapmnemonicrequest) | [`SetSwapMnemonicResponse`](#setswapmnemonicresponse) |




### Messages

#### AnySwapInfo




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `type` | [`SwapType`](#swaptype) |  |  |
| `pair` | [`Pair`](#pair) |  |  |
| `state` | [`SwapState`](#swapstate) |  |  |
| `error` | [`string`](#string) | optional |  |
| `status` | [`string`](#string) |  |  |
| `from_amount` | [`uint64`](#uint64) |  | The expected amount to be sent to the lockup address for submarine and chain swaps and the invoice amount for reverse swaps. |
| `to_amount` | [`uint64`](#uint64) |  | `from_amount` minus the service and network fee. |
| `created_at` | [`int64`](#int64) |  |  |
| `service_fee` | [`int64`](#int64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional | inclues the routing fee for reverse swaps |
| `is_auto` | [`bool`](#bool) |  |  |
| `tenant_id` | [`uint64`](#uint64) |  |  |





#### BakeMacaroonRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `tenant_id` | [`uint64`](#uint64) | optional |  |
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





#### BumpTransactionRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `tx_id` | [`string`](#string) |  | Id of the transaction to bump. The transaction has to belong to one of the clients wallets |
| `swap_id` | [`string`](#string) |  | Depending on the state of the swap, the lockup, refund or claim transaction will be bumped |
| `sat_per_vbyte` | [`double`](#double) | optional | Fee rate for the new transaction. if not specified, the daemon will query the fee rate from the configured provider and bump the fee by at least 1 sat/vbyte. |





#### BumpTransactionResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `tx_id` | [`string`](#string) |  |  |





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
| `service_fee` | [`int64`](#int64) | optional |  |
| `service_fee_percent` | [`double`](#double) |  |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `tenant_id` | [`uint64`](#uint64) |  |  |
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





#### ClaimSwapsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap_ids` | [`string`](#string) | repeated |  |
| `address` | [`string`](#string) |  |  |
| `wallet_id` | [`uint64`](#uint64) |  |  |





#### ClaimSwapsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `transaction_id` | [`string`](#string) |  |  |





#### CombinedChannelSwapInfo




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`SwapInfo`](#swapinfo) |  |  |
| `channel_creation` | [`ChannelCreationInfo`](#channelcreationinfo) |  |  |





#### CreateChainSwapRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) | optional | Amount of satoshis to swap. It is the amount expected to be sent to the lockup address. If left empty, any amount within the limits will be accepted. |
| `pair` | [`Pair`](#pair) |  |  |
| `to_address` | [`string`](#string) | optional | Address where funds will be swept to if the swap succeeds |
| `refund_address` | [`string`](#string) | optional | Address where the coins should be refunded to if the swap fails. |
| `from_wallet_id` | [`uint64`](#uint64) | optional | Wallet from which the swap should be paid from. Ignored if `external_pay` is set to true. If the swap fails, funds will be refunded to this wallet as well. |
| `to_wallet_id` | [`uint64`](#uint64) | optional | Wallet where the the funds will go if the swap succeeds. |
| `accept_zero_conf` | [`bool`](#bool) | optional | Whether the daemon should broadcast the claim transaction immediately after the lockup transaction is in the mempool. Should only be used for smaller amounts as it involves trust in Boltz. |
| `external_pay` | [`bool`](#bool) | optional | If set, the daemon will not pay the swap from an internal wallet. |
| `lockup_zero_conf` | [`bool`](#bool) | optional | **Deprecated.**  |
| `sat_per_vbyte` | [`double`](#double) | optional | Fee rate to use when sending from internal wallet |
| `accepted_pair` | [`PairInfo`](#pairinfo) | optional | Rates to accept for the swap. Queries latest from boltz otherwise The recommended way to use this is to pass a user approved value from a previous `GetPairInfo` call |





#### CreateChannelRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`int64`](#int64) |  |  |
| `inbound_liquidity` | [`uint32`](#uint32) |  | Percentage of inbound liquidity the channel that is opened should have. 25 by default. |
| `private` | [`bool`](#bool) |  |  |





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
| `description` | [`string`](#string) | optional | Description of the invoice which will be created for the swap |
| `description_hash` | [`bytes`](#bytes) | optional | Description hash of the invoice which will be created for the swap. Takes precedence over `description` |
| `invoice_expiry` | [`uint64`](#uint64) | optional | Expiry of the reverse swap invoice in seconds |
| `accepted_pair` | [`PairInfo`](#pairinfo) | optional | Rates to accept for the swap. Queries latest from boltz otherwise The recommended way to use this is to pass a user approved value from a previous `GetPairInfo` call |
| `routing_fee_limit_ppm` | [`uint64`](#uint64) | optional | The routing fee limit for paying the lightning invoice in ppm (parts per million) |





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
| `amount` | [`uint64`](#uint64) |  | amount of sats to be received on lightning. related: `invoice` field |
| `pair` | [`Pair`](#pair) |  |  |
| `send_from_internal` | [`bool`](#bool) |  | the daemon will pay the swap using the onchain wallet specified in the `wallet` field or the first internal wallet with the correct currency otherwise. |
| `refund_address` | [`string`](#string) | optional | address where the coins should go if the swap fails. Refunds will go to any of the daemons wallets otherwise. |
| `wallet_id` | [`uint64`](#uint64) | optional | wallet to pay swap from. only used if `send_from_internal` is set to true |
| `invoice` | [`string`](#string) | optional | bolt11 invoice, lnurl, or lnaddress to use for the swap. required in standalone mode. when connected to a lightning node, a new invoice for `amount` sats will be fetched the `amount` field has to be populated in case of a lnurl and lnaddress |
| `zero_conf` | [`bool`](#bool) | optional | **Deprecated.**  |
| `sat_per_vbyte` | [`double`](#double) | optional | Fee rate to use when sending from internal wallet |
| `accepted_pair` | [`PairInfo`](#pairinfo) | optional | Rates to accept for the swap. Queries latest from boltz otherwise The recommended way to use this is to pass a user approved value from a previous `GetPairInfo` call |
| `ignore_mrh` | [`bool`](#bool) | optional | Ignore any magic routing hints found in the specified `invoice`. |





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





#### CreateTenantRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |





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





#### Fees




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`float`](#float) |  |  |
| `miner` | [`MinerFees`](#minerfees) |  |  |





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
| `tenant` | [`Tenant`](#tenant) | optional | the currently authenticated tenant |
| `claimable_swaps` | [`string`](#string) | repeated | swaps that need a manual interaction to claim |
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





#### GetStatsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `include` | [`IncludeSwaps`](#includeswaps) |  |  |





#### GetStatsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `stats` | [`SwapStats`](#swapstats) |  |  |





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
| `id` | [`string`](#string) |  | **Deprecated.**  |
| `swap_id` | [`string`](#string) |  |  |
| `payment_hash` | [`bytes`](#bytes) |  | Only implemented for submarine swaps |





#### GetSwapInfoResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`SwapInfo`](#swapinfo) |  |  |
| `channel_creation` | [`ChannelCreationInfo`](#channelcreationinfo) |  |  |
| `reverse_swap` | [`ReverseSwapInfo`](#reverseswapinfo) |  |  |
| `chain_swap` | [`ChainSwapInfo`](#chainswapinfo) |  |  |





#### GetSwapMnemonicRequest







#### GetSwapMnemonicResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `mnemonic` | [`string`](#string) |  |  |





#### GetTenantRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |





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
| `outbound_sat` | [`uint64`](#uint64) |  |  |
| `inbound_sat` | [`uint64`](#uint64) |  |  |
| `peer_id` | [`string`](#string) |  |  |





#### Limits




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `minimal` | [`uint64`](#uint64) |  |  |
| `maximal` | [`uint64`](#uint64) |  |  |
| `maximal_zero_conf_amount` | [`uint64`](#uint64) |  |  |





#### ListSwapsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `from` | [`Currency`](#currency) | optional |  |
| `to` | [`Currency`](#currency) | optional |  |
| `state` | [`SwapState`](#swapstate) | optional |  |
| `include` | [`IncludeSwaps`](#includeswaps) |  |  |
| `limit` | [`uint64`](#uint64) | optional |  |
| `offset` | [`uint64`](#uint64) | optional |  |
| `unify` | [`bool`](#bool) | optional | wether to return swaps in the shared `all_swaps` list or in the detailed lists. the `limit` and `offset` are only considered when `unify` is true. |





#### ListSwapsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swaps` | [`SwapInfo`](#swapinfo) | repeated |  |
| `channel_creations` | [`CombinedChannelSwapInfo`](#combinedchannelswapinfo) | repeated | **Deprecated.**  |
| `reverse_swaps` | [`ReverseSwapInfo`](#reverseswapinfo) | repeated |  |
| `chain_swaps` | [`ChainSwapInfo`](#chainswapinfo) | repeated |  |
| `all_swaps` | [`AnySwapInfo`](#anyswapinfo) | repeated | populated when `unify` is set to true in the request |





#### ListTenantsRequest







#### ListTenantsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `tenants` | [`Tenant`](#tenant) | repeated |  |





#### ListWalletTransactionsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`uint64`](#uint64) |  |  |
| `exclude_swap_related` | [`bool`](#bool) | optional |  |
| `limit` | [`uint64`](#uint64) | optional |  |
| `offset` | [`uint64`](#uint64) | optional |  |





#### ListWalletTransactionsResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `transactions` | [`WalletTransaction`](#wallettransaction) | repeated |  |





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
| `wallet_id` | [`uint64`](#uint64) |  |  |





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
| `onchain_amount` | [`uint64`](#uint64) |  |  |
| `invoice_amount` | [`uint64`](#uint64) |  |  |
| `timeout_block_height` | [`uint32`](#uint32) |  |  |
| `lockup_transaction_id` | [`string`](#string) |  |  |
| `claim_transaction_id` | [`string`](#string) |  |  |
| `pair` | [`Pair`](#pair) |  |  |
| `chan_ids` | [`ChannelId`](#channelid) | repeated |  |
| `blinding_key` | [`string`](#string) | optional |  |
| `created_at` | [`int64`](#int64) |  |  |
| `paid_at` | [`int64`](#int64) | optional | the time when the invoice was paid |
| `service_fee` | [`int64`](#int64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `routing_fee_msat` | [`uint64`](#uint64) | optional |  |
| `external_pay` | [`bool`](#bool) |  |  |
| `tenant_id` | [`uint64`](#uint64) |  |  |
| `is_auto` | [`bool`](#bool) |  |  |





#### SetSubaccountRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `wallet_id` | [`uint64`](#uint64) |  |  |
| `subaccount` | [`uint64`](#uint64) | optional | The subaccount to use. If not set, a new one will be created. |





#### SetSwapMnemonicRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `existing` | [`string`](#string) |  |  |
| `generate` | [`bool`](#bool) |  |  |





#### SetSwapMnemonicResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `mnemonic` | [`string`](#string) |  |  |





#### Subaccount




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `balance` | [`Balance`](#balance) |  |  |
| `pointer` | [`uint64`](#uint64) |  |  |
| `type` | [`string`](#string) |  |  |
| `descriptors` | [`string`](#string) | repeated |  |





#### SwapFees




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `percentage` | [`double`](#double) |  |  |
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
| `service_fee` | [`int64`](#int64) | optional |  |
| `onchain_fee` | [`uint64`](#uint64) | optional |  |
| `wallet_id` | [`uint64`](#uint64) | optional | internal wallet which was used to pay the swap |
| `tenant_id` | [`uint64`](#uint64) |  |  |
| `is_auto` | [`bool`](#bool) |  |  |





#### SwapStats




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `total_fees` | [`int64`](#int64) |  |  |
| `total_amount` | [`uint64`](#uint64) |  |  |
| `avg_fees` | [`int64`](#int64) |  |  |
| `avg_amount` | [`uint64`](#uint64) |  |  |
| `count` | [`uint64`](#uint64) |  |  |
| `success_count` | [`uint64`](#uint64) |  |  |





#### Tenant




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`uint64`](#uint64) |  |  |
| `name` | [`string`](#string) |  |  |





#### TransactionInfo




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap_id` | [`string`](#string) | optional | will be populated for LOCKUP, REFUND and CLAIM |
| `type` | [`TransactionType`](#transactiontype) |  |  |





#### TransactionOutput




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `address` | [`string`](#string) |  |  |
| `amount` | [`uint64`](#uint64) |  |  |
| `is_our_address` | [`bool`](#bool) |  | wether the address is controlled by the wallet |





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
| `tenant_id` | [`uint64`](#uint64) |  |  |





#### WalletCredentials




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `mnemonic` | [`string`](#string) | optional | only one of these is allowed to be present |
| `xpub` | [`string`](#string) | optional |  |
| `core_descriptor` | [`string`](#string) | optional |  |
| `subaccount` | [`uint64`](#uint64) | optional | **Deprecated.** only used in combination with mnemonic |





#### WalletParams




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `name` | [`string`](#string) |  |  |
| `currency` | [`Currency`](#currency) |  |  |
| `password` | [`string`](#string) | optional | the password to encrypt the wallet with. If there are existing encrypted wallets, the same password has to be used. |





#### WalletReceiveRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`uint64`](#uint64) |  |  |





#### WalletReceiveResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `address` | [`string`](#string) |  |  |





#### WalletSendFee




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  | amount of sats which would be sent |
| `fee` | [`uint64`](#uint64) |  |  |
| `fee_rate` | [`double`](#double) |  | the fee rate used for the estimation in sat/vbyte |





#### WalletSendRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`uint64`](#uint64) |  |  |
| `address` | [`string`](#string) |  |  |
| `amount` | [`uint64`](#uint64) |  | Amount of satoshis to be sent to 'address` |
| `sat_per_vbyte` | [`double`](#double) | optional | Fee rate to use for the transaction |
| `send_all` | [`bool`](#bool) | optional | Sends all available funds to the address. The `amount` field is ignored. |
| `is_swap_address` | [`bool`](#bool) | optional | whether `address` is the lockup of a swap. |





#### WalletSendResponse




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `tx_id` | [`string`](#string) |  |  |





#### WalletTransaction




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `id` | [`string`](#string) |  |  |
| `balance_change` | [`int64`](#int64) |  | balance change of the wallet in satoshis. its the sum of all output values minus the sum of all input values which are controlled by the wallet. positive values indicate incoming transactions, negative values outgoing transactions |
| `timestamp` | [`int64`](#int64) |  |  |
| `outputs` | [`TransactionOutput`](#transactionoutput) | repeated |  |
| `block_height` | [`uint32`](#uint32) |  |  |
| `infos` | [`TransactionInfo`](#transactioninfo) | repeated | additional informations about the tx (type, related swaps etc.) |





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



#### IncludeSwaps


| Name | Number | Description |
| ---- | ------ | ----------- |
| ALL | 0 |  |
| MANUAL | 1 |  |
| AUTO | 2 |  |



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



#### TransactionType


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| LOCKUP | 1 |  |
| REFUND | 2 |  |
| CLAIM | 3 |  |
| CONSOLIDATION | 4 |  |






## autoswaprpc.AutoSwap



### Methods
#### GetRecommendations

Returns a list of swaps which are currently recommended by autoswap. Also works when autoswap is not running.

| Request | Response |
| ------- | -------- |
| [`GetRecommendationsRequest`](#getrecommendationsrequest) | [`GetRecommendationsResponse`](#getrecommendationsresponse) |

#### ExecuteRecommendations

Executes recommendations previously returned by `GetRecommendations`. Intended to be used when autoswap is fully configured but not enabled to allow for manual approval.

| Request | Response |
| ------- | -------- |
| [`ExecuteRecommendationsRequest`](#executerecommendationsrequest) | [`ExecuteRecommendationsResponse`](#executerecommendationsresponse) |

#### GetStatus

Returns the current budget of autoswap and some relevant stats.

| Request | Response |
| ------- | -------- |
| [`GetStatusRequest`](#getstatusrequest) | [`GetStatusResponse`](#getstatusresponse) |

#### UpdateLightningConfig

Updates the lightning configuration entirely or partially. Autoswap will reload the configuration after this call.

| Request | Response |
| ------- | -------- |
| [`UpdateLightningConfigRequest`](#updatelightningconfigrequest) | [`Config`](#config) |

#### UpdateChainConfig

Updates the chain configuration entirely or partially. Autoswap will reload the configuration after this call.

| Request | Response |
| ------- | -------- |
| [`UpdateChainConfigRequest`](#updatechainconfigrequest) | [`Config`](#config) |

#### GetConfig

Returns the currently used configuration.

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
| `remaining` | [`uint64`](#uint64) |  |  |
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
| `reserve_balance` | [`uint64`](#uint64) |  |  |
| `max_fee_percent` | [`float`](#float) |  |  |
| `budget` | [`uint64`](#uint64) |  |  |
| `budget_interval` | [`uint64`](#uint64) |  |  |
| `tenant` | [`string`](#string) | optional |  |





#### ChainRecommendation




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`ChainSwap`](#chainswap) | optional | Populated when a swap is recommended based on the configured `wallet_balance` of the configured `from_wallet` exceeds the currently configured `max_balance` |
| `wallet_balance` | [`boltzrpc.Balance`](#boltzrpc.balance) |  |  |
| `max_balance` | [`uint64`](#uint64) |  |  |





#### ChainSwap




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  |  |
| `fee_estimate` | [`uint64`](#uint64) |  |  |
| `dismissed_reasons` | [`string`](#string) | repeated | Reasons for which the swap is not being executed |





#### Config




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `chain` | [`ChainConfig`](#chainconfig) | repeated |  |
| `lightning` | [`LightningConfig`](#lightningconfig) | repeated |  |





#### ExecuteRecommendationsRequest




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `lightning` | [`LightningRecommendation`](#lightningrecommendation) | repeated |  |
| `chain` | [`ChainRecommendation`](#chainrecommendation) | repeated |  |
| `force` | [`bool`](#bool) | optional | Forcefully execute all recommendations, even ones that have dismissal reasons. |





#### ExecuteRecommendationsResponse







#### GetConfigRequest







#### GetRecommendationsRequest







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





#### LightningConfig




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `enabled` | [`bool`](#bool) |  |  |
| `channel_poll_interval` | [`uint64`](#uint64) |  |  |
| `static_address` | [`string`](#string) |  |  |
| `outbound_balance` | [`uint64`](#uint64) |  |  |
| `inbound_balance` | [`uint64`](#uint64) |  |  |
| `outbound_balance_percent` | [`float`](#float) |  |  |
| `inbound_balance_percent` | [`float`](#float) |  |  |
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
| `tenant` | [`string`](#string) | optional |  |





#### LightningRecommendation




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `swap` | [`LightningSwap`](#lightningswap) | optional | Populated when a swap is recommended for the associated `channel`, otherwise, the current balances are below the configured thresholds |
| `channel` | [`boltzrpc.LightningChannel`](#boltzrpc.lightningchannel) |  |  |
| `thresholds` | [`LightningThresholds`](#lightningthresholds) |  | the thresholds for a swap to be recommended for the `channel` |





#### LightningSwap




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `amount` | [`uint64`](#uint64) |  |  |
| `fee_estimate` | [`uint64`](#uint64) |  |  |
| `type` | [`boltzrpc.SwapType`](#boltzrpc.swaptype) |  |  |
| `dismissed_reasons` | [`string`](#string) | repeated | Reasons for which the swap is not being executed |





#### LightningThresholds




| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| `inbound` | [`uint64`](#uint64) | optional |  |
| `outbound` | [`uint64`](#uint64) | optional |  |





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

