# Repository Guidelines

## Project Structure & Module Organization

`cocoq` is a Go module for a local HTTP MITM proxy around Anthropic-compatible Claude traffic. The CLI entry point is `cmd/cocoq/main.go`, with Cobra command implementations under `cmd/cocoq/commands/`. Runtime server setup and request logging live in `server/`. Anthropic-compatible service handling is under `server/anthropic/`, MITM proxy primitives under `server/proxy/`, SSE forwarding under `server/sse/`, configuration under `config/`, and SQLite/Ent persistence under `server/database/`. Generated Ent runtime code lives in `server/database/dbrt/`; update it through generation rather than hand edits.

## Build, Test, and Development Commands

- `go test ./...`: run the full Go test suite.
- `go build ./cmd/cocoq`: build the CLI binary.
- `go run ./cmd/cocoq server run --addr 127.0.0.1:8888`: run the proxy locally.
- `go run ./cmd/cocoq server run --har-file /tmp/cocoq.har`: run the proxy and write accepted sessions to a HAR file.
- `go run ./cmd/cocoq default-config`: print the commented default config.
- `go generate ./server/database`: regenerate Ent code after schema changes.
- `go mod tidy`: update module metadata after dependency changes.

## Coding Style & Naming Conventions

Use standard Go formatting with `gofmt` or `go fmt ./...`. Keep package names short, lowercase, and aligned with directory names. Prefer direct helpers that match the existing server and proxy packages over broad abstractions. Exported identifiers should be clear and documented when they form package API. Keep generated binaries, HAR files, local databases, and other run artifacts out of the repository.

## Testing Guidelines

Tests use Go's built-in `testing` package and live beside implementation files as `*_test.go`. Use `t.TempDir()` for temporary CA files, HAR output, databases, and config files so tests do not touch `~/.cocoq`. Cover command parsing, config resolution, service filtering, MITM behavior, SSE forwarding, logging fields, usage extraction, and database behavior when changing those areas.

## Commit & Pull Request Guidelines

Use concise imperative commit subjects, such as `Update proxy logging` or `Add Poe API service config`. Codex-assisted commits must use this trailer:

`Co-authored-by: Codex <codex@openai.com>`

Pull requests should summarize behavior changes, call out affected commands or proxy flows, link related issues, and include relevant `go test ./...` output. Note generated database code changes when Ent schemas are touched.

## Security & Configuration Tips

Generated CA files and the default SQLite database live under `~/.cocoq`. Do not commit HAR captures, private keys, credentials, local `.env` files, or real proxy traffic. Treat logs and captured requests as sensitive.
