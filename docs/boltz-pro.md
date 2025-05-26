---
description: >-
  Boltz Pro is a service designed to dynamically adjust swap fees based on
  Boltz's liquidity needs, helping to maintain wallet and Lightning channel
  balances.
---

# 🏅 Boltz Pro

## Basics

Boltz Client is the recommended way to programmatically interact with Boltz Pro to check for fee discounts, identify earn opportunities, and trigger swaps.

To configure Boltz Client to use the Boltz Pro API, simply start the daemon with the `--pro` startup flag or set the `pro` [configuration option](configuration.md). Since Boltz Pro discounts and earn opportunities are primarily available for Chain -> Lightning swaps, this guide will focus on that setup.

Boltz Client exposes a powerful [gRPC API](grpc.md), which you can integrate into your own applications. For scripted usage of `boltzcli`, use the `--json` flag, which is available on most commands.

The current fee rates can be retrieved using the [`GetPairs`](grpc.md#getpairs) endpoint or with the `boltzcli getpairs` command.

Here is an example for querying the current service fee for a Bitcoin -> Lightning swap using `boltzcli` and `jq` for processing the JSON output:

```bash
boltzcli getpairs --json | jq '.submarine[] | select(.pair.from == "BTC") | .fees.percentage'
```

## Paying Lightning Invoices

### **Paying invoices of your own node**

* [Connect Boltz Client to your CLN or LND node](README.md#configuration)
* Set the `amount` field to automatically generate a new invoice
* Example: `boltzcli createswap --amount 100000` (100k sats)

### **Paying invoices of an external service**

* [Start Boltz Client in standalone mode](README.md#standalone)
* Provide an existing invoice via `--invoice` or the `invoice` field
* Example: `boltzcli createswap --invoice lnbc1...`

## Funding Swaps

You can fund swaps in two ways:

1. Using Boltz Client's internal wallets
2. Using an external wallet

This choice is controlled by:

* API: `send_from_internal` parameter in [`CreateSwapRequest`](grpc.md#createswaprequest)
* CLI: `--from-wallet` (internal) or `--external-pay` (external)
