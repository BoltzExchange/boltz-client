# Autoswap

The autoswap feature of `boltz-client` allows for automated channel rebalancing.

## Setup

The `boltzcli autoswap setup` command will guide you through the initial configuration of autoswap.

## Advanced Configuration

Advanced configuration that is not covered in the setup process can be managed using `boltzcli autoswap config`. For instance, run `boltzcli autoswap config AcceptZeroConf true` to enable [0-conf swaps](https://docs.boltz.exchange/v/api/0-conf). Optionally, you can instead edit the `autoswap.toml` file inside your data directory. To apply changes while the daemon is running, reload the config file via `boltzcli autoswap config --reload`.

While autoswap won't create any swaps if `Enabled` is set to false, you can still view swap "recommendations" using `boltzcli autoswap recommendations`, which were to be executed using the current configuration.

Autoswap can either rebalance individual channels or only look at the total balance of your node. This behaviour can be controlled with the `PerChannel` parameter.

### Thresholds

Autoswap will create normal swaps when the local balance goes below the minimum balance and reverse swaps when it exceeds the max balance. These thresholds can be set as absolute amounts of sats (`MaxBalance` and `MinBalance`) or as percentage of total channel capacity (`MaxBalancePercent` and `MinBalancePercent`).

### Wallet

Autoswap needs access to a wallet (specified by name) which normal swaps use to send funds from and reverse swaps to send funds to. You can see a list of available wallets using `boltzcli wallet list`. Note that the wallet currency has to match the autoswap currency. 

### Swap Types

You can choose to only create one type of swap. If set to `reverse`, only the max threshold has to be configured while the min will be set to 0. If set to `normal`, only the min threshold needs to be configured and the max will be set to the channel capacity. If left empty, both reverse and normal swaps will be created.

### Budget

Autoswap has a fixed `Budget` (in sats) it is allowed to spend on fees in a specified `BudgetInterval` (in seconds).

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
