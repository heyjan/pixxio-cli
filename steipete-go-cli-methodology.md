# Go CLI Methodology: Lessons from steipete/gogcli

A methodological breakdown of how steipete builds production-grade Go CLI tools.
Not about what gogcli does (Google Workspace CLI) but **how** it's built.

---

## 1. Project Structure

```
cmd/gog/main.go              # 14 lines. Delegates everything.
internal/
  cmd/                        # ALL commands live here (271 .go files, 331 test files)
    root.go                   # CLI struct, Execute(), flag wiring
    gmail.go                  # Command group struct
    gmail_send.go             # Actual command implementation
    gmail_send_test.go        # Tests next to code
    exit.go                   # ExitError type
    usage.go                  # Shared validation helpers
    strings.go                # Shared string utilities
  googleapi/                  # One file per API service wrapper
  googleauth/                 # OAuth2 flows, token handling
  config/                     # JSON5 config with file locking
  secrets/                    # OS keychain + encrypted fallback
  outfmt/                     # JSON/plain/interactive output modes
  ui/                         # Terminal colors via termenv
  errfmt/                     # Error formatting with actionable hints
  authclient/                 # Context-based auth passing
  input/                      # Prompt/readline
go.mod
Makefile
.goreleaser.yaml
.golangci.yml
```

**Key decisions:**
- Everything under `internal/` -- no `pkg/` directory. Nothing is meant to be imported by others.
- All commands in a single `internal/cmd` package -- no sub-packages per command group. This keeps cross-command sharing trivial (helpers, test utilities, shared types).
- Each "concern" gets its own package (`outfmt`, `errfmt`, `ui`, `secrets`), but they're small and focused.
- The `cmd/gog/main.go` entry point is deliberately tiny (14 lines). It calls `cmd.Execute()` and maps the error to an exit code. Nothing else.

```go
func main() {
    if err := cmd.Execute(os.Args[1:]); err != nil {
        os.Exit(cmd.ExitCode(err))
    }
}
```

## 2. CLI Framework: Kong (not Cobra)

steipete uses [Kong](https://github.com/alecthomas/kong) instead of Cobra. Kong is struct-based rather than builder-based.

**Why this matters:** The entire CLI hierarchy is defined as nested Go structs with struct tags. No `init()` functions, no builder chains, no `AddCommand()` calls scattered across files.

```go
type CLI struct {
    RootFlags `embed:""` // Global flags available to all commands

    // Top-level aliases ("desire paths" for quick access)
    Send   GmailSendCmd `cmd:"" name:"send" help:"Send an email (alias for 'gmail send')"`
    Ls     DriveLsCmd   `cmd:"" name:"ls" aliases:"list" help:"List Drive files"`

    // Command groups
    Gmail    GmailCmd    `cmd:"" aliases:"mail,email" help:"Gmail"`
    Calendar CalendarCmd `cmd:"" aliases:"cal" help:"Google Calendar"`
    Drive    DriveCmd    `cmd:"" aliases:"drv" help:"Google Drive"`
    // ...
}

type GmailCmd struct {
    Send   GmailSendCmd   `cmd:"" name:"send"`
    Search GmailSearchCmd `cmd:"" name:"search"`
    Labels GmailLabelsCmd `cmd:"" name:"labels"`
}
```

**Each command is a struct with a `Run` method:**

```go
type GmailSendCmd struct {
    To      string   `name:"to" help:"Recipients (comma-separated)"`
    Subject string   `name:"subject" help:"Subject (required)"`
    Body    string   `name:"body" help:"Body (plain text)"`
    Attach  []string `name:"attach" help:"Attachment file path (repeatable)"`
}

func (c *GmailSendCmd) Run(ctx context.Context, flags *RootFlags) error {
    // Validation → API call → output
}
```

**Struct tag features used:**
- `name:"..."` -- flag/command name
- `help:"..."` -- help text
- `aliases:"..."` -- short alternatives
- `short:"n"` -- single-char flag
- `default:"${var}"` -- defaults from kong.Vars (env-aware)
- `hidden:""` -- hide from help output
- `embed:""` -- embed flags from another struct
- `env:"GOG_ACCESS_TOKEN"` -- bind to env var

## 3. Root Flags (Global State)

All global flags live in a single embedded struct:

```go
type RootFlags struct {
    Color           string `help:"Color output: auto|always|never" default:"${color}"`
    Account         string `help:"Account email" aliases:"acct" short:"a"`
    JSON            bool   `help:"Output JSON" short:"j"`
    Plain           bool   `help:"Output stable text (TSV)" short:"p"`
    DryRun          bool   `help:"Preview changes" aliases:"noop,preview" short:"n"`
    Force           bool   `help:"Skip confirmations" aliases:"yes" short:"y"`
    NoInput         bool   `help:"Never prompt; fail instead" aliases:"non-interactive"`
    Verbose         bool   `help:"Enable verbose logging" short:"v"`
    Select          string `name:"select" help:"Select fields in JSON output (dot paths)"`
    ResultsOnly     bool   `name:"results-only" help:"Emit only results (no envelope)"`
    EnableCommands  string `help:"Comma-separated allow-list"`
    DisableCommands string `help:"Comma-separated block-list"`
    GmailNoSend     bool   `help:"Block Gmail send (agent safety)"`
}
```

These are passed to every command's `Run()` via Kong's dependency injection: `kctx.Bind(&cli.RootFlags)`.

## 4. The Execute() Flow

The `Execute()` function in `root.go` is the orchestration backbone. The sequence:

1. **Rewrite args** -- "desire path" rewrites (e.g., `--fields` → `--select`)
2. **Parse** via Kong
3. **Enforce safety** -- baked safety profile, enabled/disabled command lists, `--gmail-no-send`
4. **Set up logging** -- `log/slog` to stderr, level from `--verbose`
5. **Auto-detect output mode** -- if piped + `GOG_AUTO_JSON=1` → JSON
6. **Wire context** -- output mode, auth client, access token, UI → `context.Context`
7. **Run the command** -- `kctx.Run()`
8. **Format errors** -- via `errfmt.Format()` to stderr
9. **Return exit code** -- mapped through `ExitCode(err)`

Everything is passed through `context.Context` -- not globals, not singletons.

## 5. Output System (Three Modes)

Every command supports three output modes, switched by global flags:

| Mode | Flag | Use case | Format |
|------|------|----------|--------|
| Interactive | (default) | Human in terminal | Colored text |
| JSON | `--json` / `-j` | Scripting, piping | Pretty JSON |
| Plain | `--plain` / `-p` | Stable parsing | TSV-like, no colors |

```go
mode, _ := outfmt.FromFlags(cli.JSON, cli.Plain)
ctx = outfmt.WithMode(ctx, mode)
```

Commands check the mode and format accordingly:

```go
if outfmt.IsJSON(ctx) {
    return outfmt.WriteJSON(ctx, os.Stdout, result)
}
u.Out().Printf("Sent to %s", recipient)
```

**JSON transforms** are built in -- `--results-only` strips envelope metadata (pagination tokens, counts), `--select` projects specific fields via dot-path notation. This is handled generically in `outfmt.WriteJSON()`.

## 6. Error Handling

**Three-layer approach:**

**Layer 1 -- ExitError with codes:**
```go
type ExitError struct {
    Code int
    Err  error
}
```
Exit codes: 0 = success, 1 = general error, 2 = usage/validation error.

**Layer 2 -- Usage errors return exit code 2:**
```go
func usage(msg string) error {
    return &ExitError{Code: 2, Err: errors.New(msg)}
}
// Used in commands:
if c.To == "" { return usage("required: --to") }
```

**Layer 3 -- Error formatting with actionable hints:**
```go
// errfmt.Format() matches error types and gives the user something to DO:
// AuthRequiredError → "Run: gog auth add <email> --services gmail"
// CredentialsMissingError → "Download from: https://console.cloud.google.com/..."
// keyring.ErrKeyNotFound → "Run: gog auth add <email>"
// Google API error → extract code + message
```

The user never sees a raw stack trace. Every error gets mapped to a human-readable message with a suggested fix.

## 7. UI System

A thin wrapper around [termenv](https://github.com/muesli/termenv) for colored terminal output:

```go
u, _ := ui.New(ui.Options{
    Stdout: os.Stdout,
    Stderr: os.Stderr,
    Color:  "auto", // auto|always|never
})
ctx = ui.WithUI(ctx, u)

// In commands:
u := ui.FromContext(ctx)
u.Out().Printf("Created: %s", name)       // stdout
u.Out().Successf("Done!")                  // green
u.Err().Errorf("Warning: %s", msg)         // red, stderr
```

**Strict stdout/stderr separation:** data goes to stdout, diagnostics/errors go to stderr. This makes piping reliable.

## 8. Context-Based Dependency Passing

Rather than globals or constructor injection, everything flows through `context.Context`:

```go
ctx = outfmt.WithMode(ctx, mode)          // output mode
ctx = outfmt.WithJSONTransform(ctx, t)    // field selection
ctx = authclient.WithClient(ctx, client)  // OAuth client
ctx = authclient.WithAccessToken(ctx, tk) // access token
ctx = ui.WithUI(ctx, u)                   // terminal UI
```

Commands pull what they need:

```go
func (c *SomeCmd) Run(ctx context.Context, flags *RootFlags) error {
    u := ui.FromContext(ctx)
    if outfmt.IsJSON(ctx) { ... }
}
```

## 9. API Client Pattern

Service wrappers are thin factory functions that handle auth:

```go
// internal/googleapi/gmail.go
func NewGmail(ctx context.Context, email string) (*gmail.Service, error) {
    opts, err := optionsForAccount(ctx, googleauth.ServiceGmail, email)
    if err != nil { return nil, fmt.Errorf("gmail options: %w", err) }
    svc, err := gmail.NewService(ctx, opts...)
    if err != nil { return nil, fmt.Errorf("create gmail service: %w", err) }
    return svc, nil
}
```

**Testability via function variables:**
```go
// In cmd package:
var newGmailService = googleapi.NewGmail

// In tests:
origNew := newGmailService
t.Cleanup(func() { newGmailService = origNew })
newGmailService = func(ctx context.Context, email string) (*gmail.Service, error) {
    return fakeService, nil
}
```

This avoids interfaces for simple cases -- a package-level function variable is swapped in tests and restored via `t.Cleanup()`.

## 10. Testing Methodology

**331 test files, ~55% of all files.** Patterns:

**a) httptest for API mocking:**
```go
func newGmailServiceForTest(t *testing.T, handler http.HandlerFunc) (*gmail.Service, func()) {
    srv := httptest.NewServer(handler)
    svc, _ := gmail.NewService(context.Background(),
        option.WithEndpoint(srv.URL+"/"),
        option.WithHTTPClient(srv.Client()),
    )
    return svc, srv.Close
}
```

The handler inspects the request path/method and returns canned JSON. No third-party mock library.

**b) Captured stdout/stderr:**
```go
out := captureStdout(t, func() {
    _ = Execute([]string{"gmail", "send", "--to", "test@example.com"})
})
// Assert on `out`
```

**c) Function variable swapping (see API Client Pattern above)**

**d) Integration tests with build tags:**
```go
//go:build integration
// Requires: GOG_IT_ACCOUNT=your@gmail.com
// Runs against real Google APIs
```

**e) No test frameworks beyond stdlib + testify assertions.**

## 11. Build System

**Makefile-driven with pinned tools:**

```makefile
VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse --short=12 HEAD)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X .../cmd.version=$(VERSION) -X .../cmd.commit=$(COMMIT) -X .../cmd.date=$(DATE)

build:
    go build -ldflags "$(LDFLAGS)" -o bin/gog ./cmd/gog
```

**Tool pinning:**
```makefile
TOOLS_DIR := .tools
TOOLS_VERSION := gofumpt=v0.9.2;goimports=v0.44.0;golangci-lint=v2.11.4

tools:
    @GOBIN=$(TOOLS_DIR) go install mvdan.cc/gofumpt@v0.9.2
    @GOBIN=$(TOOLS_DIR) go install golang.org/x/tools/cmd/goimports@v0.44.0
    @GOBIN=$(TOOLS_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
```

Tools are installed into `.tools/` (not `$GOPATH/bin`) with a stamp file for cache invalidation. `make tools` is idempotent.

**CI target:**
```makefile
ci: pnpm-gate fmt-check lint test
```

## 12. Release & Distribution

**GoReleaser** for cross-platform builds:

```yaml
builds:
  - id: gog
    main: ./cmd/gog
    ldflags: -s -w -X .../cmd.version={{ .Tag }} ...
    env: [CGO_ENABLED=0]
    targets: [linux_amd64, linux_arm64, windows_amd64, windows_arm64]

  - id: gog_darwin
    env: [CGO_ENABLED=1]  # macOS keychain needs cgo
    targets: [darwin_amd64, darwin_arm64]
```

Notable: macOS gets a separate build config because it needs `CGO_ENABLED=1` for the system keychain. Everything else is statically linked with `CGO_ENABLED=0`.

Distribution: GitHub releases + Homebrew tap (`steipete/homebrew-tap`).

## 13. Configuration

**JSON5 config** (allows comments) at `~/.config/gog/config.json`:

```json5
{
    "keyring_backend": "os",
    "default_timezone": "Europe/Vienna",
    "account_aliases": { "w": "work@company.com" },
    "gmail_no_send": false
}
```

- XDG_CONFIG_HOME aware
- File locking (`.config/gog/config.lock`) with PID-based ownership and 2s timeout
- Atomic writes: write to `.tmp`, then `os.Rename()`

**Environment variables** for everything that needs to be overridable in CI/scripts:
`GOG_COLOR`, `GOG_JSON`, `GOG_AUTO_JSON`, `GOG_ACCESS_TOKEN`, `GOG_KEYRING_BACKEND`, etc.

## 14. Secrets & Auth

- Primary: OS keychain via `99designs/keyring` (macOS Keychain, Linux Secret Service, Windows Credential Manager)
- Fallback: encrypted file keyring (for headless/CI)
- Keyring operations have timeouts to prevent hangs
- Refresh tokens stored per-account, auto-refreshed for access tokens
- Direct access token via `--access-token` or `GOG_ACCESS_TOKEN` (for CI, bypasses refresh flow)

## 15. Safety Patterns

**For agent/automation use:**
- `--dry-run` / `-n` -- preview without executing
- `--force` / `-y` -- skip confirmations (for scripts)
- `--no-input` -- fail instead of prompting (for CI)
- `--gmail-no-send` / `GOG_GMAIL_NO_SEND=1` -- block email sends
- `GOG_ENABLE_COMMANDS` / `GOG_DISABLE_COMMANDS` -- command allow/block lists
- **Baked safety profiles** -- compile-time YAML profiles that restrict which commands can run. Used when building a binary for AI agents.

## 16. Linting

**45+ linters** via golangci-lint, including:
- `gosec` (security), `errcheck`, `staticcheck`, `unused`
- Formatting: `gofumpt` + `goimports` (stricter than `go fmt`)
- Forbidden patterns: stdlib `log.` (must use `slog` or `ui`)
- Relaxed for tests: `errcheck`, `gosec`, `dupl` disabled

## 17. Dependency Philosophy

**Minimal and deliberate.** 12 direct dependencies:

| Dependency | Purpose |
|---|---|
| `alecthomas/kong` | CLI framework |
| `99designs/keyring` | OS credential storage |
| `muesli/termenv` | Terminal colors |
| `stretchr/testify` | Test assertions |
| `yosuke-furukawa/json5` | Config with comments |
| `filippo.io/age` | Encrypted backups |
| `google.golang.org/api` | Google APIs |
| `golang.org/x/oauth2` | OAuth2 |
| `golang.org/x/term` | Terminal detection |
| `golang.org/x/text` | Text processing |
| `golang.org/x/net` | Network utilities |
| `gopkg.in/yaml.v3` | Safety profiles |

No logging framework (uses stdlib `log/slog`). No HTTP client wrapper. No DI framework.

---

## Checklist: Starting a New Go CLI

Based on these patterns, here's what to set up for a new CLI project (e.g., pixx.io):

1. **Scaffold:**
   ```
   cmd/pixx/main.go          # 10-15 lines, delegates to internal/cmd
   internal/cmd/root.go      # CLI struct, Execute(), RootFlags
   internal/cmd/...           # One file per command
   go.mod
   Makefile
   ```

2. **Pick Kong** for the CLI framework. Define commands as structs.

3. **Set up three output modes** early: interactive (colored), JSON, plain.

4. **Route stdout vs stderr** strictly: data → stdout, messages → stderr.

5. **Use `context.Context`** for passing config, auth, UI -- not globals.

6. **Error handling:** define `ExitError` with codes. Exit 2 for usage errors. Format errors with actionable hints.

7. **Test with httptest** for API mocking, function variables for service injection, `t.Cleanup()` for teardown.

8. **Makefile:** `build` (with ldflags), `fmt`, `lint`, `test`, `ci`. Pin tool versions in `.tools/`.

9. **GoReleaser** for cross-platform distribution.

10. **Config:** JSON5 or YAML in XDG_CONFIG_HOME. Env var overrides for everything.

11. **Secrets:** `99designs/keyring` for credential storage. Never store tokens in plaintext config.

12. **Lint hard:** golangci-lint with 40+ linters. `gofumpt` > `go fmt`.
