---
name: ci-review
description: >
  Replicate the full GitHub Actions CI pipeline locally (gofmt, golangci-lint,
  go build, unit tests, docs prettier/spell-check). Automatically fix any
  issues found and iterate until every step passes.
---

# CI Review Skill

Replicate the full GitHub Actions CI pipeline locally: lint, build, unit tests, and docs checks. Fix any issues found and iterate until all steps pass.

## Steps

### 1. Format check (gofmt)

Run `gofmt -l -s .` and collect the list of files with formatting issues. Exclude generated directories:
- `internal/onchain/liquid-wallet/lwk/`
- `internal/onchain/bitcoin-wallet/bdk/`
- `internal/mocks/`
- `pkg/boltzrpc/` (`.pb.go` and `.pb.gw.go` files)
- `internal/cln/protos/` (`.pb.go` files)

If any files are reported, fix them by running `gofmt -l -s -w <file>` for each file, then re-run the check to confirm. Report which files were reformatted.

### 2. Lint (golangci-lint)

Run:
```bash
golangci-lint run -v 2>&1
```

If `golangci-lint` is not installed, install it:
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.0
```

The project uses golangci-lint v2 with minimal config (`.golangci.yml`). Generated code in `internal/onchain/liquid-wallet/lwk/` and `internal/onchain/bitcoin-wallet/bdk/` is excluded automatically by the config.

If lint errors are found:
1. Read each reported file and understand the violation.
2. Fix each issue. Common fixes include: unused variables/imports, missing error checks, shadowed variables, ineffectual assignments.
3. Do NOT fix issues in generated code (`*.pb.go`, `*.pb.gw.go`, `*_mock.go`, `internal/onchain/liquid-wallet/lwk/`, `internal/onchain/bitcoin-wallet/bdk/`).
4. Re-run `golangci-lint run -v` to confirm all issues are resolved.
5. If new lint errors appear after fixing, repeat.

### 3. Build

Run:
```bash
CGO_ENABLED=1 go build -v ./...
```

This compiles all packages without producing binaries (faster than `make build` since it skips Rust FFI and GDK downloads). If compilation depends on Rust FFI libraries that are not built, fall back to:
```bash
make build
```

If build errors are found:
1. Read the error messages and identify the source files.
2. Fix compilation errors (type mismatches, missing imports, undefined references, etc.).
3. Re-run the build to confirm.
4. Iterate until the build succeeds.

### 4. Unit tests

Run:
```bash
CGO_ENABLED=1 go test -v -timeout 5m ./... -tags unit 2>&1
```

If test failures are found:
1. Identify the failing test(s) and the package they belong to.
2. Read the test file and the source code under test.
3. Determine if the failure is in the test or the source code. Fix whichever is wrong.
4. Re-run only the failing test to confirm the fix:
   ```bash
   CGO_ENABLED=1 go test -v -timeout 5m -tags unit ./path/to/package -run TestName
   ```
5. After all individual fixes, re-run the full unit test suite to check for regressions.
6. Iterate until all tests pass.

### 5. Docs checks (if docs/ was modified)

Check if any files under `docs/` were modified compared to the main branch:
```bash
git diff --name-only HEAD~1 -- docs/ 2>/dev/null || git diff --name-only main -- docs/ 2>/dev/null
```

If docs were modified, run the following checks:

#### 5a. Prettier formatting check
```bash
cd docs && npx prettier --check "*.md" "*.json"
```
If formatting issues are found, fix them:
```bash
cd docs && npx prettier --write "*.md" "*.json"
```

#### 5b. Spell check with typos
```bash
typos ./docs/
```
If `typos` is not installed, install it:
```bash
cargo install typos-cli
```
If spelling errors are found, fix them in the source files. Use the suggestions from `typos` output. Re-run to confirm.

### 6. Final verification

After all fixes, run the complete pipeline one more time to confirm everything passes:

1. `gofmt -l -s .` — should produce no output
2. `golangci-lint run -v` — should pass clean
3. `CGO_ENABLED=1 go build -v ./...` — should compile
4. `CGO_ENABLED=1 go test -v -timeout 5m ./... -tags unit` — all tests should pass

Report a summary of:
- What checks passed on the first run
- What issues were found and fixed (with file names and brief descriptions)
- Whether all checks now pass
- Any issues that could NOT be auto-fixed (e.g., ambiguous test failures needing human judgment)

## Notes

- Integration tests (`make integration`) are NOT run — they require a regtest Docker environment that is unlikely to be available locally.
- The `make build` target requires Rust toolchain and Docker for GDK. If these are unavailable, use `go build ./...` which validates Go compilation only.
- Never modify generated code. Generated files live in:
  - `pkg/boltzrpc/*.pb.go`, `pkg/boltzrpc/**/*.pb.go`
  - `internal/cln/protos/*.pb.go`
  - `internal/onchain/liquid-wallet/lwk/`
  - `internal/onchain/bitcoin-wallet/bdk/`
  - `internal/mocks/`, `internal/autoswap/*_mock.go`
- When fixing lint or build errors, follow the project code style documented in `AGENTS.md`.
