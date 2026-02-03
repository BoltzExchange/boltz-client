# 🔁 Autoswap

The autoswap feature of `boltz-client` allows for automated channel rebalancing.

## Setup

The `boltzcli autoswap setup` command will guide you through the initial
configuration of autoswap.

## Advanced Configuration

Advanced configuration that is not covered in the setup process can be managed
using `boltzcli autoswap config`. For instance, run
`boltzcli autoswap config acceptZeroConf true` to enable
[0-conf swaps](https://docs.boltz.exchange/v/api/0-conf). Optionally, you can
instead edit the autoswap-specific `autoswap.toml` file inside your data
directory. To apply changes while the daemon is running, reload the config file
via `boltzcli autoswap config --reload`.

While autoswap won't create any swaps if `enabled` is set to false, you can
still view swap recommendations, swaps which were to be executed with the
current configuration, using `boltzcli autoswap recommendations`.

Autoswap can either rebalance individual channels or only look at the total
balance of your node. This behavior can be controlled with the `perChannel`
parameter.

### Thresholds

Autoswap will create normal swaps when the outbound balance goes below the
configured `outboundBalance` `outboundBalancePercent`. The same way, autoswap
will create reverse swaps when the inbound balance goes below the configured
`inboundBalance` or `inboundBalancePercent`. If the percentage values are
configured, the absolute thresholds will be calculated based on the channel
capacity.

#### Single threshold

If `inboundBalance` is configured as threshold, the rebalancing target will
always be the full channel capacity.

**Example**

- `inboundBalance` is set to 200k sats
- Current inbound balance of our 500k sats channel is 100k
- Result: A 400k sats reverse swap since the inbound balance is below the
  threshold

#### Both thresholds

If both thresholds are configured, the balance target will always be the middle
point between the two thresholds.

**Example**

- `outboundBalancePercent` and `inboundBalancePercent` are both set to 25%.
- Current outbound balance of our 100k sats channel is 20k.
- Result: A 30k sats submarine swap will be created since the outbound balance
  is below 25k sats (25% of 100k).

### Wallet

Autoswap needs access to a wallet (specified by name) which normal swaps use to
send funds from and reverse swaps to send funds to. You can see a list of
available wallets using `boltzcli wallet list`. Note that the wallet currency
has to match the autoswap currency.

### Swap Types

You can choose to only create one type of swap. If set to `reverse`, only the
max threshold has to be configured while the min will be set to 0. If set to
`normal`, only the min threshold needs to be configured and the max will be set
to the channel capacity. If left empty, both reverse and normal swaps will be
created and both thresholds will be considered.

### Budget

Autoswap has a fixed `budget` (in sats) it is allowed to spend on fees in a
specified `budgetInterval` (in seconds).

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
