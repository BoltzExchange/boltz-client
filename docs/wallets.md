# 💰 Wallets

`boltz-client` allows you to manage Bitcoin and [Liquid](https://liquid.net/) wallets using [GDK](https://github.com/Blockstream/gdk).

You can **create new** wallets, which will generate a new mnemonic and prompt you to back it up. We strongly recommend importing the displayed mnemonic into a wallet like [Green](https://blockstream.com/green/) on a second device. This serves as backup and UI to be able to monitior and control funds. Optionally, wallets can be encrypted with a passphrase that will be required on startup.

To create a new Liquid called "MyLiquidWallet" run:

`boltzcli wallet create MyLiquidWallet L-BTC`

Existing wallets can be **imported**. This can be done as hot wallet via mnemonic or cold read-only wallet via xpub or core descriptor. Read-only wallets can serve as a swap target for reverse swaps.

To import a Bitcoin wallet called "cold" run:

`boltzcli wallet import cold BTC`

A list of available wallets can be generated using `boltzcli wallet list`. The connected lightning node's internal wallet is available by default. All listed wallets can be used for manual swaps (e.g. `createreverseswap`) or [autoswap](autoswap.md).
