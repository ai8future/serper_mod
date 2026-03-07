Date Created: 2026-02-15T23:02:00-05:00
TOTAL_SCORE: 87/100

# serper_mod Code Audit & Fix Report

**Auditor:** Claude Opus 4.6
**Module:** `github.com/ai8future/serper_mod`
**Version:** 1.4.0
**Go Version:** 1.25.5
**Chassis Dependency:** `chassis-go/v5` v5.0.0

---

## Executive Summary

This is a well-structured Go client library for the Serper.dev search API with a companion CLI. The codebase demonstrates solid engineering practices: immutable request handling, typed error classification, JSON security validation via secval, retry support, and 86.4% test coverage on the library package. The code is clean, idiomatic, and relatively small (~410 lines of production code across 3 files).

No critical bugs were found. Issues are primarily minor robustness concerns, a couple of edge-case behaviors, and one silent data-corruption risk.

---

## Test Results

```
All tests:    32/32 PASS (30 serper + 2 cmd)
Race detector: CLEAN
go vet:        CLEAN
Coverage:
  serper/      86.4%
  cmd/serper/  0.0% (main() not unit-testable)
```

---

## Scoring Breakdown

| Category                   | Max | Score | Notes                                            |
|----------------------------|-----|-------|--------------------------------------------------|
| Correctness                | 25  | 23    | One silent truncation risk (Issue #1)             |
| Security                   | 20  | 19    | secval, key validation, body limits all solid     |
| Error handling             | 15  | 14    | Minor: errors use plain fmt.Errorf, not all typed |
| Test quality               | 15  | 13    | 86.4% coverage, no error-path tests for non-Search endpoints |
| Code clarity & style       | 10  | 9     | Clean, idiomatic, good comments                   |
| API design                 | 10  | 9     | Good immutability, options pattern, Doer interface |
| Dependency hygiene         | 5   | 5     | Minimal deps, all indirect are via chassis-go     |
| **Total**                  | **100** | **87** |                                               |

---

## Issues Found

### Issue #1 — Silent Response Truncation (Medium Severity)

**File:** `serper/client.go:199`

```go
body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
```

If the API returns a response larger than 10 MB, `io.LimitReader` silently truncates the stream. The subsequent `json.Unmarshal` will fail with a parse error, but the error message will say "unmarshal response" rather than "response exceeded 10 MB limit." This makes debugging production issues harder and could mislead operators into thinking the response was malformed rather than too large.

**Recommendation:** After reading, check if exactly `maxResponseBytes` were read and the stream has more data. If so, return a clear error.

**Patch-ready diff:**
```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -196,9 +196,16 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 	}
 	defer resp.Body.Close()

-	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
+	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
 	if err != nil {
 		return fmt.Errorf("serper: read response: %w", err)
+	}
+	if int64(len(body)) > maxResponseBytes {
+		return fmt.Errorf("serper: response body exceeds %d byte limit", maxResponseBytes)
 	}

 	if resp.StatusCode >= 400 {
```

---

### Issue #2 — Error Body Truncation Produces Variable-Length Messages (Low Severity)

**File:** `serper/client.go:205-208`

```go
msg := string(body)
if len(msg) > maxErrorBodyBytes {
    msg = msg[:maxErrorBodyBytes] + "...(truncated)"
}
```

The truncation suffix `"...(truncated)"` (14 chars) is appended after the 1024-byte slice, meaning error messages can be up to 1038 bytes plus the prefix. This is fine functionally but semantically inconsistent — the intent is to cap the body at 1024 bytes, yet the output exceeds it. Not a bug, but worth noting for log-size-sensitive environments.

**Recommendation:** No change required. The current behavior is tested and acceptable. If strict capping is desired:

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -204,7 +204,8 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 	if resp.StatusCode >= 400 {
 		msg := string(body)
 		if len(msg) > maxErrorBodyBytes {
-			msg = msg[:maxErrorBodyBytes] + "...(truncated)"
+			const suffix = "...(truncated)"
+			msg = msg[:maxErrorBodyBytes-len(suffix)] + suffix
 		}
 		detail := fmt.Sprintf("serper: HTTP %d: %s", resp.StatusCode, msg)
```

---

### Issue #3 — Validation Errors Are Plain `error`, Not Typed (Low Severity)

**File:** `serper/types.go:164-174`

```go
func (r *SearchRequest) Validate() error {
    if r.Q == "" {
        return fmt.Errorf("query (q) is required")
    }
    ...
}
```

Client-side validation errors from `Validate()` return plain `fmt.Errorf` errors, while HTTP-level validation errors (400 status) return `chassiserrors.ValidationError`. This means callers using `errors.Is/As` to match chassis error types will get inconsistent behavior depending on whether the validation failed locally vs. server-side.

**Recommendation:** Wrap local validation errors with `chassiserrors.ValidationError` for consistency.

**Patch-ready diff:**
```diff
--- a/serper/types.go
+++ b/serper/types.go
@@ -2,7 +2,10 @@
 package serper

-import "fmt"
+import (
+	"fmt"
+	chassiserrors "github.com/ai8future/chassis-go/v5/errors"
+)

 // SearchRequest represents a search request to Serper.dev.
 type SearchRequest struct {
@@ -162,13 +165,13 @@ func (r *SearchRequest) SetDefaults() {
 // Validate checks that the SearchRequest is valid.
 // Call SetDefaults before Validate if you want zero-value fields filled in.
 func (r *SearchRequest) Validate() error {
 	if r.Q == "" {
-		return fmt.Errorf("query (q) is required")
+		return chassiserrors.ValidationError("query (q) is required")
 	}
 	if r.Num < 1 || r.Num > 100 {
-		return fmt.Errorf("num must be between 1 and 100")
+		return chassiserrors.ValidationError("num must be between 1 and 100")
 	}
 	if r.Page < 1 {
-		return fmt.Errorf("page must be 1 or greater")
+		return chassiserrors.ValidationError("page must be 1 or greater")
 	}
 	return nil
 }
```

---

### Issue #4 — `SetDefaults` Cannot Distinguish "Explicitly Set to Zero" from "Unset" (Low Severity, Design Limitation)

**File:** `serper/types.go:147-160`

```go
func (r *SearchRequest) SetDefaults() {
    if r.Num == 0 {
        r.Num = 10
    }
```

Because Go's zero value for `int` is `0`, `SetDefaults` cannot distinguish between "the caller didn't set Num" and "the caller explicitly wants Num=0." Since `Validate` rejects `Num < 1`, this is safely caught downstream, but a caller who explicitly passes `Num: 0` will silently get `Num: 10` instead of a validation error.

**Impact:** Minimal — the behavior is reasonable and the pipeline (SetDefaults → Validate) works correctly. Same applies to `Page: 0`.

**Recommendation:** No code change. This is an inherent Go idiom and the current SetDefaults/Validate ordering handles it safely. Document the behavior if it becomes a support question.

---

### Issue #5 — HTTP Status 429 Mapped to `RateLimitError` but Not Retryable by Default (Low Severity)

**File:** `serper/client.go:217-218`

```go
case http.StatusTooManyRequests:
    return chassiserrors.RateLimitError(detail)
```

Rate-limit errors (HTTP 429) are correctly classified, but the client has no built-in back-off or retry-after header parsing. When using `call.Client` with retry, the retry middleware may re-send the request immediately without respecting the `Retry-After` header, potentially exacerbating rate limiting.

**Recommendation:** This is an enhancement, not a bug. If the Serper API sends `Retry-After` headers, consider parsing them. The current chassis-go retry with 500ms backoff provides reasonable default behavior.

---

### Issue #6 — CLI Retry/Backoff Parameters Are Hardcoded (Low Severity)

**File:** `cmd/serper/main.go:45`

```go
call.WithRetry(3, 500*time.Millisecond),
```

The retry count (3) and backoff interval (500ms) are hardcoded. All other configuration (timeout, API key, URL, num, gl, hl) is exposed via environment variables, making this an inconsistency.

**Patch-ready diff:**
```diff
--- a/cmd/serper/main.go
+++ b/cmd/serper/main.go
@@ -20,13 +20,15 @@ import (

 // Config holds CLI configuration loaded from environment.
 type Config struct {
-	APIKey   string        `env:"SERPER_API_KEY" required:"true"`
-	BaseURL  string        `env:"SERPER_BASE_URL" default:"https://google.serper.dev"`
-	Num      int           `env:"SERPER_NUM" default:"10"`
-	GL       string        `env:"SERPER_GL" default:"us"`
-	HL       string        `env:"SERPER_HL" default:"en"`
-	Timeout  time.Duration `env:"SERPER_TIMEOUT" default:"30s"`
-	LogLevel string        `env:"LOG_LEVEL" default:"error"`
+	APIKey       string        `env:"SERPER_API_KEY" required:"true"`
+	BaseURL      string        `env:"SERPER_BASE_URL" default:"https://google.serper.dev"`
+	Num          int           `env:"SERPER_NUM" default:"10"`
+	GL           string        `env:"SERPER_GL" default:"us"`
+	HL           string        `env:"SERPER_HL" default:"en"`
+	Timeout      time.Duration `env:"SERPER_TIMEOUT" default:"30s"`
+	RetryCount   int           `env:"SERPER_RETRY_COUNT" default:"3"`
+	RetryBackoff time.Duration `env:"SERPER_RETRY_BACKOFF" default:"500ms"`
+	LogLevel     string        `env:"LOG_LEVEL" default:"error"`
 }

 func main() {
@@ -43,7 +45,7 @@ func main() {

 	caller := call.New(
 		call.WithTimeout(cfg.Timeout),
-		call.WithRetry(3, 500*time.Millisecond),
+		call.WithRetry(cfg.RetryCount, cfg.RetryBackoff),
 	)
```

---

### Issue #7 — `mockDoer` in Tests Is Not Concurrency-Safe (Low Severity)

**File:** `serper/client_test.go:14-19`

```go
type mockDoer struct {
    req        *http.Request
    body       []byte
    statusCode int
    respBody   string
    err        error
}
```

The mock captures the request on each `Do()` call by writing to shared fields. While Go tests within a single `Test*` function run sequentially by default, if any tests are ever run with `t.Parallel()`, this mock would have data races. The `-race` flag passes today only because no tests use `t.Parallel()`.

**Recommendation:** Not urgent. If parallel tests are added in the future, use a `sync.Mutex` in the mock or create per-test mock instances (which is already the current pattern).

---

### Issue #8 — No GL/HL Validation (Low Severity)

**File:** `serper/types.go:164-174`

The `Validate()` method checks `Q`, `Num`, and `Page` but does not validate `GL` (geographic locale) or `HL` (language). Invalid values like `GL: "zzz"` are silently sent to the API.

**Recommendation:** This is intentional — the Serper API is the authority on valid locales and will reject invalid ones server-side. Client-side locale validation would be a maintenance burden (locale lists change). The current approach is correct.

---

### Issue #9 — `main()` Coverage Is 0% (Informational)

**File:** `cmd/serper/main.go`

The `main()` function has 0% coverage because it's not unit-testable (it calls `os.Exit`, reads `os.Args`, and requires environment variables). The two existing tests cover the `Config` struct loading, which is the testable part.

**Recommendation:** This is standard for Go CLI tools. The architecture correctly separates testable logic (serper package at 86.4%) from the thin CLI wrapper.

---

### Issue #10 — Endpoint Methods Have 71.4% Coverage (Informational)

**File:** `serper/client.go:114-163`

`Images`, `News`, `Places`, and `Scholar` methods each show 71.4% coverage. The tests exercise the success path but not the validation-error path (when `prepareRequest` returns an error). Since `prepareRequest` is thoroughly tested through `Search`, the uncovered branches are structurally identical code.

**Recommendation:** Minor gap. If comprehensive coverage is a goal, add one validation-error test per endpoint. The risk is low since all five methods delegate to the same `prepareRequest` and `doRequest` functions.

---

## Code Smells

### Repeated Endpoint Method Pattern

Methods `Search`, `Images`, `News`, `Places`, and `Scholar` (lines 101-163) are nearly identical — differing only in endpoint path and response type. With Go generics (available since 1.18), this could be reduced to a single generic function:

```go
func doEndpoint[T any](c *Client, ctx context.Context, endpoint string, req *SearchRequest) (*T, error) {
    prepared, err := prepareRequest(req)
    if err != nil {
        return nil, err
    }
    var resp T
    if err := c.doRequest(ctx, endpoint, prepared, &resp); err != nil {
        return nil, err
    }
    return &resp, nil
}
```

However, the current approach has the advantage of explicit, documented public methods with clear godoc. This is a style preference, not a defect. The five methods are stable and unlikely to diverge.

---

## Positive Observations

1. **Immutability discipline:** `prepareRequest` copies the input before modifying, tested by `TestSearch_DoesNotMutateRequest`.
2. **Defensive I/O:** Response body limited to 10 MB, error bodies truncated to 1 KB.
3. **Security validation:** `secval.ValidateJSON` guards against unsafe JSON payloads before unmarshaling.
4. **Retry support:** `req.GetBody` is set for middleware replay, tested explicitly.
5. **Context propagation:** API key overrides via context, cancellation tested.
6. **Clean error typing:** HTTP status codes mapped to chassis typed errors for downstream handling.
7. **URL validation:** Constructor validates the base URL at build time, not at request time.
8. **Version gate:** `chassis.RequireMajor(5)` ensures runtime compatibility.
9. **Test quality:** 30 focused tests with clear names, table-driven validation tests, mock doer, and edge cases (empty body, truncation, malformed JSON).
10. **Minimal dependencies:** Only chassis-go as a direct dependency.

---

## Summary of Recommended Fixes (by Priority)

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| Medium   | #1 — Silent response truncation | Small | Debuggability |
| Low      | #3 — Validation errors not typed | Small | API consistency |
| Low      | #6 — Hardcoded retry params | Small | Configurability |
| Low      | #2 — Truncation suffix length | Tiny  | Pedantic correctness |

All other issues are informational or by-design trade-offs that don't warrant code changes.
