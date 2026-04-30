# 📦 pixx — pixx.io CLI

> Manage your [pixx.io](https://www.pixx.io) digital assets from the terminal.

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![pixx.io API](https://img.shields.io/badge/pixx.io_API-v1-7FCBC6)](https://www.pixx.io)

---

## ✨ Features

- 📁 **Files** — list, get, upload, download, and delete assets
- 🗂️ **Collections** — browse and manage collections
- 📂 **Directories** — browse folder structure
- 🔐 **Bearer Auth** — API key authentication (config file + env + CLI flag)
- 📊 **Three output modes** — interactive (colored), `--json`, `--plain` (TSV)
- 🔧 **CI-friendly** — `--api-key` flag, env vars, `--plain` for scripting
- ⚡ **Minimal dependencies** — 2 direct deps (Kong, termenv)

## 🚀 Quick Start

### Install

```bash
go install github.com/heyjan/pixxio-cli/cmd/pixx@latest
```

Or build from source:

```bash
git clone https://github.com/heyjan/pixxio-cli.git
cd pixxio-cli
make build
# Binary at ./bin/pixx
```

### Authenticate

Generate an API key in your pixx.io mediaspace: **Settings → API → API-Key → Generate**

```bash
pixx --api-key YOUR_API_KEY auth login --mediaspace demo.pixx.io
```

Or use environment variables:

```bash
export PIXX_MEDIASPACE=demo.pixx.io
export PIXX_API_KEY=your-api-key
pixx files list
```

### Usage

```bash
# List files
pixx files list
pixx files list --json
pixx files list --plain -l 50

# Get file details
pixx files get 42
pixx files get 42 --json

# Download a file
pixx files download 42
pixx files download 42 -o photo.jpg --type preview

# Upload a file
pixx files upload ./photo.jpg
pixx files upload ./photo.jpg -d 5 -s "Brand Photo" --description "Q4 campaign"

# Delete files (requires --force)
pixx files delete 42 43 44 --force
pixx files delete 42 --dry-run

# List collections
pixx collections list
pixx collections list --json

# Browse directories
pixx directories list
pixx directories list -s "Photos"

# Version
pixx version
```

## 🔧 Configuration

Config is stored at `~/.config/pixx/config.json` (XDG_CONFIG_HOME aware):

```json
{
  "mediaspace": "demo.pixx.io",
  "apiKey": "your-api-key"
}
```

### Environment Variables

| Variable | Description |
|---|---|
| `PIXX_MEDIASPACE` | Mediaspace domain |
| `PIXX_API_KEY` | API key (Bearer token) |
| `PIXX_COLOR` | Color output: `auto`\|`always`\|`never` |
| `PIXX_JSON` | Set to `1` for JSON output |

### Global Flags

| Flag | Short | Description |
|---|---|---|
| `--api-key` | | API key (overrides config, for CI) |
| `--json` | `-j` | Output JSON |
| `--plain` | `-p` | Output stable TSV |
| `--verbose` | `-v` | Enable debug logging |
| `--dry-run` | `-n` | Preview changes |
| `--force` | `-y` | Skip confirmations |
| `--color` | | `auto`\|`always`\|`never` |

## 🏗️ Architecture

Built following the [steipete Go CLI methodology](steipete-go-cli-methodology.md):

- **[Kong](https://github.com/alecthomas/kong)** — struct-based CLI framework (not Cobra)
- **`context.Context`** — carries config, UI, output mode (no globals)
- **Three output modes** — interactive / JSON / plain, strict stdout/stderr separation
- **`ExitError`** with codes — 0 ok, 1 error, 2 usage
- **Actionable error messages** — every error tells you what to do next
- **Function variables** for testability — API client injection via `t.Cleanup()`
- **httptest** for API mocking — no third-party mock libraries

### Project Structure

```
cmd/pixx/main.go              # Entry point (13 lines, delegates to internal/cmd)
internal/
  cmd/
    root.go                   # CLI struct, RootFlags, Execute()
    exit.go                   # ExitError type, exit codes
    auth.go                   # pixx auth login
    files.go                  # pixx files {list,get,download,upload,delete}
    collections.go            # pixx collections list
    directories.go            # pixx directories list
    *_test.go                 # Command tests (function variable swapping)
  pixxapi/
    client.go                 # API client (Bearer auth, all endpoints)
    client_test.go            # httptest-based API tests
  config/config.go            # XDG config + env overrides
  outfmt/outfmt.go            # Three output modes (interactive/JSON/plain)
  ui/ui.go                    # Colored terminal output, stdout/stderr separation
  errfmt/errfmt.go            # Actionable error formatting
Makefile                      # build, fmt, lint, test, ci
.golangci.yml                 # Linter config
openapi.json                  # pixx.io API v1 OpenAPI spec
```

## 🧪 Testing

```bash
make test          # Run all tests
make ci            # fmt-check + lint + test
```

Tests use `net/http/httptest` for API mocking and function variable swapping for command-level tests. No external test frameworks beyond stdlib.

## 📋 API Coverage

| Endpoint | CLI Command | Method |
|---|---|---|
| `/api/v1/files` | `files list` | GET |
| `/api/v1/files/{id}` | `files get` | GET |
| `/api/v1/files` | `files upload` | POST |
| `/api/v1/files` | `files delete` | DELETE |
| `/api/v1/files/{id}/convert` | `files download` | GET |
| `/api/v1/collections` | `collections list` | GET |
| `/api/v1/directories` | `directories list` | GET |

The pixx.io API has 61 endpoints — contributions welcome for external shares, keywords, licenses, and more.

## 📦 Dependencies

| Dependency | Purpose |
|---|---|
| [`alecthomas/kong`](https://github.com/alecthomas/kong) | CLI framework |
| [`muesli/termenv`](https://github.com/muesli/termenv) | Terminal colors |

Everything else is Go stdlib (`net/http`, `encoding/json`, `log/slog`, `mime/multipart`).

## 📄 License

MIT
