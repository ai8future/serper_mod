# serper_mod

A production-grade Go client library and CLI for the [Serper.dev](https://serper.dev) Google Search API. Provides typed, validated access to web, image, news, places, and Google Scholar search results with built-in security hardening, retry support, and structured error handling.

## Features

- **Five search verticals** -- web, images, news, places, and Google Scholar via a single `SearchRequest` type
- **Functional options** -- configure base URL, HTTP transport, and timeouts at construction time
- **Dependency injection** -- the `Doer` interface decouples HTTP transport from business logic, enabling retry middleware, OpenTelemetry tracing, and offline testing
- **Per-request API key override** -- support multi-tenant usage from a single `Client` via context values
- **Request immutability** -- the caller's `*SearchRequest` is copied before defaults are applied; the original is never mutated
- **Typed errors** -- HTTP status codes map to `chassis-go` typed errors (`ValidationError`, `UnauthorizedError`, `RateLimitError`, etc.) carrying both HTTP and gRPC status codes
- **JSON security validation** -- every API response passes through `secval.ValidateJSON` before unmarshalling to block prototype pollution and injection attacks
- **Response size limits** -- 10 MB cap on response bodies; error messages truncated to 1 KB
- **Retry body replay** -- `GetBody` is set on every outgoing request so retry middleware can replay without re-serializing
- **Input validation** -- query, result count, and page number are validated before any network call

## Installation

### Library

```bash
go get github.com/ai8future/serper_mod
```

Then import the `serper` package:

```go
import "github.com/ai8future/serper_mod/serper"
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
serper golang concurrency patterns
```

The CLI joins all arguments into a single query and outputs pretty-printed JSON to stdout.

## API Reference

### Constructor

```go
func New(apiKey string, opts ...Option) (*Client, error)
```

Creates a new client. Returns an error if the API key is empty or the base URL is invalid. The default HTTP transport has a 30-second timeout.

**Options:**

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Override the default `https://google.serper.dev` endpoint |
| `WithDoer(d)` | Inject a custom HTTP executor (must satisfy the `Doer` interface) |

Options are applied in order. Last-option-wins for duplicate settings. URL validation runs after all options are applied.

### Search Methods

All methods accept a `context.Context` and a `*SearchRequest`. Defaults are applied and validation runs before any network call. The original request is never modified.

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
    Q        string `json:"q"`                    // Required. The search query.
    Num      int    `json:"num,omitempty"`         // Results per page (1-100, default: 10)
    GL       string `json:"gl,omitempty"`          // Country code (default: "us")
    HL       string `json:"hl,omitempty"`          // Language code (default: "en")
    Location string `json:"location,omitempty"`    // Free-text location filter (optional, omitted from JSON when empty)
    Page     int    `json:"page,omitempty"`        // Page number (default: 1)
}
```

**Defaults** (applied automatically via `SetDefaults`): `Num=10`, `GL="us"`, `HL="en"`, `Page=1`. Explicit non-zero values are preserved.

**Validation** (applied automatically via `Validate`): `Q` must be non-empty, `Num` must be in [1, 100], `Page` must be >= 1.

### Response Types

**SearchResponse** -- Web search results:
- `SearchParameters` -- echoed query parameters
- `KnowledgeGraph` (optional) -- knowledge panel with title, description, attributes
- `Organic` -- ranked list of `OrganicResult` (title, link, snippet, position, sitelinks)
- `PeopleAlsoAsk` -- related questions with snippets
- `RelatedSearches` -- suggested related queries

**ImagesResponse** -- Image results with `ImageResult` (title, imageUrl, thumbnailUrl, source, link, position)

**NewsResponse** -- News articles with `NewsResult` (title, link, snippet, source, date, imageUrl, position)

**PlacesResponse** -- Local business results with `PlaceResult` (title, address, lat/lng, rating, ratingCount, category, phone, website, hours, position)

**ScholarResponse** -- Academic papers with `ScholarResult` (title, link, snippet, publicationInfo, citedBy, authors, year, position)

### Per-Request API Key Override

For multi-tenant scenarios, override the client's default API key on individual requests via context:

```go
ctx := serper.WithAPIKey(context.Background(), "tenant-specific-key")
resp, err := client.Search(ctx, &serper.SearchRequest{Q: "query"})
```

Empty keys are ignored (the client's default key is used). When nested, the innermost `WithAPIKey` wins.

### Connectivity Check

```go
err := client.CheckConnectivity(ctx)
```

Sends a minimal search (`Q: "test", Num: 1`) to verify the API key and network connectivity. **Note:** this makes a billable API request.

### Doer Interface

```go
type Doer interface {
    Do(*http.Request) (*http.Response, error)
}
```

The single abstraction point for HTTP transport. Satisfied by:
- `*http.Client` -- default, no middleware
- `call.Client` -- chassis-go's retry + timeout + OpenTelemetry tracing wrapper (used by the CLI)
- Any custom mock or middleware chain

Inject via `WithDoer()` at construction time.

## Error Handling

HTTP errors from the Serper.dev API are mapped to typed `chassis-go` errors with both HTTP and gRPC status codes:

| HTTP Status | Error Type | gRPC Code |
|-------------|-----------|-----------|
| 400 | `ValidationError` | `INVALID_ARGUMENT` |
| 401 | `UnauthorizedError` | `UNAUTHENTICATED` |
| 404 | `NotFoundError` | `NOT_FOUND` |
| 429 | `RateLimitError` | `RESOURCE_EXHAUSTED` |
| 502, 503 | `DependencyError` | `UNAVAILABLE` |
| other | `InternalError` | `INTERNAL` |

Inspect typed errors with `errors.As`:

```go
import chassiserrors "github.com/ai8future/chassis-go/v5/errors"

var se *chassiserrors.ServiceError
if errors.As(err, &se) {
    fmt.Println(se.HTTPCode, se.GRPCCode)
}
```

Error response bodies longer than 1 KB are truncated with `...(truncated)` to prevent oversized error messages.

## CLI Configuration

The CLI is configured entirely through environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERPER_API_KEY` | **Yes** | -- | Serper.dev API key |
| `SERPER_BASE_URL` | No | `https://google.serper.dev` | API endpoint override |
| `SERPER_NUM` | No | `10` | Results per page (1-100) |
| `SERPER_GL` | No | `us` | Country code for geo-targeting |
| `SERPER_HL` | No | `en` | Language code for results |
| `SERPER_TIMEOUT` | No | `30s` | Per-attempt request timeout (Go duration string) |
| `LOG_LEVEL` | No | `error` | Log verbosity: `debug`, `info`, `warn`, `error` |

The CLI uses `call.Client` with 3 retries and 500ms initial backoff. Each retry respects the configured timeout independently.

## Project Structure

```
serper_mod/
├── serper/                  # Core library package
│   ├── types.go             # All data types, request defaults, and validation
│   ├── client.go            # Client constructor, 5 search methods, HTTP layer, error mapping
│   └── client_test.go       # 30+ offline unit tests using mock transport
├── cmd/serper/              # CLI binary
│   ├── main.go              # Entry point, env config, resilient HTTP client setup
│   └── main_test.go         # Config loading and version gate tests
├── go.mod                   # Module definition and dependencies
├── go.sum                   # Dependency checksums
├── VERSION                  # Current version
├── CHANGELOG.md             # Version history
└── AGENTS.md                # AI agent operating instructions
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  cmd/serper (CLI binary)                                │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Config (env vars) → call.Client (retry+timeout)  │  │
│  │  → serper.Client → JSON stdout                    │  │
│  └───────────────────────────────────────────────────┘  │
└────────────────────────┬────────────────────────────────┘
                         │ imports
┌────────────────────────▼────────────────────────────────┐
│  serper (library package)                               │
│                                                         │
│  New(apiKey, ...Option) → *Client                       │
│    │                                                    │
│    ├── Search / Images / News / Places / Scholar         │
│    │     │                                              │
│    │     ├── prepareRequest (copy → SetDefaults → Validate) │
│    │     └── doRequest                                  │
│    │           │                                        │
│    │           ├── json.Marshal → HTTP POST              │
│    │           ├── Doer.Do(req)  ◄── injectable         │
│    │           │     │                                  │
│    │           │     ├── *http.Client  (default)        │
│    │           │     ├── call.Client   (CLI injects)    │
│    │           │     └── mockDoer      (tests inject)   │
│    │           │                                        │
│    │           ├── io.LimitReader (10 MB cap)           │
│    │           ├── HTTP status → typed error mapping     │
│    │           ├── secval.ValidateJSON (security check)  │
│    │           └── json.Unmarshal → typed response       │
│    │                                                    │
│    ├── WithAPIKey(ctx, key) → per-request override      │
│    └── CheckConnectivity(ctx) → health check            │
└─────────────────────────────────────────────────────────┘
```

**Key design decisions:**

- **Library has no CLI dependencies.** The `serper` package only imports `chassis-go/v5/errors` and `chassis-go/v5/secval` (both pure functions, no I/O). Config loading, logging, and retry logic are CLI-only concerns.
- **Single `Doer` interface** is the sole abstraction point between the library and the network, keeping the seam narrow and testable.
- **Copy-before-mutate** in `prepareRequest` ensures callers can safely reuse `SearchRequest` structs across calls.
- **`GetBody` is always set** on outgoing requests to enable retry middleware to replay the body without re-serialization.
- **Version gate** (`chassis.RequireMajor(5)`) at CLI startup prevents silent ABI mismatches if the chassis-go major version changes.

## Testing

All tests are offline unit tests using mock HTTP transports. No network calls are made.

```bash
go test ./...
```

**Coverage areas:**
- Constructor validation (empty key, invalid URL, option ordering)
- Request defaults and immutability
- Input validation (table-driven, boundary values)
- All 5 search endpoints (web, images, news, places, scholar)
- HTTP error mapping (401, 500, 503)
- Network errors (doer failure, context cancellation)
- JSON security validation (malformed response detection)
- Error body truncation
- Per-request API key override (empty, nested, override)
- Retry body replay (`GetBody` verification)
- HTTP method verification (POST)
- Location field serialization (omit-when-empty)
- CLI config defaults and required-field panics

## Dependencies

Built on [chassis-go/v5](https://github.com/ai8future/chassis-go), an internal Go framework:

| Package | Used By | Purpose |
|---------|---------|---------|
| `chassis` | CLI | Version gate (`RequireMajor(5)`) |
| `chassis-go/v5/errors` | Library | Typed error constructors with HTTP + gRPC codes |
| `chassis-go/v5/secval` | Library | JSON response security validation |
| `chassis-go/v5/call` | CLI | Resilient HTTP client (retry + timeout + OTel tracing) |
| `chassis-go/v5/config` | CLI | Struct-tag-based environment variable config loader |
| `chassis-go/v5/logz` | CLI | Structured JSON logger |
| `chassis-go/v5/testkit` | CLI tests | Test environment helpers |

Transitive dependencies include OpenTelemetry (`go.opentelemetry.io/otel`), gRPC status codes (`google.golang.org/grpc`), and standard Go libraries.

## Local Development

The project uses `go.work` for local development against a sibling `chassis-go` checkout:

```bash
# Build all packages
go build ./...

# Run all tests
go test ./...

# Build the CLI binary
go build -o serper ./cmd/serper

# Run with verbose test output
go test -v ./...
```

## License

Private. All rights reserved.
