# cocoq

cocoq is a local HTTP MITM proxy for Claude traffic through OpenRouter. It currently proxies OpenRouter Anthropic-compatible `/api/v1/messages` requests, applies some prompt caching features, and rejects Anthropic event logging endpoints.

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

Install `cocoq` first:

```sh
go install github.com/realityone/cocoq/cmd/cocoq@main
```

Run the proxy:

```sh
cocoq server run
```

Configuration is loaded from `~/.cocoq/config.yaml` when that file exists:

Print a commented default config:

```sh
cocoq default-config
```

```yaml
global:
  root_dir: /Users/you/.cocoq
server:
  addr: 127.0.0.1:8888
  api_services:
    - name: openrouter
      options:
        provider: anthropic
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
cocoq --config /tmp/cocoq.yaml server run
```

Run Claude Code with proxy environment variables resolved from config:

```sh
cocoq claude
```

The `claude` command sets `HTTP_PROXY` from `server.addr` and `NODE_EXTRA_CA_CERTS` from `server.ca.cert_file`, then replaces the current process with `claude`. The CA certificate file must already exist.

The proxy creates its local CA under `global.root_dir`. Trust that CA in clients that need HTTPS MITM interception, then point the client at `127.0.0.1:8888` as its HTTP/HTTPS proxy.

By default, `global.root_dir` is `$HOME/.cocoq` and runtime data is stored in `$HOME/.cocoq/database.db`. Set `database.path` in the config file to use another database for `server run` and `db` commands. Absolute file paths are used directly; relative paths are resolved under `global.root_dir`.

Set `server.api_services` to choose which Anthropic-compatible API service implementations the proxy installs. Supported service names are `openrouter` and `anthropic`; the default installs `openrouter`. Service-specific settings live under `options`; OpenRouter supports `options.provider` to force a provider.

## Usage Records

OpenRouter usage is saved from both streaming SSE responses and non-streaming JSON responses. Each `anthropic_usage` row includes:

- Identity fields: `device_id`, `session_id`, `account_uuid`, and `model`.
- Usage fields: input tokens, output tokens, cache read tokens, cache creation tokens, 5-minute and 1-hour cache creation tokens, and cache hit rate.
- Audit fields: raw usage JSON, created timestamp, and updated timestamp.

List saved usage records:

```sh
cocoq db anthropic-usage list
```

Get or delete a specific usage record:

```sh
cocoq db anthropic-usage get 1
cocoq db anthropic-usage delete 1
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
