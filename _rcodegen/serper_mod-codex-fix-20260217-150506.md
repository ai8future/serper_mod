Date Created: 2026-02-17T15:05:06+0100
TOTAL_SCORE: 84/100

# serper_mod Quick Fix Report (Codex:gpt-5.1-codex-max-high)

## Scope
- Quick static pass on `serper/*.go` and `cmd/serper/*.go`
- Baseline checks run: `go test ./...`, `go vet ./...`
- Constraint respected: no code files were edited in the repo

## Score Breakdown
- Correctness: 31/40
- Reliability & Failure Handling: 20/25
- API/CLI ergonomics: 15/15
- Test Coverage Depth: 12/15
- Maintainability: 6/5 (strong for project size)

## Key Findings

### 1) High: nil request causes panic
- File: `serper/client.go:92`
- Root cause: `prepareRequest` dereferences `*req` without nil guard.
- Impact: direct panic when a caller accidentally passes `nil` request.
- Repro observed:
  - `go run /tmp/repro_nil_req.go`
  - output: `panic: runtime error: invalid memory address or nil pointer dereference`
- Proposed fix: explicit nil check in `prepareRequest`, plus test coverage.

### 2) High: `WithDoer(nil)` accepted, then runtime panic on first request
- File: `serper/client.go:45-67`
- Root cause: `New` does not validate `c.doer` after options are applied.
- Impact: client constructs successfully but panics during `Search`.
- Repro observed:
  - `go run /tmp/repro_nil_doer.go`
  - output: `panic: runtime error: invalid memory address or nil pointer dereference`
- Proposed fix: reject nil doer in constructor; ignore nil option functions safely.

### 3) Medium: trailing slash in base URL yields double-slash endpoint
- File: `serper/client.go:180`
- Root cause: endpoint built by raw concatenation (`c.baseURL + endpoint`).
- Impact: request URL can become `https://host//search`; often accepted, but can break strict upstream/proxy routing.
- Repro observed:
  - `go run /tmp/repro_trailing_slash.go`
  - output: `https://api.test//search`
- Proposed fix: normalize base URL with `strings.TrimRight(c.baseURL, "/")`; add regression test.

### 4) Medium: nil context path can panic in stdlib request construction
- File: `serper/client.go:174`
- Root cause: no explicit nil check before `http.NewRequestWithContext`.
- Impact: panic risk on programmer error instead of clean error return.
- Proposed fix: explicit `ctx == nil` validation with clear error.

### 5) Medium: response-size limit is not strictly enforced
- File: `serper/client.go:199`
- Root cause: reads up to exactly `maxResponseBytes`, which truncates silently and relies on later JSON failure.
- Impact: oversized bodies produce ambiguous parse/validation failures instead of clear size-limit errors.
- Proposed fix: read `maxResponseBytes+1`, fail deterministically when limit exceeded, add test.

### 6) Low-Medium: whitespace-only query currently passes validation
- File: `serper/types.go:165`
- Root cause: validation checks only `r.Q == ""`.
- Impact: invalid user input can travel to API unnecessarily.
- Repro observed:
  - `go run /tmp/repro_whitespace_query.go`
  - output: `<nil>`
- Proposed fix: `strings.TrimSpace(r.Q) == ""`, plus test case.

## Patch-Ready Diffs

```diff
diff --git a/serper/client.go b/serper/client.go
index 7286ef7..5a0aa5e 100644
--- a/serper/client.go
+++ b/serper/client.go
@@ -8,6 +8,7 @@ import (
 	"io"
 	"net/http"
 	"net/url"
+	"strings"
 	"time"
 
 	chassiserrors "github.com/ai8future/chassis-go/v5/errors"
@@ -58,11 +59,22 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 		doer:    &http.Client{Timeout: defaultTimeout},
 	}
 	for _, o := range opts {
+		if o == nil {
+			continue
+		}
 		o(c)
 	}
-	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
+	if c.doer == nil {
+		return nil, fmt.Errorf("serper: doer must not be nil")
+	}
+	parsed, err := url.ParseRequestURI(c.baseURL)
+	if err != nil {
 		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
 	}
+	if parsed.Scheme != "http" && parsed.Scheme != "https" {
+		return nil, fmt.Errorf("serper: base URL must use http or https scheme, got %q", parsed.Scheme)
+	}
+	c.baseURL = strings.TrimRight(c.baseURL, "/")
 	return c, nil
 }
 
@@ -89,6 +101,9 @@ func (c *Client) getAPIKey(ctx context.Context) string {
 // prepareRequest copies the request, applies defaults, and validates it.
 // The original request is never modified.
 func prepareRequest(req *SearchRequest) (*SearchRequest, error) {
+	if req == nil {
+		return nil, fmt.Errorf("serper: request must not be nil")
+	}
 	cp := *req
 	cp.SetDefaults()
 	if err := cp.Validate(); err != nil {
@@ -172,6 +187,10 @@ func (c *Client) CheckConnectivity(ctx context.Context) error {
 
 // doRequest performs an HTTP request to the Serper.dev API.
 func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBody any) error {
+	if ctx == nil {
+		return fmt.Errorf("serper: context must not be nil")
+	}
+
 	jsonBody, err := json.Marshal(reqBody)
 	if err != nil {
 		return fmt.Errorf("serper: marshal request: %w", err)
@@ -196,10 +215,13 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 	}
 	defer resp.Body.Close()
 
-	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
+	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
 	if err != nil {
 		return fmt.Errorf("serper: read response: %w", err)
 	}
+	if len(body) > maxResponseBytes {
+		return chassiserrors.ValidationError(fmt.Sprintf("serper: response body exceeds %d bytes", maxResponseBytes))
+	}
 
 	if resp.StatusCode >= 400 {
 		msg := string(body)
```

```diff
diff --git a/serper/types.go b/serper/types.go
index acdcda6..3bc9d23 100644
--- a/serper/types.go
+++ b/serper/types.go
@@ -1,7 +1,10 @@
 // Package serper provides a client for the Serper.dev search API.
 package serper
 
-import "fmt"
+import (
+	"fmt"
+	"strings"
+)
 
 // SearchRequest represents a search request to Serper.dev.
 type SearchRequest struct {
@@ -162,7 +165,10 @@ func (r *SearchRequest) SetDefaults() {
 // Validate checks that the SearchRequest is valid.
 // Call SetDefaults before Validate if you want zero-value fields filled in.
 func (r *SearchRequest) Validate() error {
-	if r.Q == "" {
+	if r == nil {
+		return fmt.Errorf("request must not be nil")
+	}
+	if strings.TrimSpace(r.Q) == "" {
 		return fmt.Errorf("query (q) is required")
 	}
 	if r.Num < 1 || r.Num > 100 {
```

```diff
diff --git a/serper/client_test.go b/serper/client_test.go
index e1e5b77..de920b7 100644
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -82,6 +82,16 @@ func TestNew_InvalidBaseURL(t *testing.T) {
 	}
 }
 
+func TestNew_NilDoer(t *testing.T) {
+	_, err := New("key", WithDoer(nil))
+	if err == nil {
+		t.Fatal("expected error for nil doer")
+	}
+	if !strings.Contains(strings.ToLower(err.Error()), "doer") {
+		t.Errorf("error should mention doer, got: %v", err)
+	}
+}
+
 func TestNew_WithOptions(t *testing.T) {
 	mock := &mockDoer{}
 	c := mustNew(t, "key",
@@ -137,6 +147,24 @@ func TestSearch_Success(t *testing.T) {
 	}
 }
 
+func TestSearch_NormalizesTrailingSlashBaseURL(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "my-api-key", WithDoer(mock), WithBaseURL("https://api.test/"))
+
+	_, err := c.Search(context.Background(), &SearchRequest{Q: "golang"})
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	if mock.req == nil {
+		t.Fatal("expected request to be captured")
+	}
+	wantURL := "https://api.test/search"
+	if mock.req.URL.String() != wantURL {
+		t.Errorf("URL: got %q, want %q", mock.req.URL.String(), wantURL)
+	}
+}
+
 func TestSearch_HTTPError(t *testing.T) {
 	mock := &mockDoer{statusCode: 401, respBody: `{"error":"unauthorized"}`}
 	c := mustNew(t, "bad-key", WithDoer(mock))
@@ -291,6 +319,22 @@ func TestSearch_ValidationError(t *testing.T) {
 	}
 }
 
+func TestSearch_NilRequest(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Search(context.Background(), nil)
+	if err == nil {
+		t.Fatal("expected validation error for nil request")
+	}
+	if !strings.Contains(strings.ToLower(err.Error()), "request") {
+		t.Errorf("error should mention request, got: %v", err)
+	}
+	if mock.req != nil {
+		t.Error("no HTTP request should have been made for nil input")
+	}
+}
+
 func TestSearch_AppliesDefaults(t *testing.T) {
 	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
 	c := mustNew(t,"key", WithDoer(mock))
@@ -352,6 +396,12 @@ func TestSearchRequest_Validate(t *testing.T) {
 			wantErr: true,
 			errMsg:  "query",
 		},
+		{
+			name:    "whitespace query",
+			req:     SearchRequest{Q: "   ", Num: 10, Page: 1},
+			wantErr: true,
+			errMsg:  "query",
+		},
 		{
 			name:    "num zero",
 			req:     SearchRequest{Q: "test", Num: 0, Page: 1},
@@ -490,6 +540,22 @@ func TestSearch_ContextCancellation(t *testing.T) {
 	}
 }
 
+func TestSearch_NilContext(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Search(nil, &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for nil context")
+	}
+	if !strings.Contains(strings.ToLower(err.Error()), "context") {
+		t.Errorf("error should mention context, got: %v", err)
+	}
+	if mock.req != nil {
+		t.Error("request should not be sent when context is nil")
+	}
+}
+
 func TestSearch_MalformedJSONResponse(t *testing.T) {
 	mock := &mockDoer{statusCode: 200, respBody: `{not json`}
 	c := mustNew(t, "key", WithDoer(mock))
@@ -504,6 +570,20 @@ func TestSearch_MalformedJSONResponse(t *testing.T) {
 	}
 }
 
+func TestSearch_ResponseTooLarge(t *testing.T) {
+	largeBody := strings.Repeat("x", maxResponseBytes+20)
+	mock := &mockDoer{statusCode: 200, respBody: largeBody}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for oversized response body")
+	}
+	if !strings.Contains(err.Error(), "exceeds") {
+		t.Errorf("error should mention size limit, got: %v", err)
+	}
+}
+
 func TestSearch_ErrorBodyTruncation(t *testing.T) {
 	// Build an error body larger than maxErrorBodyBytes (1024).
 	longBody := strings.Repeat("x", 2000)
```

## Notes
- This repo currently has strong baseline tests and clear API/client separation.
- Most score deductions are from panic paths and edge-case input/limit handling.
- No source code modifications were made in the repository per request.

## Suggested Validation After Applying Patch
1. `go test ./...`
2. `go vet ./...`
3. Manual sanity:
   - `WithDoer(nil)` now errors during `New(...)`
   - `Search(context.Background(), nil)` returns error instead of panic
   - `WithBaseURL("https://api.test/")` hits `https://api.test/search` (single slash)
