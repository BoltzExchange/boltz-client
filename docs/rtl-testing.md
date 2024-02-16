# ☂ Umbrel Beta

In the past months we worked on a major upgrade for [boltz-lnd](https://github.com/BoltzExchange/boltz-client/releases/tag/v1.2.7): [boltz-client](https://github.com/BoltzExchange/boltz-client). It is backwards compatible with boltz-lnd and will ship major new features like CLN support, Liquid and Taproot Swaps.&#x20;

## Goal

We are looking for cli-savvy Umbrel users who used Boltz swaps via RTL in the past, to test updating from boltz-lnd to our new boltz-client. The goal is to verify that the update procedure works as expected before we start rolling out to 1k+ umbrel users. We tested the procedure intensively ourselves and made sure that no data can be corrupted. In the unlikely event that the update procedure fails, you can simply continue running boltz-lnd. We'd ask you for some logs in this case.

{% hint style="info" %}
There is a bounty of 25k sats for the first 10 users successfully completing the upgrade [💸](https://emojipedia.org/money-with-wings)
{% endhint %}

## How it works

The update has to be performed manually in the cli of your umbrel server (`ssh umbrel@umbrel.local`):

```bash
cd umbrel
cp app-data/ride-the-lightning/boltz/.boltz-lnd/boltz.db boltz-lnd-backup.db
sudo ./scripts/repo checkout https://github.com/BoltzExchange/umbrel-apps.git
sudo ./scripts/app update ride-the-lightning
```

After updating, you should be able to open RTL and still see the old swap data and continue swapping like before.

Please verify that `boltz-client`, and not `boltz-lnd` is running using the cli:

```bash
alias boltzcli="sudo docker exec -it ride-the-lightning_boltz_1 boltzcli"
boltzcli getinfo
boltzcli listswaps
```

If anything goes wrong, you can restore the old state (feel free to contact us via the channels listed below should you need help):

```bash
sudo ./scripts/app stop ride-the-lightning
cp boltz-lnd-backup.db app-data/ride-the-lightning/boltz/.boltz-lnd/boltz.db
sudo ./scripts/repo checkout https://github.com/getumbrel/umbrel-apps
sudo ./scripts/app update ride-the-lightning
```

## Collect your Bounty

Get in touch via [Discord](https://discord.gg/QBvZGcW), [Twitter](https://twitter.com/boltzhq), [Nostr](https://snort.social/p/npub1psm37hke2pmxzdzraqe3cjmqs28dv77da74pdx8mtn5a0vegtlas9q8970) or [Email](mailto:hi@bol.tz) to collect your bounty by sending us a screenshot of your `boltzcli getinfo` `boltzcli listswaps` (feel free to remove swap details) and ideally a screenshot of RTL showing the swap history.
