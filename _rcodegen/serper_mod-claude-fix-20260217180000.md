Date Created: 2026-02-17T18:00:00Z
TOTAL_SCORE: 88/100

# serper_mod Code Audit Report

**Auditor:** Claude Opus 4.6
**Module:** `github.com/ai8future/serper_mod`
**Version:** 1.4.0
**Go Version:** 1.25.5
**Chassis-Go Version:** v5.0.0

---

## Executive Summary

This is a well-engineered Go client library for the Serper.dev search API. The codebase demonstrates strong fundamentals: clean separation of concerns, comprehensive validation, proper error handling via chassis-go typed errors, request non-mutation, and a solid test suite at 86.4% coverage for the core library.

The issues found are minor to moderate — no critical bugs or security vulnerabilities were discovered. The deductions primarily come from a missing HTTP status handling edge case, cmd coverage gap, a version number skip in the changelog, and a few code hygiene items.

---

## Scoring Breakdown

| Category | Max Points | Score | Notes |
|---|---|---|---|
| Correctness / Bugs | 25 | 22 | Missing `http.StatusForbidden` (403) mapping; Validate() error messages not wrapped with chassis errors |
| Security | 15 | 15 | Excellent: secval.ValidateJSON, response size limits, no hardcoded secrets, API key from env only |
| Error Handling | 15 | 14 | Solid overall; one minor gap: 403 maps to generic InternalError instead of UnauthorizedError |
| Test Quality | 15 | 12 | 86.4% on serper package (good), but 0% on cmd/serper (main function untestable) |
| Code Organization | 10 | 10 | Clean separation: types.go, client.go, cmd/. Option pattern, Doer interface |
| Maintainability | 10 | 8 | Version skip 1.2→1.4 in changelog; missing README.md |
| API Design | 10 | 7 | Endpoint methods are repetitive boilerplate; SetDefaults/Validate could be combined |
| **TOTAL** | **100** | **88** | |

---

## Issues Found

### Issue 1: Missing HTTP 403 (Forbidden) Error Mapping [Severity: Medium]

**File:** `serper/client.go:210-223`

The HTTP status code switch in `doRequest` handles 400, 401, 404, 429, 502, and 503 — but does not handle 403 (Forbidden). Serper.dev can return 403 when an API key is valid but lacks permission for a specific endpoint. Currently, a 403 response falls through to the `default` case and returns a generic `InternalError`, which is semantically incorrect.

**Current code:**
```go
switch resp.StatusCode {
case http.StatusBadRequest:
    return chassiserrors.ValidationError(detail)
case http.StatusUnauthorized:
    return chassiserrors.UnauthorizedError(detail)
case http.StatusNotFound:
    return chassiserrors.NotFoundError(detail)
case http.StatusTooManyRequests:
    return chassiserrors.RateLimitError(detail)
case http.StatusBadGateway, http.StatusServiceUnavailable:
    return chassiserrors.DependencyError(detail)
default:
    return chassiserrors.InternalError(detail)
}
```

**Proposed fix (patch-ready):**
```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -211,6 +211,8 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 		return chassiserrors.ValidationError(detail)
 	case http.StatusUnauthorized:
 		return chassiserrors.UnauthorizedError(detail)
+	case http.StatusForbidden:
+		return chassiserrors.UnauthorizedError(detail)
 	case http.StatusNotFound:
 		return chassiserrors.NotFoundError(detail)
 	case http.StatusTooManyRequests:
```

**Test to add:**
```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -538,6 +538,18 @@ func TestSearch_HTTP500(t *testing.T) {
 	}
 }

+func TestSearch_HTTP403(t *testing.T) {
+	mock := &mockDoer{statusCode: 403, respBody: `{"error":"forbidden"}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for 403 status")
+	}
+	if !strings.Contains(err.Error(), "403") {
+		t.Errorf("error should contain '403', got: %v", err)
+	}
+}
+
 func TestSearch_HTTP503(t *testing.T) {
```

---

### Issue 2: Validation Errors Not Wrapped as Chassis Typed Errors [Severity: Low]

**File:** `serper/types.go:164-175`

`Validate()` returns plain `fmt.Errorf` errors. All other error paths in the client use `chassiserrors` typed errors. This means callers who check for `chassiserrors.IsValidationError(err)` won't match validation failures from `Validate()`. The validation errors from `prepareRequest()` bubble up directly from the client methods.

**Current code:**
```go
func (r *SearchRequest) Validate() error {
    if r.Q == "" {
        return fmt.Errorf("query (q) is required")
    }
    if r.Num < 1 || r.Num > 100 {
        return fmt.Errorf("num must be between 1 and 100")
    }
    if r.Page < 1 {
        return fmt.Errorf("page must be 1 or greater")
    }
    return nil
}
```

**Proposed fix (patch-ready):**
```diff
--- a/serper/types.go
+++ b/serper/types.go
@@ -1,7 +1,9 @@
 // Package serper provides a client for the Serper.dev search API.
 package serper

-import "fmt"
+import (
+	chassiserrors "github.com/ai8future/chassis-go/v5/errors"
+)

 // SearchRequest represents a search request to Serper.dev.
 type SearchRequest struct {
@@ -161,14 +163,14 @@
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

**Note:** This change would affect test assertions that check for exact error strings. Tests using `strings.Contains(err.Error(), "query")` would still pass since the message is embedded. Verify that `chassiserrors.ValidationError()` wraps the message in a way that preserves it in `.Error()`.

---

### Issue 3: Ignored Error in Mock `io.ReadAll` [Severity: Low / Test-Only]

**File:** `serper/client_test.go:25`

```go
m.body, _ = io.ReadAll(req.Body)
```

**File:** `serper/client_test.go:263`

```go
data, _ := io.ReadAll(body)
```

While these are in test code and unlikely to cause issues in practice, ignoring `io.ReadAll` errors in mock infrastructure can mask test failures silently. If the body read fails, `m.body` would be nil/empty, and subsequent assertions on the body content could produce confusing false positives.

**Proposed fix (patch-ready):**
```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -13,6 +13,7 @@ type mockDoer struct {
 	req        *http.Request
 	body       []byte
 	statusCode int
+	t          *testing.T
 	respBody   string
 	err        error
 }
@@ -22,7 +23,10 @@ func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
 	m.req = req
 	if req.Body != nil {
-		m.body, _ = io.ReadAll(req.Body)
+		var err error
+		m.body, err = io.ReadAll(req.Body)
+		if err != nil && m.t != nil {
+			m.t.Fatalf("mockDoer: failed to read request body: %v", err)
+		}
 	}
```

**Alternative (minimal):** Add a comment explaining why the error is safe to ignore:
```diff
-		m.body, _ = io.ReadAll(req.Body)
+		m.body, _ = io.ReadAll(req.Body) // error ignored: test mock; body always bytes.Reader
```

---

### Issue 4: Repetitive Endpoint Method Boilerplate [Severity: Low / Code Smell]

**File:** `serper/client.go:100-163`

The five endpoint methods (`Search`, `Images`, `News`, `Places`, `Scholar`) follow an identical pattern:
1. `prepareRequest(req)`
2. Declare a response variable
3. Call `c.doRequest(ctx, endpoint, prepared, &resp)`
4. Return

This is a minor code smell — not a bug — but means any future endpoint addition requires copy-pasting the same 10-line pattern. A generic helper could reduce this.

**Proposed fix (illustrative, not strictly necessary):**
```diff
+// search is a generic helper for all search endpoints.
+func search[T any](c *Client, ctx context.Context, endpoint string, req *SearchRequest) (*T, error) {
+	prepared, err := prepareRequest(req)
+	if err != nil {
+		return nil, err
+	}
+	var resp T
+	if err := c.doRequest(ctx, endpoint, prepared, &resp); err != nil {
+		return nil, err
+	}
+	return &resp, nil
+}
+
 // Search performs a web search via Serper.dev.
 func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
-	prepared, err := prepareRequest(req)
-	if err != nil {
-		return nil, err
-	}
-	var resp SearchResponse
-	if err := c.doRequest(ctx, "/search", prepared, &resp); err != nil {
-		return nil, err
-	}
-	return &resp, nil
+	return search[SearchResponse](c, ctx, "/search", req)
 }
```

**Note:** This uses Go generics (available since Go 1.18). However, the current explicit approach is clear and readable. This is a style preference, not a correctness issue.

---

### Issue 5: Version Number Gap in Changelog [Severity: Info]

**File:** `CHANGELOG.md`

The version history jumps from 1.2.0 to 1.4.0, skipping 1.3.0 entirely. While not a code bug, this can confuse maintainers and automated tooling that expects sequential version progression. If the v1.3.0 changes were superseded or rolled back, a note explaining the skip would be helpful.

---

### Issue 6: No `README.md` [Severity: Info]

The project has no README.md file. For a published Go module, a README with basic usage examples, installation instructions, and API documentation is standard practice and expected by tools like `pkg.go.dev`.

---

### Issue 7: CLI Coverage at 0% [Severity: Low]

**File:** `cmd/serper/main.go`

The `cmd/serper` package shows 0% statement coverage. The existing tests (`TestConfig_Defaults`, `TestConfig_PanicsWithoutAPIKey`) only exercise the `Config` struct loading — they don't test the `main()` function itself. This is common for CLI entry points, but the function contains non-trivial logic (argument parsing, client creation, response formatting) that could benefit from being extracted into a testable `run() error` function.

**Proposed refactor (patch-ready):**
```diff
--- a/cmd/serper/main.go
+++ b/cmd/serper/main.go
@@ -30,7 +30,11 @@

 func main() {
 	chassis.RequireMajor(5)
+	if err := run(); err != nil {
+		fmt.Fprintf(os.Stderr, "error: %v\n", err)
+		os.Exit(1)
+	}
+}
+
+func run() error {
 	cfg := chassisconfig.MustLoad[Config]()
 	logger := logz.New(cfg.LogLevel)
 	logger.Info("starting", "chassis_version", chassis.Version)

 	if len(os.Args) < 2 {
-		fmt.Fprintln(os.Stderr, "usage: serper <query>")
-		os.Exit(1)
+		return fmt.Errorf("usage: serper <query>")
 	}
 	query := strings.Join(os.Args[1:], " ")

@@ -48,24 +52,21 @@
 	client, err := serper.New(cfg.APIKey,
 		serper.WithBaseURL(cfg.BaseURL),
 		serper.WithDoer(caller),
 	)
 	if err != nil {
-		fmt.Fprintf(os.Stderr, "error: %v\n", err)
-		os.Exit(1)
+		return fmt.Errorf("create client: %w", err)
 	}

 	logger.Debug("searching", "query", query, "num", cfg.Num, "gl", cfg.GL)

 	resp, err := client.Search(context.Background(), &serper.SearchRequest{
 		Q:   query,
 		Num: cfg.Num,
 		GL:  cfg.GL,
 		HL:  cfg.HL,
 	})
 	if err != nil {
-		fmt.Fprintf(os.Stderr, "error: %v\n", err)
-		os.Exit(1)
+		return fmt.Errorf("search: %w", err)
 	}

 	out, err := json.MarshalIndent(resp, "", "  ")
 	if err != nil {
-		fmt.Fprintf(os.Stderr, "error formatting response: %v\n", err)
-		os.Exit(1)
+		return fmt.Errorf("format response: %w", err)
 	}
 	fmt.Println(string(out))
+	return nil
 }
```

---

### Issue 8: `SetDefaults` Uses Zero-Value Sentinel (Num=0 means "not set") [Severity: Info / Design Note]

**File:** `serper/types.go:147-160`

`SetDefaults()` treats `Num == 0` and `Page == 0` as "unset" and fills in defaults. This means a caller cannot intentionally request `Num: 0` — but since `Validate()` rejects `Num < 1`, this is consistent. However, if the Serper API ever adds a parameter where `0` is a valid value, this pattern would need adjustment.

This is documented here as a design note, not a defect.

---

## What's Done Well

1. **Request Non-Mutation:** `prepareRequest` copies the input before modifying, preventing caller side-effects. Tested explicitly.

2. **Security Posture:** `secval.ValidateJSON()` before unmarshal, response body size limit (10MB), error body truncation (1KB), API key via environment only.

3. **Doer Interface:** Clean abstraction over HTTP client enabling testing and middleware (retry via `call.Client`).

4. **GetBody for Retries:** Setting `req.GetBody` allows retry middleware to replay the request body — a detail many Go HTTP clients miss.

5. **Error Classification:** HTTP status codes mapped to semantic chassis error types, enabling callers to make decisions based on error kind.

6. **Comprehensive Test Suite:** 56+ test functions covering success paths, error paths, boundary values, context cancellation, mutation prevention, and all five endpoints.

7. **Context-Based API Key Override:** `WithAPIKey()` allows per-request key switching without creating new clients — useful for multi-tenant scenarios.

---

## Summary of Recommended Fixes

| # | Issue | Severity | Effort |
|---|---|---|---|
| 1 | Add HTTP 403 to status code mapping | Medium | 5 min |
| 2 | Wrap Validate() errors as chassiserrors.ValidationError | Low | 10 min |
| 3 | Handle or document ignored io.ReadAll error in mock | Low | 2 min |
| 4 | Extract generic endpoint helper (optional) | Low | 15 min |
| 5 | Document version gap or add v1.3.0 entry | Info | 2 min |
| 6 | Add README.md | Info | 20 min |
| 7 | Extract `run()` function in CLI for testability | Low | 15 min |
| 8 | Document zero-value sentinel design choice | Info | N/A |

---

## Test Results

```
$ go test ./...
ok   github.com/ai8future/serper_mod/cmd/serper   (cached)
ok   github.com/ai8future/serper_mod/serper        (cached)

$ go test -cover ./...
ok   github.com/ai8future/serper_mod/cmd/serper   coverage: 0.0% of statements
ok   github.com/ai8future/serper_mod/serper        coverage: 86.4% of statements

$ go vet ./...
(clean - no issues)
```

All 56+ tests pass. No `go vet` warnings.
