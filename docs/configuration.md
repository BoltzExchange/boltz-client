# 🎛 Configuration

Boltz Client can be configured via a `TOML` config file or CLI arguments. By default, the config file is called `boltz.toml` and located in the data directory of Boltz Client (`~/.boltz` by default on Linux).

## Example

```toml
# Path to the log file
logfile = ""

# possible values: fatal, error, warn, info, debug, silly
loglevel = "info"

# possible values: "mainnet", "testnet" or "regtest"
network = "mainnet"

# you will have to set this to "cln" or "lnd" if you have configuration values for both
node = ""

# Whether to use Boltz Pro fee rates
pro = false

# The daemon can also operate without a lightning node
# standalone = true

# Custom referral ID to use when creating swaps
referralId = "my-referral"

[LIGHTNING]
# Default fee limit in ppm for lightning payments. Can be overridden on a per-swap basis.
routingFeeLimitPpm = 2500

[BOLTZ]
# By default the daemon automatically connects to the official Boltz Backend for the network your node is on
# This value is used to overwrite that
# url = "https://api.boltz.exchange"

[DATABASE]
# Path to the SQLite database file
# path = "~/test.db"

[LND]
# Host of the gRPC interface of LND
# host = "127.0.0.1"

# Port of the gRPC interface of LND
# port = 10009

# Path to the data directory of LND
# datadir = "~/.lnd"

# Path to a macaroon file of LND.
# The daemon needs to have permission to read various endpoints, generate addresses and pay invoices
# Not required if datadir is specified
# macaroon = "~/.lnd/data/chain/bitcoin/mainnet/admin.macaroon"

# Path to the TLS certificate of LND
# Not required if datadir is specified
# certificate = "~/.lnd/tls.cert"

[CLN]
# Host of the gRPC interface of CLN
# host = "127.0.0.1"

# Port of the gRPC interface of CLN
# port = 9736

# Path to the data directory of CLN
# datadir = "~/.lightning"

# Paths to TLS certificates and keys of CLN. Not required if datadir is specified
# rootcert = "~/.lightning/bitcoin/ca.pem"
# privatekey = "~/.lightning/bitcoin/client-key.pem"
# certchain =  "~/.lightning/bitcoin/client.pem"

[RPC]
# Host of the gRPC interface
host = "127.0.0.1"

# Port of the gRPC interface
port = 9002

# Whether the REST proxy for the gRPC interface should be disabled
restDisabled = false

# Host of the REST proxy
restHost = "127.0.0.1"

# Port of the REST proxy
restPort = 9003

# Path to the TLS cert for the gRPC and REST interface
tlsCert = ""

# Path to the TLS private key for the gRPC and REST interface
tlsKey = ""

# Whether the macaroon authentication for the gRPC and REST interface should be disabled
noMacaroons = false

# Path to the admin macaroon for the gRPC and REST interface
adminMacaroonPath = ""

# Path to the read-only macaroon for the gRPC and REST interface
readOnlyMacaroonPath = ""

# Password for authentication (alternative to macaroons and takes precedence if set)
password = ""
```
