# serper_mod

A production-grade Go client library and CLI for the [Serper.dev](https://serper.dev) Google Search API. Provides structured access to web, image, news, places, and scholar search results.

## Features

- **Five search verticals** -- web, images, news, places, and Google Scholar
- **Functional options** -- configure base URL, HTTP transport, and timeouts at construction
- **Dependency injection** -- the `Doer` interface decouples HTTP transport from business logic, enabling retry middleware, OpenTelemetry tracing, and test mocks
- **Per-request API key override** -- multi-tenant usage from a single client via context
- **Request immutability** -- caller's `SearchRequest` is never mutated
- **Typed errors** -- HTTP status codes map to `chassis-go` typed errors (`ValidationError`, `UnauthorizedError`, `RateLimitError`, etc.)
- **Response security validation** -- all API responses pass through `secval.ValidateJSON` to block prototype pollution and injection attacks
- **Response size limits** -- 10 MB cap on response bodies, 1 KB truncation on error messages
- **Retry with backoff** -- the CLI uses `call.Client` with 3 retries and exponential backoff

## Installation

### Library

```bash
go get github.com/ai8future/serper_mod
```

### CLI

```bash
go install github.com/ai8future/serper_mod/cmd/serper@latest
```

## Quick Start

### As a Library

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/ai8future/serper_mod/serper"
)

func main() {
    client, err := serper.New("your-api-key")
    if err != nil {
        log.Fatal(err)
    }

    resp, err := client.Search(context.Background(), &serper.SearchRequest{
        Q:   "golang concurrency patterns",
        Num: 5,
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, r := range resp.Organic {
        fmt.Printf("%d. %s\n   %s\n\n", r.Position, r.Title, r.Link)
    }
}
```

### As a CLI

```bash
export SERPER_API_KEY="your-api-key"
serper "golang concurrency patterns"
```

The CLI outputs pretty-printed JSON to stdout.

## API Reference

### Constructor

```go
func New(apiKey string, opts ...Option) (*Client, error)
```

Returns an error if the API key is empty or the base URL is invalid.

**Options:**

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Override the default `https://google.serper.dev` endpoint |
| `WithDoer(d)` | Inject a custom HTTP executor (must satisfy the `Doer` interface) |

### Search Methods

All methods accept a `context.Context` and a `*SearchRequest`, returning a typed response:

| Method | Endpoint | Response Type |
|--------|----------|---------------|
| `Search(ctx, req)` | `/search` | `*SearchResponse` |
| `Images(ctx, req)` | `/images` | `*ImagesResponse` |
| `News(ctx, req)` | `/news` | `*NewsResponse` |
| `Places(ctx, req)` | `/places` | `*PlacesResponse` |
| `Scholar(ctx, req)` | `/scholar` | `*ScholarResponse` |

### SearchRequest

```go
type SearchRequest struct {
    Q        string // Required. The search query.
    Num      int    // Results per page (1-100, default: 10)
    GL       string // Country code (default: "us")
    HL       string // Language code (default: "en")
    Location string // Free-text location filter (optional)
    Page     int    // Page number (default: 1)
}
```

Defaults are applied automatically before each request. Validation enforces that `Q` is non-empty, `Num` is in [1, 100], and `Page >= 1`.

### Per-Request API Key Override

For multi-tenant scenarios, override the API key on a per-request basis:

```go
ctx := serper.WithAPIKey(context.Background(), "tenant-specific-key")
resp, err := client.Search(ctx, &serper.SearchRequest{Q: "query"})
```

### Connectivity Check

```go
err := client.CheckConnectivity(ctx)
```

Sends a minimal live search to verify the API key works. Note: this makes a billable API request.

### Doer Interface

```go
type Doer interface {
    Do(*http.Request) (*http.Response, error)
}
```

Satisfied by `*http.Client`, `call.Client` (retry + tracing), or any test mock. Inject via `WithDoer()`.

## Error Handling

HTTP errors are mapped to typed `chassis-go` errors:

| HTTP Status | Error Type | gRPC Code |
|-------------|-----------|-----------|
| 400 | `ValidationError` | `INVALID_ARGUMENT` |
| 401 | `UnauthorizedError` | `UNAUTHENTICATED` |
| 404 | `NotFoundError` | `NOT_FOUND` |
| 429 | `RateLimitError` | `RESOURCE_EXHAUSTED` |
| 502, 503 | `DependencyError` | `UNAVAILABLE` |
| other | `InternalError` | `INTERNAL` |

Use `errors.As` to inspect:

```go
import chassiserrors "github.com/ai8future/chassis-go/v5/errors"

var se *chassiserrors.ServiceError
if errors.As(err, &se) {
    fmt.Println(se.HTTPCode, se.GRPCCode)
}
```

## CLI Configuration

The CLI is configured entirely through environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERPER_API_KEY` | Yes | -- | Serper.dev API key |
| `SERPER_BASE_URL` | No | `https://google.serper.dev` | API endpoint |
| `SERPER_NUM` | No | `10` | Results per page |
| `SERPER_GL` | No | `us` | Country code |
| `SERPER_HL` | No | `en` | Language code |
| `SERPER_TIMEOUT` | No | `30s` | Request timeout (Go duration) |
| `LOG_LEVEL` | No | `error` | Log verbosity: debug, info, warn, error |

## Project Structure

```
serper_mod/
├── serper/
│   ├── types.go        # Data types, request validation, and defaults
│   ├── client.go       # API client with 5 search methods
│   └── client_test.go  # 30+ unit tests (offline, using mock transport)
├── cmd/serper/
│   ├── main.go         # CLI entry point
│   └── main_test.go    # Config loading tests
├── go.mod
├── VERSION
└── CHANGELOG.md
```

## Architecture

```
cmd/serper (CLI binary)
│
├── serper.Client (library)
│   ├── Doer interface (injected HTTP transport)
│   │   ├── call.Client  ← CLI injects this (retry + OTel tracing)
│   │   ├── http.Client  ← default (no middleware)
│   │   └── mockDoer     ← tests inject this
│   │
│   ├── chassiserrors    ← HTTP status → typed error mapping
│   └── secval           ← JSON security validation on all responses
│
└── serper.SearchRequest / *Response types
```

## Testing

All tests are offline unit tests using a mock HTTP transport:

```bash
go test ./...
```

## Dependencies

Built on [chassis-go/v5](https://github.com/ai8future/chassis-go) which provides:

| Package | Purpose |
|---------|---------|
| `chassis` | Version gate (`RequireMajor(5)`) |
| `chassis-go/v5/errors` | Typed error constructors |
| `chassis-go/v5/secval` | JSON response security validation |
| `chassis-go/v5/call` | Resilient HTTP client (retry + OTel tracing) |
| `chassis-go/v5/config` | Environment-variable config loader |
| `chassis-go/v5/logz` | Structured JSON logger |
| `chassis-go/v5/testkit` | Test utilities |

## License

Private. All rights reserved.
