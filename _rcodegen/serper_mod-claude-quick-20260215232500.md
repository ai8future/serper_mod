Date Created: 2026-02-15T23:25:00-05:00
TOTAL_SCORE: 88/100

# serper_mod Quick Analysis Report

**Module:** `github.com/ai8future/serper_mod`
**Version:** 1.4.0 | **Go:** 1.25.5 | **Chassis:** v5.0.0
**Test Coverage:** 86.4% (serper pkg) | 0% (cmd pkg)
**Files:** 5 Go files (~1,260 lines)

---

## 1. AUDIT — Security and Code Quality Issues

### AUDIT-1: No Num negative value validation (Severity: Low)

`Validate()` checks `Num < 1 || Num > 100`, but `SetDefaults()` only fills in when `Num == 0`. A caller passing `Num: -5` would fail validation correctly, but the error message "num must be between 1 and 100" is slightly misleading since negative numbers aren't really "between" anything. Minor UX issue only — no security impact.

**Verdict:** Informational only. Current behavior is correct (validation rejects negative Num).

### AUDIT-2: API key logged at Debug level in CLI (Severity: Low)

In `cmd/serper/main.go:57`, the logger logs query and config parameters at Debug level. The API key itself is NOT logged (only query, num, gl), so this is clean. However, the `cfg` struct contains the API key and care should be taken that it's never passed wholesale to a logger.

**Verdict:** No issue currently, but worth noting as a defensive point.

### AUDIT-3: WithBaseURL allows any URL scheme (Severity: Low)

`url.ParseRequestURI` accepts schemes beyond https (e.g., `http://`, `ftp://`). For a production API client, allowing non-HTTPS connections could expose the API key in transit.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -60,6 +60,12 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 	for _, o := range opts {
 		o(c)
 	}
+	parsed, err := url.ParseRequestURI(c.baseURL)
+	if err != nil {
+		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
+	}
+	if parsed.Scheme != "https" {
+		return nil, fmt.Errorf("serper: base URL must use https scheme, got %q", parsed.Scheme)
+	}
-	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
-		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
-	}
 	return c, nil
 }
```

**Impact:** Prevents accidental API key leakage over unencrypted connections. Note: this would require updating tests that use `http://` mock URLs — could be made opt-out if test flexibility is needed.

### AUDIT-4: No `Content-Length` header set explicitly (Severity: Informational)

The `http.NewRequestWithContext` with a `bytes.Reader` body will set Content-Length automatically via `bytes.Reader.Len()`. No action needed — Go's HTTP client handles this correctly.

**Verdict:** No issue.

### AUDIT-5: HTTP 403 Forbidden falls through to `InternalError` (Severity: Low)

The status code switch in `doRequest` doesn't handle 403 (Forbidden). If Serper.dev returns 403 (e.g., IP blocked, account suspended), it would be classified as `InternalError` rather than a more appropriate `UnauthorizedError` or `ForbiddenError`.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -210,6 +210,8 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 		switch resp.StatusCode {
 		case http.StatusBadRequest:
 			return chassiserrors.ValidationError(detail)
-		case http.StatusUnauthorized:
+		case http.StatusUnauthorized, http.StatusForbidden:
 			return chassiserrors.UnauthorizedError(detail)
 		case http.StatusNotFound:
```

---

## 2. TESTS — Proposed Unit Tests for Untested Code

### TEST-1: Test `Validate()` with negative Num

Currently no test for `Num: -1`. The boundary tests cover `Num=0`, `Num=1`, `Num=100`, `Num=101` but not negative values.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -595,6 +595,11 @@ func TestValidate_BoundaryValues(t *testing.T) {
 			req:     SearchRequest{Q: "test", Num: 10, Page: 1},
 			wantErr: false,
 		},
+		{
+			name:    "num=-1 (negative)",
+			req:     SearchRequest{Q: "test", Num: -1, Page: 1},
+			wantErr: true,
+		},
 	}
```

### TEST-2: Test `WithBaseURL` with empty string

No test for what happens when `WithBaseURL("")` is passed. This would hit the `url.ParseRequestURI` validation, but it's worth having an explicit test.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -83,6 +83,15 @@ func TestNew_InvalidBaseURL(t *testing.T) {
 	}
 }

+func TestNew_EmptyBaseURL(t *testing.T) {
+	_, err := New("key", WithBaseURL(""))
+	if err == nil {
+		t.Fatal("expected error for empty base URL")
+	}
+	if !strings.Contains(err.Error(), "invalid base URL") {
+		t.Errorf("error should mention 'invalid base URL', got: %v", err)
+	}
+}
```

### TEST-3: Test `Images`, `News`, `Places`, `Scholar` with validation errors

The endpoint-specific tests only cover success paths. There's no test confirming that validation errors propagate correctly through `Images()`, `News()`, etc. (only `Search()` has `TestSearch_ValidationError`).

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -725,3 +725,33 @@ func TestNew_LastOptionWins(t *testing.T) {
 	}
 }
+
+func TestEndpoints_ValidationError(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{}`}
+	c := mustNew(t, "key", WithDoer(mock))
+	badReq := &SearchRequest{Q: ""}
+
+	if _, err := c.Images(context.Background(), badReq); err == nil {
+		t.Error("Images: expected validation error for empty query")
+	}
+	if _, err := c.News(context.Background(), badReq); err == nil {
+		t.Error("News: expected validation error for empty query")
+	}
+	if _, err := c.Places(context.Background(), badReq); err == nil {
+		t.Error("Places: expected validation error for empty query")
+	}
+	if _, err := c.Scholar(context.Background(), badReq); err == nil {
+		t.Error("Scholar: expected validation error for empty query")
+	}
+
+	if mock.req != nil {
+		t.Error("no HTTP request should be made for invalid input on any endpoint")
+	}
+}
```

### TEST-4: Test HTTP 400, 404, 429 error mapping

The test suite covers 401, 500, 503 but is missing explicit tests for 400 (Bad Request), 404 (Not Found), and 429 (Rate Limit) status codes — all of which have specific error type mappings in `doRequest`.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -725,3 +725,53 @@ func TestNew_LastOptionWins(t *testing.T) {
 	}
 }
+
+func TestSearch_HTTPStatusErrorMapping(t *testing.T) {
+	tests := []struct {
+		name       string
+		statusCode int
+		wantSubstr string
+	}{
+		{"400 Bad Request", 400, "400"},
+		{"404 Not Found", 404, "404"},
+		{"429 Rate Limit", 429, "429"},
+		{"502 Bad Gateway", 502, "502"},
+	}
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			mock := &mockDoer{statusCode: tt.statusCode, respBody: `{"error":"test"}`}
+			c := mustNew(t, "key", WithDoer(mock))
+
+			_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+			if err == nil {
+				t.Fatalf("expected error for HTTP %d", tt.statusCode)
+			}
+			if !strings.Contains(err.Error(), tt.wantSubstr) {
+				t.Errorf("error should contain %q, got: %v", tt.wantSubstr, err)
+			}
+		})
+	}
+}
```

### TEST-5: Test `CheckConnectivity` propagates request parameters correctly

Verify that `CheckConnectivity` sends `Q: "test"` and `Num: 1` (not defaults).

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -625,6 +625,19 @@ func TestCheckConnectivity_Success(t *testing.T) {
 	if mock.req == nil {
 		t.Fatal("expected request to be made")
 	}
+
+	// Verify it sends a minimal request.
+	var sent SearchRequest
+	if err := json.Unmarshal(mock.body, &sent); err != nil {
+		t.Fatalf("unmarshal request body: %v", err)
+	}
+	if sent.Q != "test" {
+		t.Errorf("Q: got %q, want %q", sent.Q, "test")
+	}
+	if sent.Num != 1 {
+		t.Errorf("Num: got %d, want 1", sent.Num)
+	}
 }
```

### TEST-6: Test `nil` request handling

No test for what happens when `nil` is passed as `*SearchRequest`. Currently `prepareRequest` would panic on `cp := *req` dereferencing a nil pointer. Consider a nil guard or at least document the contract.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -725,3 +725,17 @@ func TestNew_LastOptionWins(t *testing.T) {
 	}
 }
+
+func TestSearch_NilRequest(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	defer func() {
+		if r := recover(); r == nil {
+			t.Fatal("expected panic for nil request")
+		}
+	}()
+
+	// Passing nil should panic (documents current behavior).
+	_, _ = c.Search(context.Background(), nil)
+}
```

---

## 3. FIXES — Bugs, Issues, and Code Smells

### FIX-1: `WithDoer(nil)` silently accepted, causes nil pointer panic later (Severity: Medium)

If a caller passes `WithDoer(nil)`, the client is constructed successfully but will panic on the first API call when `c.doer.Do(req)` is called. The constructor should validate that doer is non-nil.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -60,6 +60,9 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 	for _, o := range opts {
 		o(c)
 	}
+	if c.doer == nil {
+		return nil, fmt.Errorf("serper: doer must not be nil")
+	}
 	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
 		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
 	}
```

### FIX-2: `nil` SearchRequest causes panic — no nil guard (Severity: Low)

`prepareRequest` dereferences the pointer `cp := *req` without checking for nil. While callers _should_ never pass nil, a defensive nil check gives a better error message than a raw panic.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -91,6 +91,9 @@ func prepareRequest(req *SearchRequest) (*SearchRequest, error) {
+	if req == nil {
+		return nil, fmt.Errorf("serper: request must not be nil")
+	}
 	cp := *req
 	cp.SetDefaults()
```

### FIX-3: `CheckConnectivity` makes a billable request — no obvious alternative (Severity: Informational)

The godoc correctly warns about this, so it's documented. However, the method name "CheckConnectivity" might mislead callers into thinking it's a free health-check. Consider renaming to `Ping` or `VerifyAPIKey` to make the cost more obvious. No diff — naming suggestion only.

### FIX-4: Error messages inconsistent — some prefixed with "serper:", validation errors are not (Severity: Low)

`Validate()` returns errors like `"query (q) is required"` and `"num must be between 1 and 100"` without the `"serper:"` prefix that all other errors use. This inconsistency makes it harder to identify error origins in logs.

```diff
--- a/serper/types.go
+++ b/serper/types.go
@@ -164,11 +164,11 @@ func (r *SearchRequest) Validate() error {
 	if r.Q == "" {
-		return fmt.Errorf("query (q) is required")
+		return fmt.Errorf("serper: query (q) is required")
 	}
 	if r.Num < 1 || r.Num > 100 {
-		return fmt.Errorf("num must be between 1 and 100")
+		return fmt.Errorf("serper: num must be between 1 and 100")
 	}
 	if r.Page < 1 {
-		return fmt.Errorf("page must be 1 or greater")
+		return fmt.Errorf("serper: page must be 1 or greater")
 	}
```

### FIX-5: `SetDefaults` uses `Num == 0` which conflicts with `omitempty` (Severity: Informational)

`SearchRequest.Num` has the JSON tag `omitempty`, meaning a zero value won't be serialized. `SetDefaults` checks `Num == 0` to apply the default. This works correctly because `prepareRequest` applies defaults before marshaling. However, it means callers cannot intentionally send `Num: 0` to the API (if they ever needed to). Since the API requires `Num >= 1`, this is a non-issue — just worth documenting.

**Verdict:** No action needed. Behavior is correct for the Serper.dev API contract.

---

## 4. REFACTOR — Opportunities to Improve Code Quality

### REFACTOR-1: Extract endpoint methods to reduce repetition

The five endpoint methods (`Search`, `Images`, `News`, `Places`, `Scholar`) are nearly identical — they all call `prepareRequest` then `doRequest` with different paths and response types. A generic helper could reduce this to a single implementation:

```go
func callEndpoint[T any](c *Client, ctx context.Context, path string, req *SearchRequest) (*T, error) {
    prepared, err := prepareRequest(req)
    if err != nil { return nil, err }
    var resp T
    if err := c.doRequest(ctx, path, prepared, &resp); err != nil { return nil, err }
    return &resp, nil
}
```

Each public method becomes a one-liner. This eliminates 5x copy-paste and reduces the chance of divergence. Tradeoff: generics add a tiny readability cost for Go newcomers.

### REFACTOR-2: Consider typed errors for validation failures

Currently `Validate()` returns plain `fmt.Errorf` strings. Since the codebase already uses `chassiserrors` for typed errors, validation failures could use `chassiserrors.ValidationError()` for consistency. This would allow callers to use `errors.As` or type-switch on the error type.

### REFACTOR-3: Consider `GL` and `HL` validation

`GL` and `HL` accept any string without validation. While the Serper.dev API will ultimately reject invalid locale codes, client-side validation (e.g., checking for 2-letter ISO codes) could provide faster feedback and better error messages.

### REFACTOR-4: `Location` field unused in defaults

`SearchRequest.Location` is declared with `omitempty` and has no default. This is fine — location is optional. But if there are plans to support geo-targeted defaults, `SetDefaults` would be the place.

### REFACTOR-5: cmd/serper could support all endpoints

The CLI only calls `Search`. It could accept an `--endpoint` flag to support Images, News, Places, and Scholar. Low priority but would make the CLI more useful as a testing tool.

### REFACTOR-6: Consider `context.WithTimeout` in CLI

`main.go:61` uses `context.Background()` with a comment explaining that `call.Client` handles timeouts. Adding a top-level context timeout as a safety net would defend against pathological retry scenarios where all retries individually succeed but the total wall-clock time is unacceptable.

---

## Score Breakdown

| Category | Score | Max | Notes |
|----------|-------|-----|-------|
| **Security** | 18 | 20 | Excellent: secval, response limits, no secrets in logs. Minor: allows non-HTTPS base URLs. |
| **Error Handling** | 16 | 20 | Strong typed errors, proper wrapping. Minor: inconsistent prefixing, missing 403 mapping. |
| **Test Coverage** | 17 | 20 | 86.4% coverage, comprehensive scenarios. Gaps: no negative Num test, no endpoint-specific validation tests, no nil request test. |
| **Code Quality** | 18 | 20 | Clean, idiomatic Go. Functional options, copy-on-write. Minor: repetitive endpoint methods, no nil doer guard. |
| **Architecture** | 19 | 20 | Well-structured: clean separation of types/client/CLI, interface-based injection, context-based overrides. |
| **TOTAL** | **88** | **100** | |

**Summary:** This is a well-engineered, security-conscious Go library. The main areas for improvement are defensive nil checks (FIX-1, FIX-2), error message consistency (FIX-4), and expanding test coverage for edge cases and non-Search endpoints. The codebase demonstrates mature patterns (functional options, copy-on-write, typed errors, response size limits) that are above average for a library of this size.
