# AGENTS.md — boltz-client

## Project Overview

Go 1.24 client for the Boltz API enabling atomic swaps (submarine, reverse, chain) between on-chain (BTC/L-BTC) and Lightning. Produces two binaries: `boltzd` (daemon) and `boltzcli` (CLI). Uses Rust FFI for BDK, LWK, and BOLT12 sub-components. CGO is always required.

Module: `github.com/BoltzExchange/boltz-client/v2`

## Build Commands

```bash
# Full build (downloads GDK, builds Rust libs, compiles Go binaries)
make build

# Build individual Rust components
make build-bolt12     # internal/lightning/lib/bolt12
make build-lwk        # lwk/lwk_bindings
make build-bdk        # bdk/

# Install to $GOPATH/bin
make install

# Static build
make static
```

## Test Commands

```bash
# Run unit tests only (excludes integration tests)
make unit
# Equivalent to: CGO_ENABLED=1 go test -v -timeout 5m ./... -tags unit

# Run a single unit test by name
CGO_ENABLED=1 go test -v -timeout 5m -tags unit ./path/to/package -run TestName

# Run a single test with subtest
CGO_ENABLED=1 go test -v -timeout 5m -tags unit ./pkg/boltz/ -run TestCheckFees/SubtestName

# Run integration tests (requires regtest environment)
make integration
# Equivalent to: make start-regtest && CGO_ENABLED=1 go test -v -timeout 5m ./... -v

# Start regtest environment for integration tests
make start-regtest
```

**Build tags:**
- `-tags unit` — excludes integration tests (files with `//go:build !unit`)
- Integration tests live in: `internal/rpcserver/`, `internal/onchain/wallet/`, `internal/onchain/liquid-wallet/`, `internal/esplora/`, `internal/electrum/`, `pkg/boltz/ws_test.go`
- `-tags mempool` — special tag for `internal/mempool/mempool_test.go`

## Lint & Format

```bash
make lint       # golangci-lint run -v
make fmt        # gofmt -l -s -w .
```

Linter config is minimal (`.golangci.yml`): uses golangci-lint v2 defaults, excludes generated code in `internal/onchain/liquid-wallet/lwk/` and `internal/onchain/bitcoin-wallet/bdk/`.

## Code Generation

```bash
make proto          # Generate gRPC/REST from boltzrpc.proto
make cln-proto      # Generate CLN protobufs
make mockery        # Generate test mocks (configured in .mockery.yml)
make install-tools  # Install protoc plugins, mockery, git-chglog
```

## Directory Structure

```
cmd/boltzd/          — Daemon entry point
cmd/boltzcli/        — CLI entry point
internal/
  autoswap/          — Automated swap logic (lightning & chain)
  config/            — TOML config loading
  database/          — SQLite database layer, migrations
  cln/               — Core Lightning integration + protos
  lnd/               — LND integration
  lightning/         — Lightning node abstraction interface
  nursery/           — Swap lifecycle management
  onchain/           — On-chain operations, wallets (GDK, BDK, LWK)
  rpcserver/         — gRPC + REST API server
  macaroons/         — Macaroon authentication
  logger/            — Custom leveled logger
  mocks/             — Generated mock implementations
  utils/             — Shared utilities
pkg/
  boltz/             — Boltz API client library
  boltzrpc/          — Protobuf definitions & generated gRPC code
bdk/                 — Rust: Bitcoin Development Kit FFI crate
lwk/                 — Git submodule: Liquid Wallet Kit FFI
gdk/                 — Git submodule: Blockstream GDK
```

## Code Style Guidelines

### Naming Conventions

- **Packages:** lowercase single words (`boltz`, `database`, `autoswap`, `onchain`)
- **Exported types/functions:** PascalCase (`AutoSwap`, `ParseSwapType`, `GetVersion`)
- **Unexported types/functions:** camelCase (`shared`, `parseSwap`, `calcNetworkFee`)
- **Constants (string):** PascalCase (`ReasonMaxFeePercent`, `ReasonBudgetExceeded`)
- **Sentinel errors:** `Err` prefix (`ErrPartialSignaturesDisabled`, `ErrInvoiceNotFound`)
- **Receiver names:** short abbreviation of type (`boltz`, `swap`, `cfg`, `c`, `p`)
- **Files:** lowercase with underscores (`routing_hints.go`, `blocktime_test.go`)

### Type Patterns

```go
// String-based enums
type SwapType string
const (
    NormalSwap  SwapType = "submarine"
    ReverseSwap SwapType = "reverse"
    ChainSwap   SwapType = "chain"
)

// Sentinel errors
var ErrInvoiceNotFound = errors.New("invoice not found")

// Interfaces — small and focused
type Wallet interface {
    NewAddress() (string, error)
    SendToAddress(args WalletSendArgs) (string, error)
    Ready() bool
    GetBalance() (*Balance, error)
}
```

JSON struct tags use camelCase: `json:"fieldName,omitempty"`.

### Error Handling

- Return `(value, error)` tuples; wrap errors with `fmt.Errorf("context: %w", err)`
- No external error wrapping library; use stdlib `errors` and `fmt.Errorf`
- Sentinel errors at package level with `var ErrXxx = errors.New(...)`

### Logging

Package-level custom logger (`internal/logger`) with printf-style formatting:

```go
logger.Infof("Starting %s auto swapper", c.swapperType)
logger.Errorf("could not update config: %v", err)
logger.Debugf("Swap %s completed", swap.Id)
```

Levels: `Fatal`, `Error`, `Warn`, `Info`, `Debug`, `Silly`. Not structured logging.

### Testing

- **Framework:** `testify/require` for assertions (not `assert` — tests fail immediately)
- **Mocks:** Generated via `mockery` using `testify/mock`; expectations use `EXPECT()` pattern
- **Table-driven tests** are the standard pattern:

```go
tests := []struct {
    name   string
    input  int
    expect int
}{
    {"case1", 1, 2},
    {"case2", 3, 4},
}
for _, tc := range tests {
    tc := tc
    t.Run(tc.name, func(t *testing.T) {
        require.Equal(t, tc.expect, doSomething(tc.input))
    })
}
```

- Test variable: `tc` (or `tt`) for the loop variable
- Test helpers are package-level functions in `_test.go` files
- Tests are in the same package as the code (not `_test` suffix package)
- Mock expectations: `rpc.EXPECT().Method(mock.Anything).Return(value, nil)`

### Comments

- Minimal godoc; code is largely self-documenting through naming
- Inline comments explain non-obvious logic
- Use `//nolint:staticcheck` directives for specific lint suppressions

### Configuration

- TOML-based config files (`internal/config/`)
- CLI flags via `go-flags` struct tags
- Protobuf for RPC definitions with gRPC-Gateway for REST

### Generated Code — Do Not Edit

- `pkg/boltzrpc/*.pb.go`, `pkg/boltzrpc/**/*.pb.go` — protobuf generated
- `internal/cln/protos/*.pb.go` — CLN protobuf generated
- `internal/onchain/liquid-wallet/lwk/` — uniffi-bindgen-go generated
- `internal/onchain/bitcoin-wallet/bdk/` — uniffi-bindgen-go generated
- `internal/mocks/`, `internal/autoswap/*_mock.go` — mockery generated
