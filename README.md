# cocoq

cocoq is a local HTTP MITM proxy for improving Claude/Anthropic prompt-cache behavior. It intercepts Anthropic-style `/v1/messages` requests and applies small request rewrites before forwarding them upstream.

## Anthropic Cache Enhancements

- Forces every existing `cache_control: {"type":"ephemeral"}` block in `system` and `messages[].content[]` to use `ttl: "1h"`.
- Normalizes the Claude Code `# currentDate` system-reminder block to the current UTC date.
- Rejects Anthropic event logging endpoints.
- Supports Anthropic API hosts and OpenRouter Anthropic-compatible messages requests.
- Can optionally write accepted proxy sessions to a HAR file.

## Usage

Run the proxy:

```sh
go run ./cmd/cocoq server run --addr 127.0.0.1:8888
```

Optional HAR logging:

```sh
go run ./cmd/cocoq server run --addr 127.0.0.1:8888 --har-file /tmp/cocoq.har
```

The proxy creates its local CA under `~/.cocoq/`. Trust that CA in clients that need HTTPS MITM interception, then point the client at `127.0.0.1:8888` as its HTTP/HTTPS proxy.

## Development

```sh
go test ./...
go build ./cmd/cocoq
```
