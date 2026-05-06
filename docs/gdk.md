---
description: GDK support has been removed from Boltz Client
---

# GDK Support Removed

Boltz Client no longer builds or links Blockstream GDK. Bitcoin wallets use BDK
and Liquid wallets use LWK.

Legacy GDK wallet credentials are migrated to descriptor-based credentials on
startup when possible. If migration is not possible, re-import the wallet from
your backup and remove the old database entry with `boltzcli wallet remove`.
