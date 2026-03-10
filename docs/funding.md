# 🏦 Funding Addresses

Funding addresses let you receive funds before creating a swap. This is useful
when you can't reliably predict the swap amount or if you want to simply decide
later. Funding addresses are currently available for `BTC` and `LBTC`.

## Create a Funding Address

Create a new funding address with:

```bash
boltzcli funding create BTC
```

Or for Liquid:

```bash
boltzcli funding create LBTC
```

The command returns the funding address ID, the address itself, and the timeout
block height.

::: info

Until the timeout block height is reached, funds sent to the address can only be
spent cooperatively with Boltz. After the timeout block height is reached, you
can spend the funds unilaterally. Keep this in mind when using funding
addresses.

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
address and uses the funded amount automatically, so you do not need to pass the
currency or amount again.

::: info

If you just pass `--from-funding` without an ID, like `boltzcli createswap --from-funding`, it 
looks through all available funding addresses. If exactly one eligible funding address exists, it is selected
automatically, otherwise you will be prompted to select one.

:::

### Chain to Lightning

Create a chain-to-lightning swap funded from an existing funding address:

```bash
boltzcli createswap --from-funding <funding_address_id>
```

In [standalone](standalone.md) mode, or if you want to provide the destination explicitly, also
pass an invoice:

```bash
boltzcli createswap --from-funding <funding_address_id> --invoice <bolt11_invoice>
```

If you want to know the exact Lightning invoice amount before creating the swap,
query a quote first. For submarine swaps, the `Receive Amount` shown in the
quote is the amount your invoice should request.

To get a quote for a funded address:

```bash
boltzcli quote --funding-address <funding_address_id> submarine
```

The funding address currency takes precedence here, so this works for both `BTC`
and `LBTC` funding addresses without having to pass `--from`.

This is especially useful in standalone mode, where you may want to calculate
the invoice amount first and then create the swap with:

```bash
boltzcli createswap --from-funding <funding_address_id> --invoice <bolt11_invoice>
```

You can also run `createswap` without specifying `--invoice`. `boltzcli` will
prompt you for an invoice with the correct amount, so you usually do not need to
run `quote` manually.

### Chain to Chain

Create a chain swap from a funded address without passing the amount again:

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

You can also spend directly from a funding address into an existing swap. The
expected swap amount and the amount locked in the funding address must match
exactly:

```bash
boltzcli funding swap <funding_address_id> <swap_id>
```

## Refund an Unused Funding Address

If you funded an address but do not want to use it for a swap, refund to an
external address:

```bash
boltzcli funding refund <funding_address_id> <address>
```

Or refund to one of your internal wallets:

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
