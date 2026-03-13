---
name: add-rpc-endpoint
description: >
  Scaffold a new gRPC+REST endpoint across all layers: proto definition, REST
  annotation, code generation, macaroon permission, RPC handler, Go client
  wrapper, and CLI command. Follows existing boltz-client conventions.
---

# Add RPC Endpoint

Walk through every file that must be touched when adding a new gRPC+REST method
to the Boltz service (or AutoSwap service). The steps must be done in order
because later steps depend on generated code from earlier steps.

## Files touched (in order)

| # | File | What to add |
|---|------|-------------|
| 1 | `pkg/boltzrpc/boltzrpc.proto` | `rpc` method + request/response messages |
| 2 | `pkg/boltzrpc/rest-annotations.yaml` | HTTP verb + URL path mapping |
| 3 | *(run `make proto`)* | Regenerate Go bindings — do not hand-edit `*.pb.go` |
| 4 | `internal/macaroons/permissions.go` | Permission entry in `RPCServerPermissions` |
| 5 | `internal/rpcserver/router.go` | Handler on `routedBoltzServer` |
| 6 | `pkg/boltzrpc/client/client.go` | Convenience method on `Boltz` client struct |
| 7 | `cmd/boltzcli/commands.go` | `*cli.Command` variable + action function |
| 8 | `cmd/boltzcli/boltzcli.go` | Register command in `app.Commands` slice |

For the **AutoSwap** service the same pattern applies with these substitutions:
- Proto: `pkg/boltzrpc/autoswaprpc/autoswaprpc.proto`
- Handler: `internal/rpcserver/autoswap.go` (on `routedAutoSwapServer`)
- Client: `pkg/boltzrpc/client/autoswap.go`
- Permissions key: `/autoswaprpc.AutoSwap/MethodName`

## Step 1 — Proto definition

Add inside the `service Boltz { ... }` block in `pkg/boltzrpc/boltzrpc.proto`:

```protobuf
/*
Short description of what this endpoint does.
*/
rpc YourMethod (YourMethodRequest) returns (YourMethodResponse);
```

Then define the messages at the bottom of the file following existing patterns:

```protobuf
message YourMethodRequest {
  string some_field = 1;
}
message YourMethodResponse {
  string result = 1;
}
```

Use `snake_case` for proto field names. Use `optional` for fields that may be absent.
Field numbers must not collide with existing messages.

## Step 2 — REST annotation

Add a rule to `pkg/boltzrpc/rest-annotations.yaml`:

```yaml
# Read endpoints — use GET, path params with {field}:
- selector: boltzrpc.Boltz.YourMethod
  get: "/v1/yourmethod/{id}"

# Write endpoints — use POST with body:
- selector: boltzrpc.Boltz.YourMethod
  post: "/v1/yourmethod"
  body: "*"
```

Convention: `get:` for reads, `post:` with `body: "*"` for mutations.

## Step 3 — Code generation

Run:
```bash
make proto
```

This regenerates `boltzrpc.pb.go`, `boltzrpc_grpc.pb.go`, and `boltzrpc.pb.gw.go`.
The generated `BoltzServer` interface now requires the new method — the build will
fail until the handler is implemented.

## Step 4 — Macaroon permission

Add to `RPCServerPermissions` in `internal/macaroons/permissions.go`:

```go
"/boltzrpc.Boltz/YourMethod": {{
    Entity: "<entity>",
    Action: "<action>",
}},
```

Entity/Action conventions:
| Entity | Action | Used for |
|--------|--------|----------|
| `info` | `read` | Informational queries (GetInfo, GetPairInfo) |
| `swap` | `read` | Reading swap data (ListSwaps, GetSwapInfo) |
| `swap` | `write` | Creating/modifying swaps (CreateSwap, RefundSwap) |
| `wallet` | `read` | Reading wallet data (GetWallets, WalletReceive) |
| `wallet` | `write` | Modifying wallets (CreateWallet, WalletSend) |
| `admin` | `read` | Admin reads (ListTenants, VerifyWalletPassword) |
| `admin` | `write` | Admin writes (Stop, Unlock, BakeMacaroon) |
| `autoswap` | `read`/`write` | Autoswap configuration and status |

## Step 5 — RPC handler

Implement the method on `routedBoltzServer` in `internal/rpcserver/router.go`:

```go
func (server *routedBoltzServer) YourMethod(ctx context.Context, request *boltzrpc.YourMethodRequest) (*boltzrpc.YourMethodResponse, error) {
    // Tenant scoping:
    // tenantId := macaroons.TenantIdFromContext(ctx)

    // Business logic using server.database, server.boltz, server.onchain, etc.

    // Return gRPC status errors for client-facing problems:
    // return nil, status.Errorf(codes.NotFound, "not found")

    // Wrap internal errors:
    // return nil, fmt.Errorf("could not do thing: %w", err)

    return &boltzrpc.YourMethodResponse{...}, nil
}
```

No explicit registration is needed — the struct satisfies the generated `BoltzServer`
interface and is already registered via `boltzrpc.RegisterBoltzServer()` in `server.go`.
State-checking and macaroon interceptors are applied automatically.

## Step 6 — Client wrapper

Add a convenience method on the `Boltz` struct in `pkg/boltzrpc/client/client.go`:

```go
func (boltz *Boltz) YourMethod(request *boltzrpc.YourMethodRequest) (*boltzrpc.YourMethodResponse, error) {
    return boltz.Client.YourMethod(boltz.Ctx, request)
}
```

For simple parameterless methods, construct the request inline:

```go
func (boltz *Boltz) YourMethod() (*boltzrpc.YourMethodResponse, error) {
    return boltz.Client.YourMethod(boltz.Ctx, &boltzrpc.YourMethodRequest{})
}
```

## Step 7 — CLI command

Add a command variable and action function in `cmd/boltzcli/commands.go`:

```go
var yourMethodCommand = &cli.Command{
    Name:     "yourmethod",
    Category: "Category",  // "Infos", "Swaps", "Wallet", "Auto", etc.
    Usage:    "Short description of what this does",
    Action:   yourMethodAction,
    Flags: []cli.Flag{
        jsonFlag,
    },
}

func yourMethodAction(ctx *cli.Context) error {
    client := getClient(ctx)
    response, err := client.YourMethod(&boltzrpc.YourMethodRequest{
        // populate from ctx.Args() or ctx.String("flag"), etc.
    })
    if err != nil {
        return err
    }
    if ctx.Bool("json") {
        printJson(response)
    } else {
        // formatted human-readable output
    }
    return nil
}
```

## Step 8 — Register CLI command

Add the command to the `app.Commands` slice in `cmd/boltzcli/boltzcli.go`,
grouped with related commands:

```go
app.Commands = []*cli.Command{
    // ...existing commands...
    yourMethodCommand,
}
```

## Verification

After all steps, confirm correctness:

```bash
make proto                # regenerate (should be idempotent now)
go build ./...            # must compile
go test -tags unit ./...  # must pass
```

If the build fails after `make proto`, the most common cause is the handler
signature not matching the generated interface exactly.
