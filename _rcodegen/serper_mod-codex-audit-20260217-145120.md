Date Created: 2026-02-17 14:51:20 CET (+0100)
TOTAL_SCORE: 82/100

# serper_mod Audit Report (Codex:gpt-5.1-codex-max-high)

## Scope
- Reviewed all production and test code in:
  - `serper/client.go`
  - `serper/types.go`
  - `serper/client_test.go`
  - `cmd/serper/main.go`
  - `cmd/serper/main_test.go`
- Ran:
  - `go test ./...` (pass)
  - `go vet ./...` (pass)
- Tooling gap during this audit:
  - `govulncheck` not installed
  - `staticcheck` not installed

## Score Breakdown (100-point scale)
- Correctness: 18/20
- Security: 16/25
- Reliability/Resilience: 15/20
- Test Coverage Quality: 18/20
- Maintainability/Readability: 15/15
- **Total**: **82/100**

## Findings (highest severity first)

### 1) High: nil request can panic the client
- Location: `serper/client.go:91`
- Detail: `prepareRequest` dereferences `req` (`cp := *req`) without a nil check.
- Impact: A caller passing `nil` can crash the process (denial-of-service in long-running services embedding this library).
- Recommendation: Return a validation error when `req == nil`; add regression tests.

### 2) Medium-High (Security): API key can be sent over plaintext HTTP
- Location: `serper/client.go:63`, `serper/client.go:186`
- Detail: `New` validates URI syntax only; it allows `http://...`. `doRequest` always sends `X-API-KEY`.
- Impact: Accidental misconfiguration can leak credentials over unencrypted transport.
- Recommendation: Enforce `https` by default, with optional loopback exceptions (`localhost`, `127.0.0.1`, `::1`) for local testing.

### 3) Medium: nil `Doer` accepted, then panics at request time
- Location: `serper/client.go:45`, `serper/client.go:193`
- Detail: `WithDoer(nil)` is accepted and stored; later `c.doer.Do(req)` panics.
- Impact: Runtime crash in callers that inject transport dynamically.
- Recommendation: Validate `c.doer != nil` in `New`.

### 4) Medium (Security/Observability): untrusted error body included unsanitized in error text
- Location: `serper/client.go:205-210`
- Detail: Response body is truncated, but newline/control characters are not normalized before embedding in error text.
- Impact: Log injection or multiline log confusion in downstream logging pipelines.
- Recommendation: sanitize to single-line printable output before composing errors.

## Positive Notes
- Good input validation boundaries in `SearchRequest.Validate`.
- Default timeout exists on default HTTP client (`30s`).
- Response size is bounded (`10MB`) and error body is truncated (`1KB`).
- Strong unit test coverage across happy paths and many edge cases.

## Patch-Ready Diffs

### Diff A: Harden client input and transport security
```diff
diff --git a/serper/client.go b/serper/client.go
--- a/serper/client.go
+++ b/serper/client.go
@@ -6,8 +6,10 @@ import (
 	"encoding/json"
 	"fmt"
 	"io"
+	"net"
 	"net/http"
 	"net/url"
+	"strings"
 	"time"
 
 	chassiserrors "github.com/ai8future/chassis-go/v5/errors"
@@ -60,13 +62,27 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 	for _, o := range opts {
 		o(c)
 	}
-	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
+	u, err := url.ParseRequestURI(c.baseURL)
+	if err != nil {
 		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
 	}
+	if c.doer == nil {
+		return nil, fmt.Errorf("serper: doer must not be nil")
+	}
+	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
+		return nil, fmt.Errorf("serper: base URL must use https unless host is loopback")
+	}
 	return c, nil
 }
+
+func isLoopbackHost(host string) bool {
+	if host == "localhost" {
+		return true
+	}
+	ip := net.ParseIP(host)
+	return ip != nil && ip.IsLoopback()
+}
@@ -89,6 +105,9 @@ func (c *Client) getAPIKey(ctx context.Context) string {
 // prepareRequest copies the request, applies defaults, and validates it.
 // The original request is never modified.
 func prepareRequest(req *SearchRequest) (*SearchRequest, error) {
+	if req == nil {
+		return nil, fmt.Errorf("serper: request must not be nil")
+	}
 	cp := *req
 	cp.SetDefaults()
 	if err := cp.Validate(); err != nil {
@@ -171,6 +190,9 @@ func (c *Client) CheckConnectivity(ctx context.Context) error {
 
 // doRequest performs an HTTP request to the Serper.dev API.
 func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBody any) error {
+	if ctx == nil {
+		return fmt.Errorf("serper: context must not be nil")
+	}
 	jsonBody, err := json.Marshal(reqBody)
 	if err != nil {
 		return fmt.Errorf("serper: marshal request: %w", err)
@@ -202,7 +224,7 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respB
 	}
 
 	if resp.StatusCode >= 400 {
-		msg := string(body)
+		msg := sanitizeForSingleLine(string(body))
 		if len(msg) > maxErrorBodyBytes {
 			msg = msg[:maxErrorBodyBytes] + "...(truncated)"
 		}
@@ -231,5 +253,20 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respB
 
 	return nil
 }
+
+func sanitizeForSingleLine(s string) string {
+	s = strings.Map(func(r rune) rune {
+		switch r {
+		case '\n', '\r', '\t':
+			return ' '
+		}
+		if r < 0x20 {
+			return -1
+		}
+		return r
+	}, s)
+
+	return strings.TrimSpace(s)
+}
```

### Diff B: Add regression tests for panic/security edges
```diff
diff --git a/serper/client_test.go b/serper/client_test.go
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -723,3 +723,55 @@ func TestNew_LastOptionWins(t *testing.T) {
 		"https://second.com")
 	}
 }
+
+func TestSearch_NilRequest(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Search(context.Background(), nil)
+	if err == nil {
+		t.Fatal("expected error for nil request")
+	}
+	if !strings.Contains(err.Error(), "request must not be nil") {
+		t.Errorf("error should mention nil request, got: %v", err)
+	}
+	if mock.req != nil {
+		t.Fatal("expected no HTTP call when request is nil")
+	}
+}
+
+func TestSearch_NilContext(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Search(nil, &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for nil context")
+	}
+	if !strings.Contains(err.Error(), "context must not be nil") {
+		t.Errorf("error should mention nil context, got: %v", err)
+	}
+}
+
+func TestNew_NilDoerRejected(t *testing.T) {
+	_, err := New("key", WithDoer(nil))
+	if err == nil {
+		t.Fatal("expected error for nil doer")
+	}
+	if !strings.Contains(err.Error(), "doer must not be nil") {
+		t.Errorf("error should mention nil doer, got: %v", err)
+	}
+}
+
+func TestNew_HTTPBaseURLRejected(t *testing.T) {
+	_, err := New("key", WithBaseURL("http://api.example.com"))
+	if err == nil {
+		t.Fatal("expected error for non-https base URL")
+	}
+	if !strings.Contains(err.Error(), "must use https") {
+		t.Errorf("unexpected error: %v", err)
+	}
+}
+
+func TestNew_HTTPBaseURLLoopbackAllowed(t *testing.T) {
+	_, err := New("key", WithBaseURL("http://localhost:8080"))
+	if err != nil {
+		t.Fatalf("expected localhost http URL to be allowed, got: %v", err)
+	}
+}
```

## Residual Risks / Gaps
- No dependency vulnerability scan was possible in this run because `govulncheck` is not installed.
- No integration tests against live Serper endpoint in this audit pass.

## Recommended Next Actions
1. Apply Diff A and Diff B.
2. Run `go test ./...` and `go vet ./...`.
3. Install/run `govulncheck ./...` in CI for ongoing dependency security coverage.
