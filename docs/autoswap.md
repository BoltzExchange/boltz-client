# Autoswap

The autoswap feature of `boltz-client` allows for automated channel rebalancing.

The `boltzcli autoswap setup` command will guide you through the initial configuration of autoswap.

## Configuration

Configuration can be managed with the cli, using the `boltzcli autoswap config` command, e.g.

`boltzcli autoswap config enabled true`

You can manually edit the `autoswap.toml` file inside your data directory too, altough it has to be reload from disk using `boltzcli autoswap config --reload`.

The autoswapper wont create any swaps if `Enabled` is set to false, you can still view the swap recommendations using `boltzcli autoswap recommendations` which would be executed.

Autoswap has two modes of operation. It can either rebalance individual channels or only look at the total balance of your node, as if it was one big channel.
This behaviour can be controlled with the `PerChannel` parameter.

### Thresholds

The autoswapper will create normal swaps when the local balance goes below the minimum balance and reverse swaps when it exceeds the max balance.
These thresholds can be set as absolute amounts of sats (`MaxBalance` and `MinBalance`) or as percentage of total channel capacity (`MaxBalancePercent` and `MinBalancePercent`)

### Wallet

The autoswapper also needs access to a `Wallet` (specified by name) which normal swaps can be paid from and reverse swap funds will end up.
You can see a list of available wallets using `boltzcli wallet list`. Note that the wallet currency must be the same as the autoswap currency. 

### Swap Types

You can also choose to only create one `Type` of swap. 
If set to `reverse` only the max threshold has to be configured while the min will be set to 0. and the usage of a readonly `Wallet` is allowed.
If set to `normal` only the min threshold needs to be configured and the max will be set to the channel capacity.
If left empty, both reverse and normal swaps will be created.

### Budget

The autoswapper has a fixed amount of satoshis (`Budget`) it is allowed to spend on fees. This budget refreshes automatically after a specified internval (`BudgetInterval` in seconds).

### Full Example

```toml
Enabled = false
ChannelPollInterval = 30
LiquidAddress = ""
BitcoinAddress = ""
MaxBalance = 0
MinBalance = 0
MaxBalancePercent = 75.0
MinBalancePercent = 25.0
MaxFeePercent = 1.0
AcceptZeroConf = false
FailureBackoff = 86400
Budget = 100000
BudgetInterval = 604800
Currency = "L-BTC"
Type = ""
PerChannel = false
Wallet = "autoswap"
```
