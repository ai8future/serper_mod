Date Created: 2026-02-17 14:56:23 CET (+0100)
TOTAL_SCORE: 74/100

# serper_mod Untested-Code Unit Test Proposal (Codex:gpt-5.1-codex-max-high)

## Snapshot
- Current coverage from `go test ./... -coverprofile=...`:
  - `./serper`: `86.4%`
  - `./cmd/serper`: `0.0%`
  - Total: `71.4%`
- Most important gap: `cmd/serper/main.go` has no executable-path coverage.
- `serper/client.go` still misses key error branches (marshal/create/read/unmarshal + status mapping branches).

## Untested Code Map
- `cmd/serper/main.go:31`
- `cmd/serper/main.go:37`
- `cmd/serper/main.go:41`
- `cmd/serper/main.go:52`
- `cmd/serper/main.go:57`
- `cmd/serper/main.go:67`
- `cmd/serper/main.go:72`
- `cmd/serper/main.go:73`
- `cmd/serper/main.go:77`
- `serper/client.go:116`
- `serper/client.go:120`
- `serper/client.go:129`
- `serper/client.go:133`
- `serper/client.go:142`
- `serper/client.go:146`
- `serper/client.go:155`
- `serper/client.go:159`
- `serper/client.go:176`
- `serper/client.go:181`
- `serper/client.go:200`
- `serper/client.go:211`
- `serper/client.go:215`
- `serper/client.go:217`
- `serper/client.go:230`

## Recommended Test Additions
1. Add endpoint-wrapper error-path tests for `Images`, `News`, `Places`, `Scholar`:
- input validation failure propagation
- `doRequest` failure propagation

2. Add direct `doRequest` branch tests:
- JSON marshal failure
- request construction failure
- response read failure
- JSON unmarshal failure

3. Add HTTP status-to-error-mapping tests:
- `400` => validation
- `404` => not found
- `429` => rate limit

4. Add subprocess-based CLI tests for `main()`:
- no args => usage + exit 1
- invalid base URL => client creation failure + exit 1
- API/search failure => exit 1
- success path => JSON printed + expected request details

## Patch-Ready Diff A (`serper/client_test.go`)
```diff
diff --git a/serper/client_test.go b/serper/client_test.go
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -9,6 +9,8 @@ import (
 	"strings"
 	"testing"
+
+	chassiserrors "github.com/ai8future/chassis-go/v5/errors"
 )
@@ -723,3 +725,201 @@ func TestNew_LastOptionWins(t *testing.T) {
 		c.baseURL, "https://second.com")
 	}
 }
+
+func TestEndpointMethods_ErrorPaths(t *testing.T) {
+	type endpointCase struct {
+		name string
+		call func(*Client, context.Context, *SearchRequest) error
+	}
+
+	endpoints := []endpointCase{
+		{
+			name: "Images",
+			call: func(c *Client, ctx context.Context, req *SearchRequest) error {
+				_, err := c.Images(ctx, req)
+				return err
+			},
+		},
+		{
+			name: "News",
+			call: func(c *Client, ctx context.Context, req *SearchRequest) error {
+				_, err := c.News(ctx, req)
+				return err
+			},
+		},
+		{
+			name: "Places",
+			call: func(c *Client, ctx context.Context, req *SearchRequest) error {
+				_, err := c.Places(ctx, req)
+				return err
+			},
+		},
+		{
+			name: "Scholar",
+			call: func(c *Client, ctx context.Context, req *SearchRequest) error {
+				_, err := c.Scholar(ctx, req)
+				return err
+			},
+		},
+	}
+
+	t.Run("validation_error", func(t *testing.T) {
+		mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+		c := mustNew(t, "key", WithDoer(mock))
+
+		for _, ep := range endpoints {
+			t.Run(ep.name, func(t *testing.T) {
+				mock.req = nil
+				err := ep.call(c, context.Background(), &SearchRequest{Q: ""})
+				if err == nil {
+					t.Fatal("expected validation error")
+				}
+				if mock.req != nil {
+					t.Fatal("expected no HTTP request for invalid input")
+				}
+			})
+		}
+	})
+
+	t.Run("request_error", func(t *testing.T) {
+		for _, ep := range endpoints {
+			t.Run(ep.name, func(t *testing.T) {
+				mock := &mockDoer{statusCode: 500, respBody: `{"error":"boom"}`}
+				c := mustNew(t, "key", WithDoer(mock))
+
+				err := ep.call(c, context.Background(), &SearchRequest{Q: "test"})
+				if err == nil {
+					t.Fatal("expected endpoint error")
+				}
+				if !strings.Contains(err.Error(), "500") {
+					t.Fatalf("expected status in error, got: %v", err)
+				}
+			})
+		}
+	})
+}
+
+func TestDoRequest_MarshalRequestError(t *testing.T) {
+	c := mustNew(t, "key", WithDoer(&mockDoer{}))
+
+	badReq := map[string]any{"bad": make(chan int)}
+	err := c.doRequest(context.Background(), "/search", badReq, &SearchResponse{})
+	if err == nil {
+		t.Fatal("expected marshal request error")
+	}
+	if !strings.Contains(err.Error(), "marshal request") {
+		t.Fatalf("unexpected error: %v", err)
+	}
+}
+
+func TestDoRequest_CreateRequestError(t *testing.T) {
+	c := &Client{
+		apiKey:  "key",
+		baseURL: "://bad-url",
+		doer:    &mockDoer{},
+	}
+
+	err := c.doRequest(context.Background(), "/search", map[string]any{"q": "test"}, &SearchResponse{})
+	if err == nil {
+		t.Fatal("expected create request error")
+	}
+	if !strings.Contains(err.Error(), "create request") {
+		t.Fatalf("unexpected error: %v", err)
+	}
+}
+
+type readErrorBody struct{}
+
+func (readErrorBody) Read([]byte) (int, error) {
+	return 0, fmt.Errorf("forced read failure")
+}
+
+func (readErrorBody) Close() error {
+	return nil
+}
+
+type readErrorDoer struct{}
+
+func (readErrorDoer) Do(*http.Request) (*http.Response, error) {
+	return &http.Response{StatusCode: 200, Body: readErrorBody{}}, nil
+}
+
+func TestDoRequest_ReadResponseError(t *testing.T) {
+	c := mustNew(t, "key", WithDoer(readErrorDoer{}))
+
+	err := c.doRequest(context.Background(), "/search", map[string]any{"q": "test"}, &SearchResponse{})
+	if err == nil {
+		t.Fatal("expected read response error")
+	}
+	if !strings.Contains(err.Error(), "read response") {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if !strings.Contains(err.Error(), "forced read failure") {
+		t.Fatalf("expected original read error, got: %v", err)
+	}
+}
+
+func TestSearch_HTTPStatusMappings(t *testing.T) {
+	tests := []struct {
+		name         string
+		status       int
+		wantHTTPCode int
+	}{
+		{name: "bad_request", status: http.StatusBadRequest, wantHTTPCode: http.StatusBadRequest},
+		{name: "not_found", status: http.StatusNotFound, wantHTTPCode: http.StatusNotFound},
+		{name: "rate_limit", status: http.StatusTooManyRequests, wantHTTPCode: http.StatusTooManyRequests},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			mock := &mockDoer{statusCode: tt.status, respBody: `{"error":"boom"}`}
+			c := mustNew(t, "key", WithDoer(mock))
+
+			_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+			if err == nil {
+				t.Fatal("expected HTTP error")
+			}
+
+			svcErr := chassiserrors.FromError(err)
+			if svcErr.HTTPCode != tt.wantHTTPCode {
+				t.Fatalf("HTTP code: got %d, want %d (err=%v)", svcErr.HTTPCode, tt.wantHTTPCode, err)
+			}
+		})
+	}
+}
+
+func TestSearch_UnmarshalResponseError(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":"not-an-array"}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+	if err == nil {
+		t.Fatal("expected unmarshal response error")
+	}
+	if !strings.Contains(err.Error(), "unmarshal response") {
+		t.Fatalf("unexpected error: %v", err)
+	}
+}
```

## Patch-Ready Diff B (`cmd/serper/main_test.go`)
```diff
diff --git a/cmd/serper/main_test.go b/cmd/serper/main_test.go
--- a/cmd/serper/main_test.go
+++ b/cmd/serper/main_test.go
@@ -1,7 +1,17 @@
 package main
 
 import (
+	"bytes"
+	"encoding/json"
+	"errors"
+	"io"
+	"net/http"
+	"net/http/httptest"
 	"os"
+	"os/exec"
+	"strings"
 	"testing"
+	"time"
 
 	chassis "github.com/ai8future/chassis-go/v5"
 	chassisconfig "github.com/ai8future/chassis-go/v5/config"
@@ -46,3 +56,162 @@ func TestConfig_PanicsWithoutAPIKey(t *testing.T) {
 	}()
 	_ = chassisconfig.MustLoad[Config]()
 }
+
+func runMainProcess(t *testing.T, scenario string, extraEnv map[string]string) (stdout string, stderr string, exitCode int) {
+	t.Helper()
+
+	cmd := exec.Command(os.Args[0], "-test.run=TestMain_Subprocess")
+	env := append(os.Environ(),
+		"GO_WANT_HELPER_PROCESS=1",
+		"SERPER_TEST_SCENARIO="+scenario,
+	)
+	for k, v := range extraEnv {
+		env = append(env, k+"="+v)
+	}
+	cmd.Env = env
+
+	var outBuf bytes.Buffer
+	var errBuf bytes.Buffer
+	cmd.Stdout = &outBuf
+	cmd.Stderr = &errBuf
+
+	err := cmd.Run()
+	if err == nil {
+		return outBuf.String(), errBuf.String(), 0
+	}
+
+	var exitErr *exec.ExitError
+	if errors.As(err, &exitErr) {
+		return outBuf.String(), errBuf.String(), exitErr.ExitCode()
+	}
+
+	t.Fatalf("failed to run helper process: %v", err)
+	return "", "", -1
+}
+
+func TestMain_Subprocess(t *testing.T) {
+	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
+		return
+	}
+
+	switch os.Getenv("SERPER_TEST_SCENARIO") {
+	case "no_args":
+		os.Args = []string{"serper"}
+	default:
+		os.Args = []string{"serper", "open", "ai"}
+	}
+
+	main()
+	os.Exit(0)
+}
+
+func TestCLI_UsageWhenNoArgs(t *testing.T) {
+	_, stderr, exitCode := runMainProcess(t, "no_args", map[string]string{
+		"SERPER_API_KEY": "test-key",
+	})
+
+	if exitCode != 1 {
+		t.Fatalf("exit code: got %d, want 1 (stderr=%q)", exitCode, stderr)
+	}
+	if !strings.Contains(stderr, "usage: serper <query>") {
+		t.Fatalf("expected usage output, got: %q", stderr)
+	}
+}
+
+func TestCLI_NewClientError(t *testing.T) {
+	_, stderr, exitCode := runMainProcess(t, "with_query", map[string]string{
+		"SERPER_API_KEY":  "test-key",
+		"SERPER_BASE_URL": "://bad",
+	})
+
+	if exitCode != 1 {
+		t.Fatalf("exit code: got %d, want 1 (stderr=%q)", exitCode, stderr)
+	}
+	if !strings.Contains(stderr, "invalid base URL") {
+		t.Fatalf("expected invalid base URL message, got: %q", stderr)
+	}
+}
+
+func TestCLI_SearchError(t *testing.T) {
+	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+		w.WriteHeader(http.StatusUnauthorized)
+		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
+	}))
+	defer srv.Close()
+
+	_, stderr, exitCode := runMainProcess(t, "with_query", map[string]string{
+		"SERPER_API_KEY":  "test-key",
+		"SERPER_BASE_URL": srv.URL,
+		"SERPER_TIMEOUT":  "3s",
+	})
+
+	if exitCode != 1 {
+		t.Fatalf("exit code: got %d, want 1 (stderr=%q)", exitCode, stderr)
+	}
+	if !strings.Contains(stderr, "401") {
+		t.Fatalf("expected 401 in stderr, got: %q", stderr)
+	}
+}
+
+func TestCLI_Success(t *testing.T) {
+	type requestCapture struct {
+		Method      string
+		Path        string
+		APIKey      string
+		ContentType string
+		Body        []byte
+	}
+
+	reqCh := make(chan requestCapture, 8)
+	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+		body, _ := io.ReadAll(r.Body)
+		select {
+		case reqCh <- requestCapture{
+			Method:      r.Method,
+			Path:        r.URL.Path,
+			APIKey:      r.Header.Get("X-API-KEY"),
+			ContentType: r.Header.Get("Content-Type"),
+			Body:        body,
+		}:
+		default:
+		}
+
+		w.Header().Set("Content-Type", "application/json")
+		_, _ = w.Write([]byte(`{"searchParameters":{"q":"open ai","gl":"us","hl":"en","num":10,"type":"search","engine":"google"},"organic":[]}`))
+	}))
+	defer srv.Close()
+
+	stdout, stderr, exitCode := runMainProcess(t, "with_query", map[string]string{
+		"SERPER_API_KEY":  "test-key",
+		"SERPER_BASE_URL": srv.URL,
+		"SERPER_TIMEOUT":  "3s",
+	})
+
+	if exitCode != 0 {
+		t.Fatalf("exit code: got %d, want 0 (stderr=%q)", exitCode, stderr)
+	}
+
+	var out map[string]any
+	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
+		t.Fatalf("stdout is not valid JSON: %v\nstdout=%q\nstderr=%q", err, stdout, stderr)
+	}
+	if _, ok := out["organic"]; !ok {
+		t.Fatalf("stdout missing organic field: %q", stdout)
+	}
+
+	select {
+	case got := <-reqCh:
+		if got.Method != http.MethodPost {
+			t.Fatalf("method: got %q, want %q", got.Method, http.MethodPost)
+		}
+		if got.Path != "/search" {
+			t.Fatalf("path: got %q, want %q", got.Path, "/search")
+		}
+		if got.APIKey != "test-key" {
+			t.Fatalf("X-API-KEY: got %q, want %q", got.APIKey, "test-key")
+		}
+		if got.ContentType != "application/json" {
+			t.Fatalf("Content-Type: got %q, want %q", got.ContentType, "application/json")
+		}
+
+		var sent map[string]any
+		if err := json.Unmarshal(got.Body, &sent); err != nil {
+			t.Fatalf("request body is not valid JSON: %v", err)
+		}
+		if sent["q"] != "open ai" {
+			t.Fatalf("query: got %v, want %q", sent["q"], "open ai")
+		}
+	case <-time.After(2 * time.Second):
+		t.Fatal("did not capture outbound request")
+	}
+}
```

## Optional (for full branch closure)
- `cmd/serper/main.go:73` (`json.MarshalIndent` failure branch) is hard to force with current types because `SearchResponse` is always marshalable.
- If you want that branch unit-tested, add a tiny seam:

```diff
diff --git a/cmd/serper/main.go b/cmd/serper/main.go
--- a/cmd/serper/main.go
+++ b/cmd/serper/main.go
@@ -28,6 +28,8 @@ type Config struct {
 	LogLevel string       `env:"LOG_LEVEL" default:"error"`
 }
+
+var marshalIndent = json.MarshalIndent
@@ -69,7 +71,7 @@ func main() {
 		os.Exit(1)
 	}
 
-	out, err := json.MarshalIndent(resp, "", "  ")
+	out, err := marshalIndent(resp, "", "  ")
 	if err != nil {
 		fmt.Fprintf(os.Stderr, "error formatting response: %v\n", err)
 		os.Exit(1)
```

## Expected Impact
- `serper/client.go` error-branch confidence materially improves (not just happy-path validation).
- `cmd/serper/main.go` moves from `0%` to high practical coverage of real CLI behavior.
- Remaining gap should mostly be the marshal-failure branch unless the optional seam is accepted.
