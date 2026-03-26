# Product Overview: serper_mod

## What This Product Is

serper_mod is a Go-based client SDK and command-line tool that provides programmatic access to Google Search results through the Serper.dev API. It acts as a managed intermediary layer between applications (or human operators) and Google's search infrastructure, abstracting away the raw HTTP/JSON mechanics of the Serper.dev service into a type-safe, production-hardened Go library.

The product ships in two forms:

1. **A reusable Go library** (`serper` package) that any Go application can import to run Google searches programmatically.
2. **A CLI binary** (`cmd/serper`) that enables command-line search queries with JSON output, suitable for scripting, automation pipelines, and quick ad-hoc lookups.

---

## Why This Product Exists

### The Core Business Problem

Google does not offer a simple, affordable, developer-friendly API for accessing its search results. Serper.dev fills that gap as a third-party SERP (Search Engine Results Page) API provider, but Serper.dev itself is just a raw REST API that returns JSON. Every consumer of that API must independently solve the same set of engineering problems:

- Constructing correctly-formed HTTP requests with authentication headers
- Parsing and validating the different response shapes for different search verticals
- Handling rate limits, transient failures, and authentication errors
- Defending against malformed or malicious response payloads
- Supporting multi-tenant use cases where different API keys are used per request
- Configuring retries, timeouts, and observability

serper_mod exists to solve all of these problems once, in a single reusable module, so that every Go-based application in the organization that needs Google search data can use a battle-tested, validated client rather than reimplementing the same integration from scratch.

### Strategic Context

This module lives within a broader `serp_suite` collection and depends on `chassis-go`, an internal Go framework (currently at v10). This indicates that serper_mod is part of a larger platform or product ecosystem where:

- Multiple services or applications need search capabilities
- There is a shared infrastructure layer (`chassis-go`) providing cross-cutting concerns like error handling, retry logic, configuration management, structured logging, and observability (OpenTelemetry)
- Consistency and reliability across services is a priority -- every search integration should behave the same way in terms of error handling, retry behavior, and security validation

---

## What the Product Does: Functional Capabilities

### Seven Search Verticals

The product provides access to seven distinct Google search verticals through a single unified request model:

| Vertical | Method | Business Use Case |
|----------|--------|-------------------|
| **Web Search** | `Search()` | General-purpose Google web search results -- organic listings, knowledge graphs, "People Also Ask" questions, and related search suggestions |
| **Image Search** | `Images()` | Finding images by query -- returns image URLs, thumbnails, source sites, and attribution links |
| **News Search** | `News()` | Current events and journalism -- returns articles with publication source, date, snippet, and thumbnail |
| **Places Search** | `Places()` | Local business and location discovery -- returns business name, address, GPS coordinates, ratings, reviews count, category, phone number, website, and operating hours |
| **Scholar Search** | `Scholar()` | Academic and research paper discovery -- returns paper titles, publication info, citation counts, author lists, and publication years |
| **Shopping Search** | `Shopping()` | E-commerce product discovery -- returns product listings with price, source retailer, ratings, review counts, delivery information, and product images |
| **Video Search** | `Videos()` | Video content discovery -- returns video titles, source platforms, channels, durations, publication dates, and thumbnails |

Each vertical returns a strongly typed response structure, meaning consumers get compile-time guarantees about the shape of the data they are working with rather than dealing with raw `map[string]interface{}` JSON.

### Unified Search Request Model

All seven verticals accept the same `SearchRequest` structure, which supports:

- **Query** (`Q`): The search terms (required)
- **Results per page** (`Num`): 1 to 100 results, defaulting to 10
- **Country targeting** (`GL`): ISO country code for geo-targeted results, defaulting to "us"
- **Language targeting** (`HL`): Language code for result language, defaulting to "en"
- **Location** (`Location`): Free-text location filter for hyper-local results
- **Pagination** (`Page`): Page number for navigating through large result sets

This unified model means a consumer can switch between search verticals with minimal code changes -- the same request can be routed to web, images, news, or any other endpoint.

### Multi-Tenant API Key Support

The product supports a multi-tenant model where a single `Client` instance can serve requests for different API key holders. This is achieved through Go context values:

- A default API key is set at client construction time
- Individual requests can override that key by attaching a different key to the request context via `WithAPIKey(ctx, key)`
- Empty overrides are silently ignored (the default key is used)
- When multiple overrides are nested, the innermost one wins

This design enables SaaS platforms or multi-user applications to share a single client instance while routing API billing to different Serper.dev accounts per request.

### Connectivity Verification

The `CheckConnectivity()` method sends a minimal real search request (`Q: "test", Num: 1`) to verify that the API key is valid and the Serper.dev service is reachable. This is a health-check mechanism that enables:

- Application startup validation (fail fast if the search API is misconfigured)
- Monitoring and alerting integrations
- Pre-flight checks before executing expensive batch search operations

Important: this is a billable request, which is documented as a product decision -- the only reliable way to verify connectivity to a metered API is to actually use it.

---

## Business Logic and Safety Rules

### Input Validation

Before any network call is made, every request passes through validation:

- **Query must not be empty**: Prevents wasted API calls on nonsensical requests
- **Results count must be between 1 and 100**: Enforces Serper.dev's API limits at the client side, avoiding 400 errors
- **Page number must be 1 or greater**: Prevents invalid pagination

This "validate early" approach serves a cost-control function -- every API call to Serper.dev is metered and billed, so catching bad requests before they hit the wire saves money.

### Smart Defaults

When a consumer provides only a query and leaves other fields at their zero values, the library automatically fills in sensible defaults:

- 10 results per page
- US country targeting
- English language
- Page 1

This reduces the effort required for the common case while still allowing full customization.

### Request Immutability

The caller's `SearchRequest` struct is copied before defaults and validation are applied. The original struct is never modified. This is a deliberate design choice that enables:

- Safe reuse of request objects across multiple calls
- Concurrent use of the same request from multiple goroutines
- Predictable behavior in loops and batch operations

### Response Security Validation

Every JSON response from Serper.dev passes through `secval.ValidateJSON` before being deserialized. This chassis-go security module checks for:

- Prototype pollution patterns (e.g., `__proto__` keys that could exploit downstream JavaScript consumers)
- JSON injection attacks
- Malformed JSON that could cause parser confusion

This is a defense-in-depth measure -- even though Serper.dev is a trusted third-party, the product treats all external data as potentially hostile. This is especially important if search results are later rendered in web UIs or passed to other systems.

### Response Size Limits

Response bodies are capped at 10 MB via `io.LimitReader`. This prevents:

- Memory exhaustion from abnormally large responses
- Denial-of-service conditions if the upstream API misbehaves
- Runaway memory allocation in production services

### Error Body Truncation

When the API returns an error (HTTP 4xx/5xx), the error body included in the error message is truncated to 1 KB. This prevents:

- Oversized error messages flooding logs
- Sensitive data from error responses leaking into log aggregation systems
- Memory pressure from storing large error payloads

### Typed Error Mapping

HTTP error codes from the Serper.dev API are mapped to semantically meaningful error types from the chassis-go framework, each carrying both an HTTP status code and a gRPC status code:

| Scenario | HTTP Code | Error Type | gRPC Code | Business Meaning |
|----------|-----------|------------|-----------|------------------|
| Bad request | 400 | `ValidationError` | `INVALID_ARGUMENT` | The search request was malformed |
| Invalid API key | 401 | `UnauthorizedError` | `UNAUTHENTICATED` | The API key is wrong or expired -- billing/account issue |
| Endpoint not found | 404 | `NotFoundError` | `NOT_FOUND` | Requested a non-existent search vertical |
| Rate limited | 429 | `RateLimitError` | `RESOURCE_EXHAUSTED` | Usage quota exceeded -- need to slow down or upgrade the Serper.dev plan |
| Upstream down | 502/503 | `DependencyError` | `UNAVAILABLE` | Serper.dev or Google is having issues -- retry may help |
| Other failures | any other | `InternalError` | `INTERNAL` | Unexpected failure |

The dual HTTP/gRPC code design suggests this module is used in systems that may expose search results through both REST and gRPC APIs, or that use gRPC status codes as an internal error taxonomy.

### Retry Support

The library sets `GetBody` on every outgoing HTTP request, which allows retry middleware (like chassis-go's `call.Client`) to replay failed requests without needing to re-serialize the JSON body. The CLI configures 3 retries with 500ms initial backoff by default.

This is critical for production reliability -- transient network failures, 502/503 errors, and brief rate limit windows can all be recovered from automatically without human intervention.

---

## CLI Product Behavior

The CLI binary provides a focused, single-purpose interface:

1. **Input**: All command-line arguments are joined into a single search query (e.g., `serper golang concurrency patterns`)
2. **Output**: Pretty-printed JSON written to stdout
3. **Configuration**: Entirely through environment variables (no config files, no flags) -- suitable for containerized deployments, CI/CD pipelines, and shell scripts
4. **Error handling**: Errors go to stderr with non-zero exit codes
5. **Resilience**: Uses chassis-go's `call.Client` with 3 retries and 500ms exponential backoff per attempt, with configurable per-attempt timeouts
6. **Observability**: Structured JSON logging via `logz`, with configurable log levels, and OpenTelemetry tracing support via the call client
7. **Version gating**: Verifies at startup that the chassis-go major version matches expectations, preventing silent ABI breakage when dependencies are upgraded
8. **Registry integration**: Registers itself as a CLI tool with the chassis framework's registry system at startup and performs a clean shutdown

---

## Target Users and Use Cases

### As a Library

- **Search-powered applications**: Any Go service that needs to enrich its data with Google search results (e.g., SEO tools, content research platforms, competitive intelligence dashboards, lead generation systems)
- **AI and LLM pipelines**: Providing real-time web search grounding for language model responses (RAG -- Retrieval-Augmented Generation)
- **Data enrichment services**: Looking up business information via Places, academic citations via Scholar, product pricing via Shopping, or news coverage via News
- **Multi-tenant SaaS platforms**: Applications serving multiple customers who each bring their own Serper.dev API keys

### As a CLI

- **Developer tooling**: Quick searches from the terminal during development
- **Automation scripts**: Shell scripts that need search results as structured JSON
- **CI/CD pipelines**: Automated testing or monitoring that depends on search results
- **Data collection pipelines**: Scheduled batch jobs that collect search data

---

## Cost and Billing Awareness

The product is built with an acute awareness that every API call to Serper.dev costs money:

- Input validation prevents wasted calls on invalid requests
- The connectivity check is documented as billable
- Rate limit errors are surfaced as a distinct error type so consumers can implement backoff or plan upgrades
- Retry logic uses exponential backoff to avoid hammering the API during outages
- The `Num` parameter is bounded to [1, 100] to prevent accidentally requesting excessive results

---

## Maturity and Evolution

The product is at version 1.8.4 and has gone through significant hardening over its lifecycle:

- **v1.0.0**: Initial release with basic search functionality
- **v1.1.0**: Input validation hardening, timeout defaults, response size limits, error truncation
- **v1.2.0**: Constructor validation, request immutability, comprehensive test coverage (30+ offline tests)
- **v1.4.0 through v1.8.4**: Progressive chassis-go framework upgrades (v4 through v10), keeping the module aligned with the evolving internal platform
- **v1.7.0**: Added Shopping and Videos search verticals, expanding from 5 to 7 search types

The changelog shows a pattern of continuous security and reliability improvements alongside framework version tracking, indicating this is an actively maintained infrastructure component rather than a one-off integration.
