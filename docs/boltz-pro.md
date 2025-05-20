---
description: >-
  Boltz Pro is a service designed to dynamically adjust swap fees based on
  Boltz's liquidity needs, helping to maintain wallet and Lightning channel
  balances.
---

# 🏅 Boltz Pro

## Basics

Boltz Client is the recommended way to programmatically interact with Boltz Pro to check for fee discounts, identify earn opportunities, and trigger swaps.

To configure Boltz Client to use the Boltz Pro API, simply start the daemon with the `--pro` startup flag. Since Boltz Pro discounts and earn opportunities are primarily available for Chain -> Lightning swaps, this guide will focus on that setup.

Boltz Client exposes a powerful [gRPC API](grpc.md), which you can integrate into your own applications or use via `boltzcli`.&#x20;

The current fee rates can be retrieved using the [`GetPairs`](grpc.md#getpairs) endpoint or with the `boltzcli getpairs` command.

Here is an example for querying the current service fee for a Bitcoin -> Lightning swap using a `boltzcli` and `jq` for processing the JSON:

```bash
boltzcli getpairs --json | jq '.submarine[] | select(.pair.from == "BTC") | .fees.percentage'
```

## Paying Lightning Invoices

You can get Lightning invoices for your swaps from:

1. **Your own node** [(CLN or LND)](README.md#configuration)
   - Set the `amount` field to automatically generate a new invoice
   - Example: `boltzcli createswap --amount 100000` (100k sats)

2. **Standalone mode** [(without connecting to a node)](README.md#standalone)
   - Provide an existing invoice via `--invoice` or the `invoice` field
   - Example: `boltzcli createswap --invoice lnbc1...`

### Funding Options

You can fund swaps in two ways:
1. Using Boltz Client's internal wallets
2. Funding externally

This choice is controlled by:
- API: `send_from_internal` parameter in [`CreateSwapRequest`](grpc.md#createswaprequest)
- CLI: `--from-wallet` (internal) or `--external-pay` (external)