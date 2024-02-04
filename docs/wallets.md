# Wallets

`boltz-client` allows you to manage both bitcoin and liquid wallets using [GDK](https://github.com/Blockstream/gdk).

You can create new wallets using `boltzcli wallet create`. It will generate a new mnemonic which you should backup carefully.

`boltzcli wallet create liquid L-BTC`

Existing wallets can be imported aswell. These can also be cold wallets which can be used as a target for reverse swap funds.

`boltzcli wallet import cold BTC`

A list of available wallets can be generated using `boltzcli wallet list`. This will include the lightning nodes BTC wallet.
These wallets can be used for manual (see cli examples) swaps or [autoswap](autoswap.md)

