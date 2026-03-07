Date Created: 2026-02-17 15:10:16 +0100 (CET)
TOTAL_SCORE: 84/100

## (1) AUDIT - Security and code quality issues with PATCH-READY DIFFS

### A1: `SERPER_BASE_URL` is too permissive for a key-bearing client (high)
`New()` currently accepts any parseable URI, including non-local `http://...`, so API keys can be sent over plaintext if misconfigured.

```diff
diff --git a/serper/client.go b/serper/client.go
index 7f18f89..3f4e0d5 100644
--- a/serper/client.go
+++ b/serper/client.go
@@ -8,6 +8,7 @@ import (
 	"io"
 	"net/http"
 	"net/url"
+	"strings"
 	"time"
 
 	chassiserrors "github.com/ai8future/chassis-go/v5/errors"
@@ -60,9 +61,43 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 	for _, o := range opts {
 		o(c)
 	}
-	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
-		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
+	normalizedBaseURL, err := normalizeBaseURL(c.baseURL)
+	if err != nil {
+		return nil, err
 	}
+	c.baseURL = normalizedBaseURL
 	return c, nil
 }
+
+func normalizeBaseURL(raw string) (string, error) {
+	u, err := url.ParseRequestURI(raw)
+	if err != nil {
+		return "", fmt.Errorf("serper: invalid base URL %q: %w", raw, err)
+	}
+	if u.RawQuery != "" || u.Fragment != "" {
+		return "", fmt.Errorf("serper: base URL must not include query or fragment")
+	}
+
+	scheme := strings.ToLower(u.Scheme)
+	host := strings.ToLower(u.Hostname())
+	isLocal := host == "localhost" || host == "127.0.0.1" || host == "::1"
+
+	switch scheme {
+	case "https":
+		// secure default for API keys
+	case "http":
+		if !isLocal {
+			return "", fmt.Errorf("serper: insecure base URL %q: use https", raw)
+		}
+	default:
+		return "", fmt.Errorf("serper: unsupported base URL scheme %q", u.Scheme)
+	}
+
+	u.Path = strings.TrimRight(u.Path, "/")
+	return u.String(), nil
+}
```

## (2) TESTS - Proposed unit tests for untested code with PATCH-READY DIFFS

### T1: Add branch tests for new URL validation and HTTP error mapping
Targets currently under-covered behavior in `New()` and `doRequest()` branches.

```diff
diff --git a/serper/client_test.go b/serper/client_test.go
index cb95ca4..6ce58e8 100644
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -99,6 +99,38 @@ func TestSearch_Success(t *testing.T) {
 	}
 }
 
+func TestNew_RejectsInsecureHTTPBaseURL(t *testing.T) {
+	_, err := New("key", WithBaseURL("http://example.com"))
+	if err == nil {
+		t.Fatal("expected error for insecure non-local HTTP base URL")
+	}
+	if !strings.Contains(err.Error(), "insecure base URL") {
+		t.Fatalf("expected insecure base URL error, got: %v", err)
+	}
+}
+
+func TestNew_AllowsLocalHTTPBaseURL(t *testing.T) {
+	_, err := New("key", WithBaseURL("http://localhost:8080"))
+	if err != nil {
+		t.Fatalf("expected localhost HTTP to be allowed, got: %v", err)
+	}
+}
+
+func TestNew_RejectsBaseURLWithQueryOrFragment(t *testing.T) {
+	tests := []string{
+		"https://google.serper.dev?x=1",
+		"https://google.serper.dev#frag",
+	}
+	for _, raw := range tests {
+		t.Run(raw, func(t *testing.T) {
+			_, err := New("key", WithBaseURL(raw))
+			if err == nil {
+				t.Fatalf("expected error for base URL %q", raw)
+			}
+			if !strings.Contains(err.Error(), "must not include query or fragment") {
+				t.Fatalf("unexpected error for %q: %v", raw, err)
+			}
+		})
+	}
+}
+
 func TestSearch_HTTPError(t *testing.T) {
 	mock := &mockDoer{statusCode: 401, respBody: `{"error":"unauthorized"}`}
 	c := mustNew(t, "bad-key", WithDoer(mock))
@@ -552,6 +584,32 @@ func TestSearch_HTTP503(t *testing.T) {
 	}
 }
 
+func TestSearch_HTTP403(t *testing.T) {
+	mock := &mockDoer{statusCode: 403, respBody: `forbidden`}
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
+func TestSearch_HTTP502(t *testing.T) {
+	mock := &mockDoer{statusCode: 502, respBody: `bad gateway`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected error for 502 status")
+	}
+	if !strings.Contains(err.Error(), "502") {
+		t.Errorf("error should contain '502', got: %v", err)
+	}
+}
+
 func TestSetDefaults_PreservesExplicitValues(t *testing.T) {
 	r := &SearchRequest{Q: "test", Num: 50, GL: "de", HL: "fr", Page: 3}
 	r.SetDefaults()
```

### T2: Add a fast CLI test that covers `main()` usage path
`cmd/serper/main.go` is currently 0% covered.

```diff
diff --git a/cmd/serper/main_test.go b/cmd/serper/main_test.go
index 8ea24a7..3c750b8 100644
--- a/cmd/serper/main_test.go
+++ b/cmd/serper/main_test.go
@@ -1,10 +1,13 @@
 package main
 
 import (
+	"bytes"
 	"os"
+	"os/exec"
+	"strings"
 	"testing"
 
 	chassis "github.com/ai8future/chassis-go/v5"
 	chassisconfig "github.com/ai8future/chassis-go/v5/config"
 	"github.com/ai8future/chassis-go/v5/testkit"
@@ -46,3 +49,19 @@ func TestConfig_PanicsWithoutAPIKey(t *testing.T) {
 	}()
 	_ = chassisconfig.MustLoad[Config]()
 }
+
+func TestMain_UsageWhenNoArgs(t *testing.T) {
+	cmd := exec.Command("go", "run", ".")
+	cmd.Dir = "."
+	cmd.Env = append(os.Environ(), "SERPER_API_KEY=test-key")
+
+	var stderr bytes.Buffer
+	cmd.Stderr = &stderr
+
+	err := cmd.Run()
+	if err == nil {
+		t.Fatal("expected non-zero exit for missing query")
+	}
+	if !strings.Contains(stderr.String(), "usage: serper <query>") {
+		t.Fatalf("expected usage message, got: %q", stderr.String())
+	}
+}
```

## (3) FIXES - Bugs, issues, and code smells with fixes and PATCH-READY DIFFS

### F1: Nil request/context can panic instead of returning typed errors (high)
`prepareRequest()` dereferences `*req` with no nil check, and `doRequest()` accepts nil context/doer.

```diff
diff --git a/serper/client.go b/serper/client.go
index 7f18f89..d7f87f0 100644
--- a/serper/client.go
+++ b/serper/client.go
@@ -89,8 +89,19 @@ func (c *Client) getAPIKey(ctx context.Context) string {
 // prepareRequest copies the request, applies defaults, and validates it.
 // The original request is never modified.
 func prepareRequest(req *SearchRequest) (*SearchRequest, error) {
+	if req == nil {
+		return nil, chassiserrors.ValidationError("serper: request must not be nil")
+	}
 	cp := *req
 	cp.SetDefaults()
+	if strings.TrimSpace(cp.Q) == "" {
+		return nil, chassiserrors.ValidationError("serper: query (q) must not be empty")
+	}
 	if err := cp.Validate(); err != nil {
 		return nil, err
 	}
 	return &cp, nil
 }
@@ -172,6 +183,13 @@ func (c *Client) CheckConnectivity(ctx context.Context) error {
 
 // doRequest performs an HTTP request to the Serper.dev API.
 func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBody any) error {
+	if ctx == nil {
+		return chassiserrors.ValidationError("serper: context must not be nil")
+	}
+	if c.doer == nil {
+		return chassiserrors.InternalError("serper: HTTP client is nil")
+	}
+
 	jsonBody, err := json.Marshal(reqBody)
 	if err != nil {
 		return fmt.Errorf("serper: marshal request: %w", err)
@@ -196,13 +214,20 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respB
 	}
 	defer resp.Body.Close()
 
-	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
+	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
 	if err != nil {
 		return fmt.Errorf("serper: read response: %w", err)
 	}
+	if len(body) > maxResponseBytes {
+		return chassiserrors.PayloadTooLargeError(
+			fmt.Sprintf("serper: response exceeded %d bytes", maxResponseBytes),
+		)
+	}
 
 	if resp.StatusCode >= 400 {
 		msg := string(body)
 		if len(msg) > maxErrorBodyBytes {
 			msg = msg[:maxErrorBodyBytes] + "...(truncated)"
 		}
 		detail := fmt.Sprintf("serper: HTTP %d: %s", resp.StatusCode, msg)
 		switch resp.StatusCode {
 		case http.StatusBadRequest:
 			return chassiserrors.ValidationError(detail)
 		case http.StatusUnauthorized:
 			return chassiserrors.UnauthorizedError(detail)
+		case http.StatusForbidden:
+			return chassiserrors.ForbiddenError(detail)
 		case http.StatusNotFound:
 			return chassiserrors.NotFoundError(detail)
+		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
+			return chassiserrors.TimeoutError(detail)
 		case http.StatusTooManyRequests:
 			return chassiserrors.RateLimitError(detail)
 		case http.StatusBadGateway, http.StatusServiceUnavailable:
```

### F2: Whitespace-only queries currently pass validation (medium)
This allows avoidable bad requests (`"   "`) through the public validator.

```diff
diff --git a/serper/types.go b/serper/types.go
index d2fa17b..9fec161 100644
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
@@ -162,7 +165,7 @@ func (r *SearchRequest) SetDefaults() {
 // Validate checks that the SearchRequest is valid.
 // Call SetDefaults before Validate if you want zero-value fields filled in.
 func (r *SearchRequest) Validate() error {
-	if r.Q == "" {
+	if strings.TrimSpace(r.Q) == "" {
 		return fmt.Errorf("query (q) is required")
 	}
 	if r.Num < 1 || r.Num > 100 {
```

## (4) REFACTOR - Opportunities to improve code quality (no diffs needed)

1. Introduce a single internal endpoint helper to remove repeated `Search/Images/News/Places/Scholar` boilerplate.
2. Extract CLI flow from `main()` into `run(args []string) (int, error)` for deterministic unit testing without subprocesses.
3. Replace raw string comparisons in tests with typed `*errors.ServiceError` assertions to validate HTTP/gRPC mapping semantics.
4. Consider adding table-driven tests for all HTTP status mappings in `doRequest()` to prevent future regressions.
5. Add package-level docs for environment variable semantics (`SERPER_TIMEOUT` retry/attempt behavior) to reduce operator misconfiguration risk.
