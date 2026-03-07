Date Created: 2026-02-17T21:00:00-05:00
TOTAL_SCORE: 78/100

# serper_mod Combined Analysis Report

**Module:** `github.com/ai8future/serper_mod`
**Go Version:** 1.25.5
**Chassis Dependency:** `chassis-go/v5 v5.0.0`
**Files Analyzed:** `serper/client.go`, `serper/types.go`, `cmd/serper/main.go`, `serper/client_test.go`, `cmd/serper/main_test.go`

---

## Scoring Breakdown

| Category | Score | Max | Notes |
|----------|-------|-----|-------|
| Security | 16 | 20 | Good API key handling, response capping; missing query length limit, secval false-positive risk |
| Code Quality | 18 | 25 | Clean patterns, good DI; inconsistent error typing, missing HTTP 403 mapping |
| Test Coverage | 16 | 25 | 86.4% serper pkg; 0% cmd/main; no error-path tests for Images/News/Places/Scholar |
| Architecture | 15 | 15 | Clean functional options, Doer interface, non-mutating request handling |
| Documentation | 8 | 10 | Good Go doc comments; missing CLI endpoint flag docs |
| Build/Config | 5 | 5 | Proper go.mod, .gitignore (missing *.out glob) |

---

## 1. AUDIT - Security and Code Quality Issues

### AUDIT-1: `secval.ValidateJSON` on API responses may reject legitimate results (Severity: Medium)

`secval.ValidateJSON` is designed for user-supplied input. Applying it to Serper.dev API responses can cause false-positive rejections. The `KnowledgeGraph.Attributes` field is `map[string]string` — its keys come from real web data. If a knowledge graph entry has an attribute named `"command"`, `"system"`, `"script"`, `"import"`, or any of the other secval-forbidden keys, the entire valid response is rejected with a `ValidationError`.

**Impact:** Legitimate search results could be silently rejected, causing search failures for certain queries.

**File:** `serper/client.go:226-228`

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -223,10 +223,6 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 		}
 	}

-	if err := secval.ValidateJSON(body); err != nil {
-		return chassiserrors.ValidationError(fmt.Sprintf("serper: unsafe response body: %v", err))
-	}
-
 	if err := json.Unmarshal(body, respBody); err != nil {
 		return fmt.Errorf("serper: unmarshal response: %w", err)
 	}
```

> **Note:** If you want to keep defense-in-depth, move the secval check to only validate user-controlled inputs (the `SearchRequest`) rather than trusted API responses. Or configure secval to use a more lenient key allowlist for responses.

---

### AUDIT-2: No query string length limit (Severity: Low)

`SearchRequest.Q` has no length limit. A caller could pass a multi-megabyte string as a query. While `json.Marshal` handles encoding safely, the oversized payload would be sent to Serper.dev, consuming bandwidth and potentially triggering a 400 error with a large error body read into memory.

**File:** `serper/types.go:164-175`

```diff
--- a/serper/types.go
+++ b/serper/types.go
@@ -162,6 +162,8 @@ func (r *SearchRequest) SetDefaults() {
 // Call SetDefaults before Validate if you want zero-value fields filled in.
 func (r *SearchRequest) Validate() error {
 	if r.Q == "" {
 		return fmt.Errorf("query (q) is required")
+	} else if len(r.Q) > 2048 {
+		return fmt.Errorf("query (q) must be 2048 characters or fewer")
 	}
 	if r.Num < 1 || r.Num > 100 {
```

---

### AUDIT-3: HTTP 403 Forbidden falls through to `InternalError` (Severity: Low)

A 403 from Serper.dev (e.g., plan-tier quota exceeded) is mapped to `InternalError` instead of a more appropriate error type. Callers cannot distinguish "forbidden" from "internal server error."

**File:** `serper/client.go:210-223`

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -213,6 +213,8 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 			return chassiserrors.ValidationError(detail)
 		case http.StatusUnauthorized:
 			return chassiserrors.UnauthorizedError(detail)
+		case http.StatusForbidden:
+			return chassiserrors.ForbiddenError(detail)
 		case http.StatusNotFound:
 			return chassiserrors.NotFoundError(detail)
 		case http.StatusTooManyRequests:
```

---

### AUDIT-4: Error response reads full 10MB before truncating to 1KB (Severity: Low)

When the API returns a 4xx/5xx error, the full body (up to 10MB) is read into memory, then truncated to 1024 bytes for the error message. For error responses, the body could use a smaller limit.

**File:** `serper/client.go:199-208`

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -196,7 +196,12 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 	defer resp.Body.Close()

-	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
+	// Use a smaller read limit for error responses to avoid reading huge bodies
+	// just to truncate them in the error message.
+	readLimit := int64(maxResponseBytes)
+	if resp.StatusCode >= 400 {
+		readLimit = int64(maxErrorBodyBytes) + 1 // +1 to detect truncation
+	}
+	body, err := io.ReadAll(io.LimitReader(resp.Body, readLimit))
 	if err != nil {
 		return fmt.Errorf("serper: read response: %w", err)
 	}
```

---

### AUDIT-5: Coverage artifacts not gitignored (Severity: Info)

`coverage.out`, `coverage_serper.out`, `coverage_cmd.out`, and `cmd_coverage.out` are present in the repo root but not gitignored. The `.gitignore` ignores `*.test` but not `*.out`.

**File:** `.gitignore`

```diff
--- a/.gitignore
+++ b/.gitignore
@@ -27,6 +27,7 @@
 *.exe
 *.test
 .gocache/
+*.out
 go.work
 go.work.sum
```

---

## 2. TESTS - Proposed Unit Tests

### TEST-1: Error paths for Images, News, Places, Scholar endpoints

Currently only the success path is tested for these four methods. The error branches (HTTP error and doer error) are untested, leaving them at 71.4% coverage.

**File:** `serper/client_test.go` (append)

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -725,3 +725,75 @@ func TestNew_LastOptionWins(t *testing.T) {
 		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://second.com")
 	}
 }
+
+func TestImages_HTTPError(t *testing.T) {
+	mock := &mockDoer{statusCode: 401, respBody: `{"error":"unauthorized"}`}
+	c := mustNew(t, "bad-key", WithDoer(mock))
+
+	_, err := c.Images(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for 401 status")
+	}
+	if !strings.Contains(err.Error(), "401") {
+		t.Errorf("error should contain '401', got: %v", err)
+	}
+}
+
+func TestImages_DoerError(t *testing.T) {
+	mock := &mockDoer{err: fmt.Errorf("network error")}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Images(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error from doer")
+	}
+}
+
+func TestNews_HTTPError(t *testing.T) {
+	mock := &mockDoer{statusCode: 429, respBody: `{"error":"rate limit"}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.News(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for 429 status")
+	}
+	if !strings.Contains(err.Error(), "429") {
+		t.Errorf("error should contain '429', got: %v", err)
+	}
+}
+
+func TestNews_ValidationError(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"news":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.News(context.Background(), &SearchRequest{Q: ""})
+	if err == nil {
+		t.Fatal("expected validation error for empty query")
+	}
+	if mock.req != nil {
+		t.Error("no HTTP request should have been made for invalid input")
+	}
+}
+
+func TestPlaces_HTTPError(t *testing.T) {
+	mock := &mockDoer{statusCode: 400, respBody: `{"error":"bad request"}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Places(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for 400 status")
+	}
+	if !strings.Contains(err.Error(), "400") {
+		t.Errorf("error should contain '400', got: %v", err)
+	}
+}
+
+func TestScholar_HTTPError(t *testing.T) {
+	mock := &mockDoer{statusCode: 502, respBody: `bad gateway`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Scholar(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for 502 status")
+	}
+	if !strings.Contains(err.Error(), "502") {
+		t.Errorf("error should contain '502', got: %v", err)
+	}
+}
+
+func TestScholar_ValidationError(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Scholar(context.Background(), &SearchRequest{Q: "", Num: 10, Page: 1})
+	if err == nil {
+		t.Fatal("expected validation error for empty query")
+	}
+}
```

---

### TEST-2: HTTP status code coverage for 400, 404, 429, 502

Currently only 401, 500, and 503 are tested. Other mapped status codes have no tests.

**File:** `serper/client_test.go` (append)

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -800,3 +800,47 @@ func TestScholar_ValidationError(t *testing.T) {
 	}
 }
+
+func TestSearch_HTTPStatusCodes(t *testing.T) {
+	tests := []struct {
+		name       string
+		statusCode int
+		wantSubstr string
+	}{
+		{"400 Bad Request", 400, "400"},
+		{"404 Not Found", 404, "404"},
+		{"429 Too Many Requests", 429, "429"},
+		{"502 Bad Gateway", 502, "502"},
+	}
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			mock := &mockDoer{statusCode: tt.statusCode, respBody: `{"error":"test"}`}
+			c := mustNew(t, "key", WithDoer(mock))
+
+			_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+			if err == nil {
+				t.Fatalf("expected error for status %d", tt.statusCode)
+			}
+			if !strings.Contains(err.Error(), tt.wantSubstr) {
+				t.Errorf("error should contain %q, got: %v", tt.wantSubstr, err)
+			}
+		})
+	}
+}
```

---

### TEST-3: Validate negative Num value

`Validate` checks `Num < 1` but there's no test for negative Num.

**File:** `serper/client_test.go` (append)

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -847,3 +847,12 @@ func TestSearch_HTTPStatusCodes(t *testing.T) {
 }
+
+func TestValidate_NegativeNum(t *testing.T) {
+	r := SearchRequest{Q: "test", Num: -5, Page: 1}
+	err := r.Validate()
+	if err == nil {
+		t.Fatal("expected error for negative Num")
+	}
+	if !strings.Contains(strings.ToLower(err.Error()), "num") {
+		t.Errorf("error should mention 'num', got: %v", err)
+	}
+}
```

---

### TEST-4: `WithAPIKey` context override used with non-Search methods

Context API key override is only tested via `Search`. Should be verified with at least one other endpoint.

**File:** `serper/client_test.go` (append)

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -859,3 +859,16 @@ func TestValidate_NegativeNum(t *testing.T) {
 }
+
+func TestImages_WithAPIKeyOverride(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"images":[]}`}
+	c := mustNew(t, "default-key", WithDoer(mock))
+
+	ctx := WithAPIKey(context.Background(), "override-key")
+	_, _ = c.Images(ctx, &SearchRequest{Q: "test"})
+
+	if mock.req == nil {
+		t.Fatal("expected request to be captured")
+	}
+	if got := mock.req.Header.Get("X-API-KEY"); got != "override-key" {
+		t.Errorf("X-API-KEY: got %q, want %q", got, "override-key")
+	}
+}
```

---

## 3. FIXES - Bugs, Issues, and Code Smells

### FIX-1: `Validate()` returns plain `error` instead of typed `*ServiceError` (Inconsistency)

`Search()` and friends return typed `*ServiceError` for HTTP-level validation issues but plain `fmt.Errorf` for local validation. Callers using `errors.As(err, &chassiserrors.ServiceError{})` to detect validation problems will miss locally-rejected requests.

**File:** `serper/types.go:164-175`

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
@@ -162,15 +164,15 @@
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

> **Note:** This would require updating tests that check `err.Error()` directly, since `*ServiceError` wraps the message differently.

---

### FIX-2: CLI only exposes `Search` endpoint — no way to use Images/News/Places/Scholar

The CLI binary hardcodes `client.Search()`. Users cannot access the other four endpoints from the command line, even though the library supports them.

**File:** `cmd/serper/main.go`

```diff
--- a/cmd/serper/main.go
+++ b/cmd/serper/main.go
@@ -19,8 +19,9 @@
 // Config holds CLI configuration loaded from environment.
 type Config struct {
-	APIKey   string        `env:"SERPER_API_KEY" required:"true"`
-	BaseURL  string        `env:"SERPER_BASE_URL" default:"https://google.serper.dev"`
+	APIKey   string        `env:"SERPER_API_KEY"   required:"true"`
+	BaseURL  string        `env:"SERPER_BASE_URL"  default:"https://google.serper.dev"`
+	Type     string        `env:"SERPER_TYPE"      default:"search"`
 	Num      int           `env:"SERPER_NUM" default:"10"`
 	GL       string        `env:"SERPER_GL" default:"us"`
 	HL       string        `env:"SERPER_HL" default:"en"`
@@ -56,8 +57,22 @@

 	logger.Debug("searching", "query", query, "num", cfg.Num, "gl", cfg.GL)

-	resp, err := client.Search(context.Background(), &serper.SearchRequest{
-		Q:   query,
+	req := &serper.SearchRequest{
+		Q:   query,
 		Num: cfg.Num,
 		GL:  cfg.GL,
 		HL:  cfg.HL,
-	})
+	}
+
+	var resp any
+	switch cfg.Type {
+	case "search":
+		resp, err = client.Search(context.Background(), req)
+	case "images":
+		resp, err = client.Images(context.Background(), req)
+	case "news":
+		resp, err = client.News(context.Background(), req)
+	case "places":
+		resp, err = client.Places(context.Background(), req)
+	case "scholar":
+		resp, err = client.Scholar(context.Background(), req)
+	default:
+		fmt.Fprintf(os.Stderr, "error: unknown search type %q (valid: search, images, news, places, scholar)\n", cfg.Type)
+		os.Exit(1)
+	}
 	if err != nil {
```

---

### FIX-3: `defaultTimeout` constant is declared but unused in the library

The `defaultTimeout` constant (line 19) is only used to set the default `http.Client` timeout in `New()`. However, if a caller provides `WithDoer()`, the constant is completely ignored. This is correct behavior, but the constant name is misleading — it's the default *http.Client* timeout, not a request-level timeout. This is cosmetic, not a bug.

No diff needed — just a documentation note.

---

### FIX-4: `CheckConnectivity` consumes API credits

`CheckConnectivity` performs a real search (`Q: "test", Num: 1`), consuming API credits. There's no documentation in the exported Go doc about this cost beyond the code comment.

**File:** `serper/client.go:165-171`

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -165,7 +165,8 @@
-// CheckConnectivity verifies the API key and connectivity to Serper.dev.
-// Note: this makes a real search request that counts toward your API usage.
+// CheckConnectivity verifies the API key and network connectivity to Serper.dev
+// by performing a minimal search. This makes a real API call that counts toward
+// your Serper.dev usage quota.
 func (c *Client) CheckConnectivity(ctx context.Context) error {
```

---

## 4. REFACTOR - Opportunities to Improve Code Quality

### REFACTOR-1: Extract endpoint method boilerplate into a generic helper

`Search`, `Images`, `News`, `Places`, and `Scholar` all follow the exact same pattern:

```go
prepared, err := prepareRequest(req)
if err != nil { return nil, err }
var resp T
if err := c.doRequest(ctx, endpoint, prepared, &resp); err != nil { return nil, err }
return &resp, nil
```

With Go generics (Go 1.18+), this could be a single private function:

```go
func doSearch[T any](c *Client, ctx context.Context, endpoint string, req *SearchRequest) (*T, error) {
    prepared, err := prepareRequest(req)
    if err != nil { return nil, err }
    var resp T
    if err := c.doRequest(ctx, endpoint, prepared, &resp); err != nil { return nil, err }
    return &resp, nil
}
```

Each public method becomes a one-liner. This eliminates 5x code duplication and makes adding new endpoints trivial.

**Priority:** Low — the current code is clear and working. Only worthwhile if more endpoints are added.

---

### REFACTOR-2: Table-driven endpoint tests

`TestImages_Success`, `TestNews_Success`, `TestPlaces_Success`, `TestScholar_Success` all follow the same pattern with different JSON and endpoint URLs. These could be consolidated into a single table-driven test, reducing ~100 lines to ~40 lines.

**Priority:** Low — the current tests are readable and work fine.

---

### REFACTOR-3: Move `prepareRequest` to a method on `*Client`

`prepareRequest` is a package-level function, not a method on `*Client`. While it doesn't currently need client state, making it a method would be more consistent with Go conventions and allow future extension (e.g., client-level default overrides).

**Priority:** Very Low — current design is fine.

---

### REFACTOR-4: Add `*.out` to `.gitignore`

Coverage output files (`coverage.out`, `coverage_serper.out`, etc.) accumulate in the repo root. Adding `*.out` to `.gitignore` prevents them from showing up in `git status`. (See AUDIT-5 for the diff.)

**Priority:** Low — cosmetic cleanup.

---

### REFACTOR-5: CLI `main()` testability

`main()` is 0% covered because it directly calls `os.Exit()` and reads from `os.Args`. Extracting the logic into a `run(args []string) error` function would allow testing the full CLI flow without process-level testing.

```go
func main() {
    if err := run(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}

func run(args []string) error {
    // ... existing main() logic, returning errors instead of os.Exit
}
```

**Priority:** Medium — 0% CLI coverage is a real gap.

---

*Report generated by Claude Opus 4.6 on 2026-02-17.*
