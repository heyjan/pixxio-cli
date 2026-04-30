# AGENTS.md — pixxio-cli

## Project Overview

A Go CLI for the [pixx.io](https://www.pixx.io) Digital Asset Management API. Manages files, collections, and authentication from the terminal with three output modes (interactive, JSON, plain/TSV).

## Tech Stack

- **Language:** Go 1.23+
- **CLI Framework:** [Kong](https://github.com/alecthomas/kong) (struct-based, not Cobra)
- **Terminal:** [termenv](https://github.com/muesli/termenv) for colored output
- **API:** pixx.io REST API v1 — OpenAPI spec at `openapi.json`
- **Auth:** Bearer token via `Authorization: Bearer {api_key}` header
- **Build:** Makefile with `build`, `fmt`, `lint`, `test`, `ci` targets

## Project Structure

```
cmd/pixx/main.go                    # Entry point — 13 lines, calls cmd.Execute()
internal/
  cmd/                              # All CLI commands (single package, no sub-packages)
    root.go                         # CLI struct, RootFlags, Execute() orchestration
    exit.go                         # ExitError type with exit codes (0/1/2)
    auth.go                         # `pixx auth login` — stores credentials
    files.go                        # `pixx files {list,get,download,upload,delete}`
    collections.go                  # `pixx collections list`
    directories.go                  # `pixx directories list`
    files_test.go                   # Command tests — function variable swapping
    collections_test.go             # Collection command tests
  pixxapi/
    client.go                       # HTTP client for pixx.io API (all endpoints)
    client_test.go                  # httptest-based API mock tests
  config/
    config.go                       # XDG config (~/.config/pixx/config.json) + env overrides
  outfmt/
    outfmt.go                       # Output mode (interactive/JSON/plain) via context
  ui/
    ui.go                           # Terminal UI — colored output, stdout/stderr separation
  errfmt/
    errfmt.go                       # Error formatting with actionable hints
Makefile                            # Build system
.golangci.yml                       # Linter configuration
openapi.json                        # pixx.io API v1 OpenAPI specification
```

## Architecture Patterns

### Kong struct-based CLI

Commands are Go structs with struct tags. The entire CLI hierarchy is defined in `root.go`:

```go
type CLI struct {
    RootFlags   `embed:""`
    Auth        AuthCmd        `cmd:""`
    Files       FilesCmd       `cmd:""`
    Collections CollectionsCmd `cmd:""`
}
```

Each command has a `Run(ctx context.Context, flags *RootFlags) error` method.

### Context-based dependency passing

Everything flows through `context.Context` — no globals, no singletons:

```go
ctx = ui.WithUI(ctx, u)           // terminal UI
ctx = outfmt.WithMode(ctx, mode)  // output mode
ctx = config.WithConfig(ctx, cfg) // configuration
```

### Three output modes

Every command supports `--json`, `--plain`, and interactive (default). Data goes to stdout, diagnostics to stderr.

### Error handling

- `ExitError{Code: 2}` for usage errors
- `errfmt.Format()` maps errors to actionable hints
- Exit codes: 0 = success, 1 = error, 2 = usage

### Testability

- **API client:** `httptest.NewServer` with `rewriteTransport` to redirect HTTPS → test server
- **Commands:** package-level function variable `newAPIClient` swapped in tests via `t.Cleanup()`
- No external test frameworks beyond stdlib

## API Details

- **Base URL:** `https://{mediaspace}.px.media/api/v1/...`
- **Auth:** `Authorization: Bearer {api_key}` header on every request
- **Config precedence:** `--api-key` flag > `PIXX_API_KEY` env > `~/.config/pixx/config.json`
- **OpenAPI spec:** Full spec in `openapi.json` (61 endpoints, currently 4 implemented)

## Build & Test

```bash
make build    # Build binary to ./bin/pixx
make test     # Run all tests
make lint     # Run golangci-lint
make ci       # fmt-check + lint + test
make fmt      # Format code
```

## Key Files to Read First

1. `internal/cmd/root.go` — CLI structure and Execute() flow
2. `internal/pixxapi/client.go` — API client with all endpoints
3. `internal/cmd/files.go` — Example command implementation with all three output modes
4. `internal/pixxapi/client_test.go` — httptest patterns for API testing

## Conventions

- All packages under `internal/` — nothing is meant to be imported externally
- One file per command group (`files.go`, `collections.go`, `auth.go`)
- Tests next to code (`*_test.go`)
- Strict stdout (data) / stderr (diagnostics) separation
- `gofmt` for formatting (enforced in CI)
