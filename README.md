# cocoq

cocoq is a local HTTP MITM proxy for Claude/Anthropic traffic. It intercepts Anthropic-style `/v1/messages` requests, applies cache-related request rewrites, captures OpenRouter usage records, and logs proxy traffic with TLS ClientHello fingerprints when available.

## Features

- Forces every existing `cache_control: {"type":"ephemeral"}` block in `system` and `messages[].content[]` to use `ttl: "1h"`.
- Normalizes the Claude Code `# currentDate` system-reminder block to the current UTC date.
- Rejects Anthropic event logging endpoints.
- Supports Anthropic API hosts and OpenRouter Anthropic-compatible messages requests.
- Forces OpenRouter requests to the Anthropic provider with fallback disabled.
- Extracts request metadata from `metadata.user_id` JSON strings, including `device_id`, `account_uuid`, and `session_id`.
- Saves OpenRouter usage records to SQLite, including model, token counts, cache read/create counts, cache hit rate, raw usage JSON, and timestamps.
- Logs accepted proxy requests with shared request fields, including client-to-cocoq TLS ClientHello JA3 fields when MITM TLS fingerprinting is available.
- Can optionally write accepted proxy sessions to a HAR file.

## Usage

Run the proxy:

```sh
go run ./cmd/cocoq server run
```

Configuration is loaded from `~/.cocoq/config.yaml` when that file exists:

```yaml
global:
  root_dir: /Users/you/.cocoq
server:
  addr: 127.0.0.1:8888
  har_file: /tmp/cocoq.har
  verbose: false
  ca:
    cert_file: ca.crt
    key_file: ca.key
database:
  path: database.db
```

Use `--config <path>` before the command to load another config file:

```sh
go run ./cmd/cocoq --config /tmp/cocoq.yaml server run
```

The proxy creates its local CA under `global.root_dir`. Trust that CA in clients that need HTTPS MITM interception, then point the client at `127.0.0.1:8888` as its HTTP/HTTPS proxy.

By default, `global.root_dir` is `$HOME/.cocoq` and runtime data is stored in `$HOME/.cocoq/database.db`. Set `database.path` in the config file to use another database for `server run` and `db` commands. Absolute file paths are used directly; relative paths are resolved under `global.root_dir`.

## Usage Records

OpenRouter usage is saved from both streaming SSE responses and non-streaming JSON responses. Each `anthropic_usage` row includes:

- Identity fields: `device_id`, `session_id`, `account_uuid`, and `model`.
- Usage fields: input tokens, output tokens, cache read tokens, cache creation tokens, 5-minute and 1-hour cache creation tokens, and cache hit rate.
- Audit fields: raw usage JSON, created timestamp, and updated timestamp.

List saved usage records:

```sh
go run ./cmd/cocoq db anthropic-usage list
```

Get or delete a specific usage record:

```sh
go run ./cmd/cocoq db anthropic-usage get 1
go run ./cmd/cocoq db anthropic-usage delete 1
```

## Logging

Accepted proxy-domain requests are logged with method, URL, host, content headers, user agent, remote address, and related request fields.

When goproxy captures the client-to-cocoq TLS ClientHello during MITM, logs include:

- `client_tls_fingerprint`: JA3 hash.
- `client_tls_ja3_hash`: JA3 hash.
- `client_tls_ja3`: JA3 string.
- `client_tls_client_hello_raw_len`: raw ClientHello record length.
- `client_tls_client_hello_parsed`: whether the ClientHello parsed successfully.

Response logs also include upstream TLS response state when available, such as TLS version, server name, and negotiated ALPN protocol.

## Acknowledgements

Thanks to [`claude-code-cache-fix`](https://github.com/cnighswonger/claude-code-cache-fix) for the original cache-fix research.

## Development

```sh
go test ./...
go build ./cmd/cocoq
```

Regenerate database code after Ent schema changes:

```sh
go generate ./server/database
```
