# Configuration

`boltz-client` can be configured via CLI arguments or a TOML config file. When CLI arguments are used, these are overwriting any conflicting configuration in the TOML. The config file `boltz.toml` is expected to be located in the data directory of `boltz-client` (`/home/<user>/.boltz-client` by default on Linux).

## Example

```toml
# Path the the log file
logfile = ""

# Prefix for all log messages.
# Useful in cases two boltz-client instances (one for BTC and LTC) are running in a single Docker container
logprefix = "[BTC] "

[BOLTZ]
# By default the daemon automatically connects to the official Boltz instance for the network LND is on
# This value is used to override that
url = "https://testnet.boltz.exchange/api"

[DATABASE]
# Path to the SQLite database file
path = "/home/michael/test.db"

[LND]
# Host of the gRPC interface of LND
host = "127.0.0.1"

# Port of the gRPC interface of LND
port = 10009

# Path to a macaroon file of LND
# The daemon needs to have permission to read various endpoints, generate addresses and pay invoices
macaroon = ""

# Path to the TLS certificate of LND
certificate = ""

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

# Path to the read only macaroon for the gRPC and REST interface
readOnlyMacaroonPath = ""
```
