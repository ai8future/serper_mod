Date Created: Monday, February 17, 2026 at 7:00:00 PM PST
TOTAL_SCORE: 87/100

# Refactor Report: serper_mod

**Agent:** Claude:Opus 4.6
**Codebase:** 5 Go files, ~1,135 lines total (3 source + 2 test)
**Test status:** All passing, 86.4% coverage (serper pkg), 0.0% coverage (cmd pkg)

---

## Executive Summary

`serper_mod` is a compact, well-written Go client for the Serper.dev search API with a CLI wrapper. The code is idiomatic and clean. However, there are several concrete refactoring opportunities that would reduce duplication, improve maintainability, and tighten the API surface. The deductions below reflect genuine improvement areas, not nitpicks.

---

## Scoring Breakdown

| Category | Score | Max | Notes |
|---|---|---|---|
| Code duplication | 6 | 10 | Five near-identical endpoint methods |
| API design consistency | 9 | 10 | Clean options pattern, minor edge case |
| Error handling | 9 | 10 | Solid; one gap in status code mapping |
| Type design | 7 | 10 | Shared request type across all endpoints |
| Testability | 9 | 10 | Good mocks; cmd coverage at 0% |
| Constants & magic values | 7 | 10 | Hardcoded endpoint strings, no User-Agent |
| Separation of concerns | 9 | 10 | Clean library/CLI split |
| Maintainability | 8 | 10 | Small but structurally repetitive |
| Documentation | 8 | 10 | All exported types documented |
| Overall coherence | 15 | 20 | Well-structured; generics would elevate it |
| **TOTAL** | **87** | **100** | |

---

## Refactoring Opportunities

### 1. Eliminate boilerplate with a generic search helper (-5 pts)

**Location:** `serper/client.go:100-163`

The five endpoint methods (`Search`, `Images`, `News`, `Places`, `Scholar`) are structurally identical:

```go
func (c *Client) Foo(ctx context.Context, req *SearchRequest) (*FooResponse, error) {
    prepared, err := prepareRequest(req)
    if err != nil { return nil, err }
    var resp FooResponse
    if err := c.doRequest(ctx, "/foo", prepared, &resp); err != nil {
        return nil, err
    }
    return &resp, nil
}
```

**Recommendation:** Since this project uses Go 1.25, a generic helper would collapse all five methods into single-line delegations:

```go
func doSearch[T any](c *Client, ctx context.Context, endpoint string, req *SearchRequest) (*T, error) {
    prepared, err := prepareRequest(req)
    if err != nil { return nil, err }
    var resp T
    if err := c.doRequest(ctx, endpoint, prepared, &resp); err != nil {
        return nil, err
    }
    return &resp, nil
}

func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
    return doSearch[SearchResponse](c, ctx, "/search", req)
}
```

This removes ~50 lines of repetitive code and eliminates the class of bug where copy-paste results in the wrong endpoint path or response type.

### 2. Define endpoint constants (-3 pts)

**Location:** `serper/client.go:107,120,133,146,159`

Endpoint paths like `"/search"`, `"/images"`, `"/news"`, `"/places"`, `"/scholar"` are bare string literals scattered across methods. A typo (e.g., `"/seach"`) would only be caught at runtime.

**Recommendation:** Define constants alongside the existing ones:

```go
const (
    endpointSearch  = "/search"
    endpointImages  = "/images"
    endpointNews    = "/news"
    endpointPlaces  = "/places"
    endpointScholar = "/scholar"
)
```

### 3. Single `SearchRequest` type serves all five endpoints (-3 pts)

**Location:** `serper/types.go:7-14`

`SearchRequest` is shared across web search, images, news, places, and scholar. But the Serper.dev API likely accepts different parameters for different endpoints (e.g., `tbs` for time-based search, `imageType` for images, `location` already exists but is only meaningful for places). Using one struct for everything means:

- Callers can pass irrelevant fields (e.g., `Page` on images) without compiler help
- Adding endpoint-specific fields pollutes the shared type

**Recommendation:** Either:
- (a) Define per-endpoint request types that embed a common `BaseRequest`, or
- (b) Accept the current design but document which fields apply to which endpoints via field-level doc comments.

Option (b) is the pragmatic choice for a codebase this size.

### 4. Missing HTTP status code mappings (-1 pt)

**Location:** `serper/client.go:204-223`

The error mapping covers 400, 401, 404, 429, 502, 503, and has a default fallback. However:

- **403 Forbidden** is not explicitly mapped. Some APIs return 403 for rate limiting or insufficient permissions; it currently falls through to `InternalError`, which is misleading.
- **408 Request Timeout** is not mapped and would also become an `InternalError`.

**Recommendation:** Add explicit cases for 403 (map to `UnauthorizedError` or a new `ForbiddenError`) and consider 408.

### 5. `CheckConnectivity` consumes API credits (-2 pts)

**Location:** `serper/client.go:166-171`

The comment documents this honestly, but the method makes a real billable search (`Q: "test", Num: 1`). Any automated health check calling this method would silently burn API credits.

**Recommendation:** Consider one of:
- Rename to `CheckConnectivityWithSearch` to make the cost explicit in the method name
- Use a cheaper signal if the API offers one (e.g., a lightweight endpoint, or check if the API returns a specific header on OPTIONS)
- Add a prominent `// WARNING:` comment beyond the current doc comment

### 6. `WithBaseURL` does not validate eagerly (-1 pt)

**Location:** `serper/client.go:40-42`

The `WithBaseURL` option stores the URL without validation. Validation happens after all options in `New()`, which is fine structurally, but if someone constructs a client and passes a valid-looking but semantically wrong URL (e.g., `"https://google.serper.dev/v2"` with a path suffix), `baseURL + endpoint` would produce `"https://google.serper.dev/v2/search"` — probably wrong.

**Recommendation:** Validate in `New()` that `baseURL` has no trailing path component (or strip trailing slashes) to prevent silent URL composition bugs.

### 7. cmd/serper test coverage is 0% for `main()` (-3 pts)

**Location:** `cmd/serper/main_test.go`

The test file only tests `Config` struct loading. The actual `main()` function (argument parsing, client creation, search execution, JSON output) has zero coverage. While testing `main()` is notoriously hard in Go, the current tests could be expanded:

**Recommendation:**
- Extract the core logic from `main()` into a `run(args []string, stdout io.Writer, stderr io.Writer) error` function
- Test `run()` with a mock doer injected via a test-only constructor or by accepting `Config` as a parameter
- This is a common Go pattern (see `cobra`, `kong`, etc.) and would bring meaningful coverage to the CLI

### 8. No `User-Agent` header (-1 pt)

**Location:** `serper/client.go:185-186`

Requests are sent without a `User-Agent` header. Many APIs use this for analytics, debugging, and rate-limiting decisions.

**Recommendation:** Set `User-Agent: serper_mod/<version>` (or a configurable value via an option).

### 9. `apiKey` stored as plain string (-1 pt)

**Location:** `serper/client.go:31`

The API key is stored as a bare `string` in the `Client` struct. While this is standard Go practice, the `chassis-go/v5/secval` package is already imported and used for JSON validation. If `secval` offers a `Secret` type or redaction helper, the API key could benefit from being wrapped to prevent accidental logging (e.g., if someone `fmt.Printf("%+v", client)`).

**Recommendation:** Investigate if `secval` provides a redactable string type and use it for `apiKey`. If not, this is low priority.

---

## Things Done Well (no deductions)

1. **`prepareRequest` copies before mutating** — The defensive copy at `client.go:92` prevents caller mutation, which is tested explicitly in `TestSearch_DoesNotMutateRequest`.

2. **`GetBody` set for retry support** — `client.go:189-191` correctly sets `GetBody` so that `call.Client` retry middleware can replay the request body.

3. **`io.LimitReader` on response** — `client.go:199` caps response reads at 10MB, preventing a malicious or buggy server from causing OOM.

4. **Error body truncation** — `client.go:206-208` truncates error bodies to 1KB before including them in error messages, preventing log flooding.

5. **Context propagation** — All public methods accept `context.Context` as the first parameter, and the HTTP request is created with `NewRequestWithContext`.

6. **`secval.ValidateJSON` before unmarshal** — `client.go:226` validates response JSON safety before deserializing, adding a defense layer against injection payloads.

7. **Functional options pattern** — Clean, extensible client configuration.

8. **Per-context API key override** — `WithAPIKey` provides a clean mechanism for multi-tenant usage without creating multiple clients.

9. **Comprehensive test coverage** — 86.4% on the library, with edge cases (truncation, cancellation, malformed JSON, boundary values) all covered.

---

## Summary

This is a well-crafted, small codebase. The primary refactoring opportunity is eliminating the five nearly-identical endpoint methods via Go generics — this alone would remove ~50 lines of boilerplate and reduce the copy-paste error surface. The other suggestions are lower priority but would incrementally improve robustness (URL validation, status code coverage) and observability (User-Agent, API key redaction). The cmd package would benefit from a testable `run()` extraction pattern.
