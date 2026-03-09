# 🏦 Funding Addresses

Funding addresses let you pre-fund a swap before you actually create it. This is
useful when you want to receive coins first and decide later whether to use them
for a submarine swap or a chain swap, or when you do not want to fund the swap
from an internal wallet.

Funding addresses are currently available for `BTC` and `LBTC`.

## Create a Funding Address

Create a new funding address with:

```bash
boltzcli funding create BTC
```

Or for Liquid:

```bash
boltzcli funding create LBTC
```

The command returns the funding address ID, the address itself, and its timeout
block height.

::: info


Until the timeout block height is reached, funds sent to the address can only be spent 
together with boltz. After the timeout block height is reached, you can spend the funds unilaterally.
Keep this in mind when using funding addresses.

:::

## Check Funding Status

To list all funding addresses:

```bash
boltzcli funding list
```

To filter by currency:

```bash
boltzcli funding list --currency BTC
```

To watch updates for a single funding address in real time:

```bash
boltzcli funding stream <funding_address_id>
```

To watch updates for all funding addresses:

```bash
boltzcli funding stream
```

After sending coins to the funding address, `boltzcli funding list` or
`boltzcli funding stream` will show the detected amount and status.

## Use a Funding Address for a Swap

Once a funding address has received coins, you can create a swap from it. When
using `--from-funding`, `boltzcli` derives the source currency from the funding
address automatically, so you do not need to pass the amount again.

::: info

If you pass `--from-funding` without an ID, `boltzcli` looks
for available funds. If exactly one eligible funding address exists,
it is selected automatically, otherwise you will be prompted to select one.

::: 

### Chain to Lightning

Create a normal swap funded from an existing funding address:

```bash
boltzcli createswap --from-funding <funding_address_id>
```

In standalone mode, or if you want to provide the destination explicitly, pass
an invoice as well:

```bash
boltzcli createswap --from-funding <funding_address_id> --invoice <bolt11_invoice>
```

If you want to know the exact Lightning invoice amount before creating the swap,
query a quote first. For submarine swaps, the `Receive Amount` in the quote is
the amount your invoice should request.

For a BTC funding address:

```bash
boltzcli quote --funding-address <funding_address_id> submarine
```

The funding address currency takes priority here, so this works for both `BTC`
and `LBTC` funding addresses without having to pass `--from`.

```bash
boltzcli quote --funding-address <funding_address_id> submarine
```

This is especially useful in standalone mode, where you may want to calculate
the invoice amount first and then create the swap with:

```bash
boltzcli createswap --from-funding <funding_address_id> --invoice <bolt11_invoice>
```

You can also run `createswap` without specifying the invoice and you will 
be prompted to enter an invoice for the correct amount without requiring manual usage of the 
`quote` command.

### Chain to Chain

Create a chain swap from a funded address without specifying the amount again:

```bash
boltzcli createchainswap --from-funding <funding_address_id> --to-address <destination_address>
```

For example, to swap from a funded `LBTC` address to a Bitcoin address:

```bash
boltzcli createchainswap --from-funding <funding_address_id> --to-address bc1q...
```

If you want the output to go to an internal wallet instead:

```bash
boltzcli createchainswap --from-funding <funding_address_id> --to-wallet cold
```

## Fund an Existing Swap Manually

You can also spend directly from a funding address into an already created swap, where the expected swap amount
and amount locked in the funding address have to match up exactly:

```bash
boltzcli funding swap <funding_address_id> <swap_id>
```

## Refund an Unused Funding Address

If you funded an address but do not want to use it for a swap, refund it to an
external address:

```bash
boltzcli funding refund <funding_address_id> <address>
```

Or refund it to one of your internal wallets:

```bash
boltzcli funding refund <funding_address_id> <wallet_name>
```

## Restore Funding Addresses

Funding addresses are derived from the swap mnemonic, so they can be restored if
the local database is lost.

Restore them with:

```bash
boltzcli funding restore --mnemonic "abandon ... about"
```

You can inspect the mnemonic used by the daemon with:

```bash
boltzcli swapmnemonic get
```
