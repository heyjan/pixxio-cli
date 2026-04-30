# Changelog

## v0.1.0 — 2026-04-30

Initial release of `pixx`, a CLI for the pixx.io Digital Asset Management API.

### Features

- **Files** — `pixx files list`, `get`, `upload`, `download`, `delete`
- **Collections** — `pixx collections list`
- **Directories** — `pixx directories list`
- **Authentication** — `pixx auth login` with Bearer token (config file, env vars, CLI flag)
- **Three output modes** — interactive (colored), `--json`, `--plain` (TSV)
- **CI-friendly** — `--api-key` flag, env vars, `--plain` for scripting
- **Dry-run & force** — `--dry-run` to preview, `--force` to skip confirmations
- **Actionable errors** — every error tells you what to do next
