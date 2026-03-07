Date Created: 2026-02-15T22:48:17-05:00
TOTAL_SCORE: 87/100

# Audit Report: serper_mod

**Auditor:** Claude:Opus 4.6
**Version Audited:** 1.4.0
**Module:** `github.com/ai8future/serper_mod`

---

## Executive Summary

serper_mod is a well-structured Go client library + CLI for the Serper.dev search API. The codebase is compact (~490 LOC across 3 source files), follows idiomatic Go patterns, has strong test coverage (86.4% for the library), and demonstrates good security hygiene. Deductions stem from a few design gaps, a concurrency safety concern, missing CI, and minor hardening opportunities.

---

## Score Breakdown

| Category | Max | Score | Notes |
|---|---|---|---|
| **Security** | 25 | 21 | API key in memory (not avoidable in Go), no TLS pinning, SSRF potential via WithBaseURL |
| **Correctness & Bug Risk** | 25 | 22 | Client not concurrency-safe, Num=-1 after SetDefaults passes validation |
| **Code Quality** | 20 | 19 | Clean, idiomatic Go; minor DRY opportunity in endpoint methods |
| **Testing** | 15 | 13 | 86.4% lib coverage, 0% CLI main coverage, no integration/fuzz tests |
| **Project Hygiene** | 15 | 12 | No CI/CD, no LICENSE file, CHANGELOG version gap (1.2.0 -> 1.4.0) |

---

## Detailed Findings

### SECURITY

#### SEC-1: SSRF via `WithBaseURL` (Medium)
**Location:** `serper/client.go:40-42`
**Description:** `WithBaseURL` accepts any URL including internal/private network addresses. A caller could set `http://169.254.169.254/latest/meta-data` or `http://localhost:9200` to reach internal services. While the library is a client SDK (callers control their own config), library consumers embedding this in a multi-tenant service could inadvertently expose SSRF if the base URL is user-configurable.
**Recommendation:** Consider validating that the base URL uses HTTPS scheme, or at minimum document the risk.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -62,6 +62,9 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
 		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
 	}
+	if u, _ := url.Parse(c.baseURL); u != nil && u.Scheme != "https" && u.Scheme != "http" {
+		return nil, fmt.Errorf("serper: base URL must use http or https scheme, got %q", u.Scheme)
+	}
 	return c, nil
 }
```

#### SEC-2: API Key Logged at Debug Level (Low)
**Location:** `cmd/serper/main.go:57`
**Description:** The debug log at line 57 logs the query and config fields but not the API key directly. This is fine. However, the `Config` struct has no `String()` method to prevent accidental logging of the APIKey field if the whole config is ever logged.
**Recommendation:** Add a `String()` or `MarshalJSON()` method on `Config` that redacts APIKey, or use chassis secval to wrap it.

#### SEC-3: JSON Validation of Response (Positive)
**Location:** `serper/client.go:226-228`
**Description:** The code uses `secval.ValidateJSON` before unmarshalling. This is a strong security practice that prevents JSON-based injection attacks.

#### SEC-4: Response Size Limiting (Positive)
**Location:** `serper/client.go:199`
**Description:** `io.LimitReader(resp.Body, maxResponseBytes)` prevents memory exhaustion from a malicious or misconfigured server response. 10MB limit is reasonable.

#### SEC-5: Error Body Truncation (Positive)
**Location:** `serper/client.go:206-208`
**Description:** Error bodies are truncated to 1KB before inclusion in error messages, preventing information leakage and log flooding.

---

### CORRECTNESS & BUG RISK

#### BUG-1: Client Is Not Concurrency-Safe (Medium)
**Location:** `serper/client.go:30-34`
**Description:** The `Client` struct stores `apiKey`, `baseURL`, and `doer` as plain fields. While these are set once during construction and read-only after `New()` returns, the `Doer` interface contract does not specify thread safety. The default `http.Client` is safe, but a user-supplied `Doer` might not be. More importantly, the `Client` struct has no documentation asserting it is safe for concurrent use.
**Recommendation:** Add a doc comment stating concurrency expectations.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -28,7 +28,8 @@ type Doer interface {
 	Do(*http.Request) (*http.Response, error)
 }

-// Client is a Serper.dev API client.
+// Client is a Serper.dev API client. It is safe for concurrent use as long as
+// the Doer provided via WithDoer is also safe for concurrent use.
 type Client struct {
 	apiKey  string
 	baseURL string
```

#### BUG-2: Negative Num Bypasses Validation After SetDefaults (Low)
**Location:** `serper/types.go:148-149`, `serper/types.go:168`
**Description:** `SetDefaults()` only sets `Num` when it is `0`. If a caller passes `Num: -1`, `SetDefaults()` leaves it as `-1`, and `Validate()` correctly catches it. However, the zero-value default behavior means there's no way for a caller to explicitly request "server default" (Num=0) after SetDefaults runs. This is acceptable design but worth noting.

#### BUG-3: `prepareRequest` Shallow Copy May Break With Future Slice/Map Fields (Low)
**Location:** `serper/client.go:92`
**Description:** `cp := *req` performs a shallow copy. Currently `SearchRequest` contains only value types (string, int), so this is safe. If a slice or map field is ever added to `SearchRequest`, this would share underlying data with the caller's struct.
**Recommendation:** Add a comment noting this constraint, or proactively use a deep-copy approach.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -89,6 +89,8 @@ func (c *Client) getAPIKey(ctx context.Context) string {

 // prepareRequest copies the request, applies defaults, and validates it.
 // The original request is never modified.
+// NOTE: This is a shallow copy. If SearchRequest gains slice/map fields,
+// this must be updated to a deep copy to avoid aliasing.
 func prepareRequest(req *SearchRequest) (*SearchRequest, error) {
 	cp := *req
 	cp.SetDefaults()
```

#### BUG-4: `CheckConnectivity` Costs Money With No Warning in Code (Info)
**Location:** `serper/client.go:166-171`
**Description:** `CheckConnectivity` makes a real API call that counts toward billing. The doc comment notes this, which is good. However, there's no programmatic way to do a health check without consuming a credit.

#### BUG-5: Endpoint String Concatenation (Info)
**Location:** `serper/client.go:180`
**Description:** `c.baseURL+endpoint` concatenates directly. If the baseURL has a trailing slash, the result is `https://example.com//search`. `url.ParseRequestURI` in `New()` doesn't reject trailing slashes. This is unlikely to cause issues with the Serper API but could with other servers.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -60,6 +60,7 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 	for _, o := range opts {
 		o(c)
 	}
+	c.baseURL = strings.TrimRight(c.baseURL, "/")
 	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
 		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
 	}
```
Note: This would require adding `"strings"` to the import block.

---

### CODE QUALITY

#### CQ-1: Repetitive Endpoint Methods (Low)
**Location:** `serper/client.go:100-163`
**Description:** The five endpoint methods (Search, Images, News, Places, Scholar) follow an identical pattern: prepareRequest -> doRequest -> return. This is a classic case where a generic helper could reduce duplication. However, each returns a different response type, and Go's type system (pre-generics or with generics) makes this a design trade-off. The current approach is explicit and readable, which is preferable for a small number of endpoints.

#### CQ-2: Error Wrapping Consistency (Low)
**Location:** `serper/client.go:175-231`
**Description:** Some errors use `fmt.Errorf("serper: ...")` (plain errors) while others use `chassiserrors.*Error(...)` (typed errors). The typed errors are only used for HTTP status code mapping (400, 401, 404, 429, 502, 503). Marshal/create/read/unmarshal errors use plain `fmt.Errorf`. This is a reasonable split: typed errors for recoverable/actionable conditions, plain errors for unexpected failures. However, the marshal error at line 177 could arguably be an `InternalError`.

#### CQ-3: Good Use of Functional Options Pattern (Positive)
**Location:** `serper/client.go:37-47`
**Description:** The options pattern is clean and idiomatic Go.

#### CQ-4: Clean Separation of Concerns (Positive)
**Description:** Types in `types.go`, HTTP logic in `client.go`, CLI in `cmd/serper/main.go`. Good layering.

---

### TESTING

#### TEST-1: 0% Coverage on CLI main (Medium)
**Location:** `cmd/serper/main.go`
**Description:** The `main()` function has no coverage. The existing tests in `main_test.go` only test config loading. The actual CLI flow (arg parsing, client creation, search execution, output formatting) is untested. This is understandable for a thin CLI wrapper but means regressions in the wiring could go unnoticed.
**Recommendation:** Extract the core logic into a `run(args []string, stdout io.Writer) error` function and test that.

```diff
--- a/cmd/serper/main.go
+++ b/cmd/serper/main.go
@@ -29,8 +29,8 @@ type Config struct {
 }

 func main() {
-	chassis.RequireMajor(5)
-	cfg := chassisconfig.MustLoad[Config]()
+	if err := run(os.Args, os.Stdout); err != nil {
+		fmt.Fprintf(os.Stderr, "error: %v\n", err)
+		os.Exit(1)
+	}
+}
+
+func run(args []string, stdout io.Writer) error {
+	chassis.RequireMajor(5)
+	cfg := chassisconfig.MustLoad[Config]()
 	logger := logz.New(cfg.LogLevel)
 	logger.Info("starting", "chassis_version", chassis.Version)
 	// ... (extract remainder into run())
```

#### TEST-2: No Fuzz Tests (Low)
**Description:** `SearchRequest.Validate()` and JSON parsing are good candidates for fuzz testing (`testing.F`). This could catch edge cases in validation logic.

#### TEST-3: No Integration Test Infrastructure (Low)
**Description:** There's no test that hits the real Serper API (even gated behind an env var). A `TestIntegration_Search` guarded by `if os.Getenv("SERPER_API_KEY") == ""` would be valuable for CI.

#### TEST-4: Test Quality Is High (Positive)
**Description:** Tests are well-structured, use table-driven patterns where appropriate, verify both success and failure paths, check request details (URL, headers, method, body), and test edge cases like context cancellation, truncation, and boundary values. 86.4% coverage for the core library is strong.

---

### PROJECT HYGIENE

#### HYG-1: No CI/CD Pipeline (Medium)
**Description:** No `.github/workflows/`, `Makefile`, or CI configuration exists. Tests, vet, and race detection are only run manually.
**Recommendation:** Add a minimal GitHub Actions workflow.

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go vet ./...
      - run: go test -race -cover ./...
```

#### HYG-2: No LICENSE File (Low)
**Description:** The repository has no LICENSE file. This means the code is technically "all rights reserved" by default. If this is an internal project, this may be intentional.

#### HYG-3: CHANGELOG Version Gap (Info)
**Description:** Versions jump from 1.2.0 to 1.4.0 (no 1.3.0). This suggests either a skipped version or work done without changelog updates. Not a bug, but unusual.

#### HYG-4: `.gitignore` Ignores `go.work` (Info)
**Location:** `.gitignore:29-30`
**Description:** `go.work` and `go.work.sum` are gitignored. These files exist in the working directory for local development with chassis-go. This is the correct pattern for libraries that use `go.work` only for local dev.

---

## Concurrency & Race Condition Analysis

All tests pass with `-race` flag. The `Client` struct is effectively immutable after construction (all fields set in `New()` and never modified). Each request creates its own `http.Request` and response processing is stack-local. No global mutable state exists. **No race conditions detected.**

---

## Dependency Analysis

| Dependency | Version | Risk |
|---|---|---|
| chassis-go/v5 | v5.0.0 | Internal framework, controlled |
| opentelemetry (transitive) | v1.40.0 | Well-maintained, low risk |
| grpc (transitive) | v1.78.0 | Well-maintained, low risk |
| protobuf (transitive) | v1.36.11 | Well-maintained, low risk |

No known CVEs in listed dependency versions as of audit date.

---

## Summary of Recommended Changes (Priority Order)

| # | Finding | Severity | Effort |
|---|---|---|---|
| 1 | BUG-1: Document concurrency safety | Medium | 5 min |
| 2 | SEC-1: Validate/document SSRF risk in WithBaseURL | Medium | 10 min |
| 3 | TEST-1: Extract CLI `run()` for testability | Medium | 30 min |
| 4 | HYG-1: Add CI/CD pipeline | Medium | 15 min |
| 5 | BUG-5: Normalize trailing slash in baseURL | Low | 5 min |
| 6 | BUG-3: Document shallow copy constraint | Low | 2 min |
| 7 | SEC-2: Add Config.String() to redact API key | Low | 10 min |
| 8 | HYG-2: Add LICENSE file | Low | 2 min |

---

## What's Done Well

- **Immutable request handling**: `prepareRequest` copies before mutating - prevents caller side effects
- **Typed error mapping**: HTTP status codes map to semantic error types from chassis-go
- **Response size limiting**: 10MB cap via `io.LimitReader` prevents memory exhaustion
- **JSON validation**: `secval.ValidateJSON` before unmarshal catches malformed/malicious JSON
- **Error body truncation**: Prevents log flooding from large error responses
- **GetBody for retry support**: Enables middleware to replay request bodies
- **Clean functional options pattern**: Idiomatic Go construction
- **Context propagation**: All methods accept `context.Context` and pass it through
- **Comprehensive test suite**: 86.4% coverage with edge cases, boundary values, and mock infrastructure
