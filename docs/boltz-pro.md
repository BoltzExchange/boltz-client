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

Boltz Client exposes a powerful[ gRPC API](grpc.md), but can also be controlled programmatically via `boltzcli`.&#x20;

Current Boltz Pro fees for a specific pair can be retrieved using the [`PairInfo`](grpc.md#pairinfo) endpoint.

## Pay External Lightning Invoices

If you are paying Lightning invoices of an external service with Boltz Pro, you should also set the `--standalone` startup flag. This starts the daemon without connecting to a local Lightning node.

You can then execute swaps using either [Boltz Client's internal wallets](wallets.md) or fund them from an external wallet. This choice is managed by the `send_from_internal` in the [`CreateSwapRequest`](grpc.md#createswaprequest). The invoice to be paid is also specified here. The same can be achieved by using `boltzcli`  with the `--from-wallet`, `--external-pay`, and `--invoice` arguments, respectively.

## Pay Lightning Invoices of your own node

The first step here is to [configure](configuration.md) the connection to your own CLN or LND node. When creating a swap request, if you set the `amount` field, Boltz Client automatically fetches a new invoice for that amount (in satoshis) from the connected Lightning node. The same external and internal wallet options described above apply.
