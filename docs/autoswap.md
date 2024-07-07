# 🔁 Autoswap

The autoswap feature of `boltz-client` allows for automated channel rebalancing.

## Setup

The `boltzcli autoswap setup` command will guide you through the initial configuration of autoswap.

## Advanced Configuration

Advanced configuration that is not covered in the setup process can be managed using `boltzcli autoswap config`. For instance, run `boltzcli autoswap config acceptZeroConf true` to enable [0-conf swaps](https://docs.boltz.exchange/v/api/0-conf). Optionally, you can instead edit the autoswap-specific `autoswap.toml` file inside your data directory. To apply changes while the daemon is running, reload the config file via `boltzcli autoswap config --reload`.

While autoswap won't create any swaps if `enabled` is set to false, you can still view swap recommendations, swaps which were to be executed with the current configuration, using `boltzcli autoswap recommendations.`

Autoswap can either rebalance individual channels or only look at the total balance of your node. This behavior can be controlled with the `perChannel` parameter.

### Thresholds

Autoswap will create normal swaps when the outbound balance goes below the minimum balance and reverse swaps when it exceeds the max balance. These thresholds can be set as absolute amounts of sats (`maxBalance` and `minBalance`) or as percentage of total channel capacity (`maxBalancePercent` and `minBalancePercent`).

The balance target will always be the average of the two thresholds.

Example:
Total channel balance: 100000sat
Outbound balance: 80000sat
Min/Max thresholds: 25%/75%
In this situation, a reverse swap for 30000sats will be created since the target balance is 50000sat (50% of 100000sat).

### Wallet

Autoswap needs access to a wallet (specified by name) which normal swaps use to send funds from and reverse swaps to send funds to. You can see a list of available wallets using `boltzcli wallet list`. Note that the wallet currency has to match the autoswap currency.

### Swap Types

You can choose to only create one type of swap. If set to `reverse`, only the max threshold has to be configured while the min will be set to 0. If set to `normal`, only the min threshold needs to be configured and the max will be set to the channel capacity. If left empty, both reverse and normal swaps will be created and both thresholds will be considered.

### Budget

Autoswap has a fixed `budget` (in sats) it is allowed to spend on fees in a specified `budgetInterval` (in seconds).

### Full Example

```toml
acceptZeroConf = false
budget = "100000"
budgetInterval = "604800"
channelPollInterval = "30"
currency = "LBTC"
enabled = true
failureBackoff = "86400"
inboundBalance = "0"
inboundBalancePercent = 25.0
maxFeePercent = 1.0
maxSwapAmount = "0"
outboundBalance = "0"
outboundBalancePercent = 25.0
perChannel = false
staticAddress = ""
swapType = "reverse"
wallet = "autoswap"
```
