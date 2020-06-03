# Boltz LND

The LND node to which this daemon connects to, has to be version `v0.10.0-beta` or higher. Also, LND needs to be compiled with these build flags (binaries from the official releases already include them):

- `routerrpc` (multi path payments)
- `chainrpc` (block listener)
- `walletrpc` (fee estimations)