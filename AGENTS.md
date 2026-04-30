# Repository Guidelines

## Project Structure & Module Organization

This is a Go module named `cocoq`. The CLI entry point is `cmd/cocoq/main.go` and uses Cobra to expose proxy server commands. Runtime proxy code lives in `server/`: Anthropic host filtering and request handling in `server/anthropic.go`, MITM CA and CONNECT handling in `server/mitm.go`, request/response logging in `server/logging.go`, and server construction/lifecycle in `server/server.go`. The `claude-code-cache-fix/` directory is a Git submodule; do not edit it as ordinary vendored source from this repository.

## Build, Test, and Development Commands

- `go test ./...`: run the full test suite.
- `go build ./cmd/cocoq`: build the CLI binary.
- `go run ./cmd/cocoq server run --addr 127.0.0.1:8888`: run the local HTTP proxy with the default listen address.
- `go run ./cmd/cocoq server run --har-file /tmp/cocoq.har`: write accepted proxy sessions as HAR entries.
- `go mod tidy`: update module metadata after dependency changes.
- `git submodule update --init --recursive`: initialize or refresh submodules after cloning.

## Coding Style & Naming Conventions

Use standard Go formatting: run `gofmt` or `go fmt ./...` before submitting changes. Keep package names short and lowercase. Exported identifiers should be clear and documented when they are part of a package API. Prefer small, direct helpers that match the existing server package style over new abstractions. Keep generated binaries and local run artifacts out of the repo.

## Testing Guidelines

The project uses Go's built-in `testing` package. Place tests beside the implementation and name files `*_test.go`. Prefer temporary paths from `t.TempDir()` for HAR output, CA files, and other artifacts so tests do not touch `~/.cocoq`. Cover command parsing, host/path filtering, MITM setup, logging fields, and server lifecycle behavior when changing those areas.

## Commit & Pull Request Guidelines

Use concise imperative commit subjects such as `Update proxy logging` or `Add claude-code-cache-fix submodule`. Every commit created with Codex assistance must include this trailer exactly:

`Assisted-by: Codex <codex@openai.com>`

Pull requests should summarize behavior changes, call out affected commands or proxy flows, note submodule pointer changes when relevant, link related issues, and include relevant `go test ./...` output.

## Security & Configuration Tips

Generated CA files live under `~/.cocoq`. Do not commit HAR captures, private keys, local credentials, or local `.env` files. Treat captured proxy traffic as sensitive. Keep submodule updates pinned to intentional commits and review `.gitmodules` changes carefully.
