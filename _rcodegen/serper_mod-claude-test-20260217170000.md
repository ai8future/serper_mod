Date Created: 2026-02-17T17:00:00-05:00
TOTAL_SCORE: 76/100

# Unit Test Coverage Report: serper_mod

**Auditor:** Claude Opus 4.6
**Module:** `github.com/ai8future/serper_mod`
**Existing Coverage:** 86.4% (`serper` pkg), 0.0% (`cmd/serper` pkg)

---

## Executive Summary

The `serper` package has a strong test foundation with 86.4% statement coverage across 32 test functions. Every public API method has at least one happy-path test, and the `Search` method in particular has comprehensive error-path coverage. The remaining coverage gaps fall into three clear categories, all of which represent real behavioral contracts:

1. **HTTP status code classification** — the `doRequest` switch statement maps 7 HTTP codes to 6 typed error constructors, but only 3 (401, 500, 503) are tested. The 400/404/429/502 branches are completely uncovered.
2. **Endpoint validation error paths** — `Images`, `News`, `Places`, and `Scholar` each sit at 71.4% because their `prepareRequest` error branch is never exercised. `Search` covers this path but the other four methods don't.
3. **Concurrency safety** — the `Client` is immutable after construction and should be safe for concurrent use, but no test exercises this with the race detector.

The CLI (`cmd/serper`) has 0% statement coverage, though `Config` loading is well-tested. The `main()` function calls `os.Exit` and uses real I/O, making it untestable without a refactor that would be disproportionate to the value gained. The config tests are missing assertions for `HL`, `LogLevel`, and `Timeout` defaults.

**Score: 76/100** reflects strong fundamentals with meaningful, addressable gaps in branch coverage and typed error classification.

---

## Score Breakdown

| Category | Max | Score | Notes |
|---|---|---|---|
| **Branch coverage of core library** | 30 | 23 | 86.4% statement; 4 HTTP error branches + 4 endpoint error paths uncovered |
| **Endpoint method coverage** | 15 | 10 | Search fully covered; Images/News/Places/Scholar each miss validation-failure path |
| **Error classification coverage** | 20 | 13 | 3/7 HTTP status branches tested (401, 500, 503); typed error assertions missing |
| **Edge cases and robustness** | 20 | 17 | Good: context cancel, truncation, boundaries, malformed JSON, empty response, mutation guard. Missing: concurrency |
| **CLI coverage** | 15 | 13 | Config loading tested well; missing HL/LogLevel/Timeout default assertions |

---

## Prior Report Assessment

### Gemini Test Report (2026-02-03)

| Proposal | Verdict | Reason |
|---|---|---|
| `TestClient_ConcurrentRequests` | **VALUABLE** but has a bug | The `mockDoer` writes to shared `req`/`body` fields without synchronization — test would flag the mock, not the client. Needs a stateless mock. |
| `TestSearch_UnexpectedJSONStructure` | **DEAD WEIGHT** | Go's `json.Unmarshal` of `[]` into a struct pointer produces a zero-value struct without error. The test incorrectly asserts an error. Tests stdlib, not our code. |
| `TestConfig_Defaults` LogLevel check | **VALUABLE** | One-line addition validating a real default. |

### Claude Test Report (2026-02-15)

All 6 proposals (T-1 through T-6) were well-reasoned. My assessment:

| Proposal | Verdict | Reason |
|---|---|---|
| T-1: HTTP Status Code Mapping | **VALUABLE** | Covers 4 untested switch branches. Correct approach. I strengthen it below with typed error assertions. |
| T-2: Concurrent Requests (corrected mock) | **VALUABLE** | The corrected `concurrentSafeDoer` approach is right. |
| T-3: Endpoint Validation Errors | **VALUABLE** | Covers 4 error paths. Minor issue: the mock state check (`mock.req != nil`) has a subtle bug — mock's `req` may persist from a prior subtest since the mock is shared. I fix this below. |
| T-4: LogLevel Default | **VALUABLE** | One-line, validates contract. |
| T-5: Config Timeout/HL Defaults | **VALUABLE** | Completes the config default coverage. |
| T-6: Config Env Overrides | **MEDIUM** | Tests the chassis config loader more than our code. Still worth adding since it validates the struct tags work end-to-end. |

---

## Proposed New Tests

### HIGH VALUE — T-1: HTTP Status Code Classification with Typed Error Assertions

**Location:** `serper/client_test.go` after line 725
**Covers:** Lines 210-223 of `client.go` — 4 untested switch branches (400, 404, 429, 502)
**Why valuable:** The typed error mapping is a core behavioral contract. If someone rearranges the switch or changes an error type, these regressions would go undetected. The previous report (T-1) tested only that the error message contained the status code — this version also verifies the correct `chassiserrors.ServiceError` HTTP code is returned, which is what downstream callers actually depend on.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -1,6 +1,7 @@
 package serper

 import (
+	"errors"
 	"context"
 	"encoding/json"
 	"fmt"
@@ -8,6 +9,8 @@ import (
 	"net/http"
 	"strings"
 	"testing"
+
+	chassiserrors "github.com/ai8future/chassis-go/v5/errors"
 )

 // mockDoer captures the request and returns a canned response.
@@ -725,3 +728,49 @@ func TestNew_LastOptionWins(t *testing.T) {
 		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://second.com")
 	}
 }
+
+func TestSearch_HTTPStatusCodeMapping(t *testing.T) {
+	tests := []struct {
+		name         string
+		statusCode   int
+		wantHTTPCode int
+	}{
+		{"400 Bad Request → ValidationError", 400, 400},
+		{"404 Not Found → NotFoundError", 404, 404},
+		{"429 Too Many Requests → RateLimitError", 429, 429},
+		{"502 Bad Gateway → DependencyError", 502, 503},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			mock := &mockDoer{statusCode: tt.statusCode, respBody: `{"error":"test"}`}
+			c := mustNew(t, "key", WithDoer(mock))
+
+			_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+			if err == nil {
+				t.Fatalf("expected error for HTTP %d", tt.statusCode)
+			}
+
+			// Verify the error message contains the status code.
+			if !strings.Contains(err.Error(), fmt.Sprintf("%d", tt.statusCode)) {
+				t.Errorf("error should contain %q, got: %v", fmt.Sprintf("%d", tt.statusCode), err)
+			}
+
+			// Verify the correct typed error was returned.
+			var svcErr *chassiserrors.ServiceError
+			if !errors.As(err, &svcErr) {
+				t.Fatalf("expected *chassiserrors.ServiceError, got %T", err)
+			}
+			if svcErr.HTTPCode != tt.wantHTTPCode {
+				t.Errorf("HTTPCode: got %d, want %d", svcErr.HTTPCode, tt.wantHTTPCode)
+			}
+		})
+	}
+}
```

**Note on `wantHTTPCode` for 502:** `DependencyError` creates a `ServiceError` with HTTPCode 503 (not 502), since the factory function hardcodes the code. The test maps input status 502 → expected HTTPCode 503, which validates the deliberate `StatusBadGateway → DependencyError` collapsing behavior in line 219. If this is wrong, the test author should update the mapping, but either way, the test surfaces the actual behavior.

---

### HIGH VALUE — T-2: Concurrent Client Usage

**Location:** `serper/client_test.go` after T-1
**Covers:** Race detector validation of `Client` immutability
**Why valuable:** Guards against future introduction of shared mutable state. The existing `mockDoer` is NOT concurrency-safe (writes to `req`/`body`), so a stateless mock is required.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -725,3 +728,30 @@ func TestNew_LastOptionWins(t *testing.T) {
 		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://second.com")
 	}
 }
+
+// concurrentSafeDoer is a stateless mock for concurrency tests.
+type concurrentSafeDoer struct {
+	statusCode int
+	respBody   string
+}
+
+func (d *concurrentSafeDoer) Do(req *http.Request) (*http.Response, error) {
+	return &http.Response{
+		StatusCode: d.statusCode,
+		Body:       io.NopCloser(strings.NewReader(d.respBody)),
+	}, nil
+}
+
+func TestClient_ConcurrentRequests(t *testing.T) {
+	doer := &concurrentSafeDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "key", WithDoer(doer))
+
+	concurrency := 10
+	errCh := make(chan error, concurrency)
+
+	for i := 0; i < concurrency; i++ {
+		go func() {
+			_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
+			errCh <- err
+		}()
+	}
+
+	for i := 0; i < concurrency; i++ {
+		if err := <-errCh; err != nil {
+			t.Errorf("concurrent search error: %v", err)
+		}
+	}
+}
```

---

### HIGH VALUE — T-3: Endpoint Validation Error Propagation

**Location:** `serper/client_test.go` after T-2
**Covers:** Lines 114-117, 127-130, 140-143, 153-156 of `client.go` — the `prepareRequest` error return in each non-Search method
**Why valuable:** Lifts `Images`, `News`, `Places`, `Scholar` from 71.4% to 100%. One table-driven test covers all four gaps. The previous report's version had a subtle bug: it shared a single `mockDoer` across subtests and checked `mock.req != nil` to verify no HTTP call was made, but `mock.req` could persist from setup. Fixed below by creating a fresh mock per subtest.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -725,3 +728,32 @@ func TestNew_LastOptionWins(t *testing.T) {
 		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://second.com")
 	}
 }
+
+func TestEndpoints_ValidationError(t *testing.T) {
+	badReq := &SearchRequest{Q: ""}
+
+	tests := []struct {
+		name string
+		call func(*Client) error
+	}{
+		{"Images", func(c *Client) error { _, err := c.Images(context.Background(), badReq); return err }},
+		{"News", func(c *Client) error { _, err := c.News(context.Background(), badReq); return err }},
+		{"Places", func(c *Client) error { _, err := c.Places(context.Background(), badReq); return err }},
+		{"Scholar", func(c *Client) error { _, err := c.Scholar(context.Background(), badReq); return err }},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			mock := &mockDoer{statusCode: 200, respBody: `{}`}
+			c := mustNew(t, "key", WithDoer(mock))
+
+			err := tt.call(c)
+			if err == nil {
+				t.Fatal("expected validation error for empty query")
+			}
+			if !strings.Contains(err.Error(), "query") {
+				t.Errorf("error should mention 'query', got: %v", err)
+			}
+			if mock.req != nil {
+				t.Error("no HTTP request should have been made for invalid input")
+			}
+		})
+	}
+}
```

---

### MEDIUM VALUE — T-4: Complete Config Default Assertions

**Location:** `cmd/serper/main_test.go`, expanding `TestConfig_Defaults`
**Covers:** `HL`, `LogLevel`, and `Timeout` default values — none currently asserted
**Why valuable:** Completes the config contract testing. These are real defaults that downstream behavior depends on (e.g., `Timeout` determines `call.Client` deadline).

```diff
--- a/cmd/serper/main_test.go
+++ b/cmd/serper/main_test.go
@@ -3,6 +3,7 @@ package main
 import (
 	"os"
 	"testing"
+	"time"

 	chassis "github.com/ai8future/chassis-go/v5"
 	chassisconfig "github.com/ai8future/chassis-go/v5/config"
@@ -34,6 +35,15 @@ func TestConfig_Defaults(t *testing.T) {
 	if cfg.GL != "us" {
 		t.Errorf("expected default GL 'us', got %q", cfg.GL)
 	}
+	if cfg.HL != "en" {
+		t.Errorf("expected default HL 'en', got %q", cfg.HL)
+	}
+	if cfg.LogLevel != "error" {
+		t.Errorf("expected default LogLevel 'error', got %q", cfg.LogLevel)
+	}
+	if cfg.Timeout != 30*time.Second {
+		t.Errorf("expected default Timeout 30s, got %v", cfg.Timeout)
+	}
 }

 func TestConfig_PanicsWithoutAPIKey(t *testing.T) {
```

---

### MEDIUM VALUE — T-5: Config Environment Variable Overrides

**Location:** `cmd/serper/main_test.go` after `TestConfig_PanicsWithoutAPIKey`
**Covers:** The env→struct mapping for non-default values
**Why valuable:** Validates that the struct tags (`env:"..."`) work correctly for all fields. Currently only the default path is tested, never the override path.

```diff
--- a/cmd/serper/main_test.go
+++ b/cmd/serper/main_test.go
@@ -47,3 +48,33 @@ func TestConfig_PanicsWithoutAPIKey(t *testing.T) {
 	_ = chassisconfig.MustLoad[Config]()
 }
+
+func TestConfig_EnvironmentOverrides(t *testing.T) {
+	testkit.SetEnv(t, map[string]string{
+		"SERPER_API_KEY":  "override-key",
+		"SERPER_BASE_URL": "https://custom.example.com",
+		"SERPER_NUM":      "25",
+		"SERPER_GL":       "de",
+		"SERPER_HL":       "fr",
+		"SERPER_TIMEOUT":  "10s",
+		"LOG_LEVEL":       "debug",
+	})
+
+	cfg := chassisconfig.MustLoad[Config]()
+	if cfg.APIKey != "override-key" {
+		t.Errorf("APIKey: got %q, want %q", cfg.APIKey, "override-key")
+	}
+	if cfg.BaseURL != "https://custom.example.com" {
+		t.Errorf("BaseURL: got %q, want %q", cfg.BaseURL, "https://custom.example.com")
+	}
+	if cfg.Num != 25 {
+		t.Errorf("Num: got %d, want 25", cfg.Num)
+	}
+	if cfg.GL != "de" {
+		t.Errorf("GL: got %q, want %q", cfg.GL, "de")
+	}
+	if cfg.HL != "fr" {
+		t.Errorf("HL: got %q, want %q", cfg.HL, "fr")
+	}
+	if cfg.LogLevel != "debug" {
+		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "debug")
+	}
+}
```

---

## Rejected — Not Valuable / Dead Weight

| Proposal | Reason |
|---|---|
| `TestSearch_UnexpectedJSONStructure` (Gemini) | Go's `json.Unmarshal` of `[]` into a struct pointer returns a zero-value struct without error. The test would **fail** as written. Tests standard library behavior, not our code. |
| Fuzz tests for `Validate()` | Validation has exactly 3 branches with simple integer/string checks. Table-driven boundary tests already exhaust the space. Fuzzing adds complexity for zero additional coverage. |
| `doRequest` marshal failure test | Would require making `SearchRequest` unmarshalable (e.g., adding a `chan` field). Tests Go's `json.Marshal` error path, not meaningful application logic. |
| `http.NewRequestWithContext` failure test | Would require an invalid URL after construction (impossible since `New()` validates) or an invalid context (Go doesn't expose this). Untestable without breaking the type system. |
| Integration test with real Serper API | Requires a real API key, spends credits, needs network. Not suitable for CI. The thin-wrapper nature means the mock-based tests already cover all our code paths. |
| `io.ReadAll` failure test | Would require a reader that errors on read. Possible but contrived — the error path is a simple `fmt.Errorf` wrapper with no branching logic. Minimal regression value. |

---

## Coverage Impact Estimate

| Test | New Branches Covered | Estimated Statement Δ |
|---|---|---|
| T-1: HTTP Status Code Mapping (with typed assertions) | 4 switch cases in `doRequest` | +3-4% |
| T-2: Concurrent Requests | 0 (behavioral, race detector) | +0% |
| T-3: Endpoint Validation Errors | 4 error paths (Images/News/Places/Scholar) | +3-4% |
| T-4: Complete Config Defaults | 0 (CLI pkg, already 0% statement) | +0% |
| T-5: Config Env Overrides | 0 (CLI pkg, already 0% statement) | +0% |
| **Total estimated serper pkg** | **8 new branches** | **~92-94%** |

---

## Summary

| Priority | Test | Value | Files |
|---|---|---|---|
| **High** | T-1: HTTP Status Code Mapping + Typed Assertions | Covers 4 untested error classification branches with type verification | `serper/client_test.go` |
| **High** | T-2: Concurrent Requests (stateless mock) | Race detector protection; prevents future regression if shared state introduced | `serper/client_test.go` |
| **High** | T-3: Endpoint Validation Errors (fresh mock per subtest) | Lifts Images/News/Places/Scholar from 71.4% to 100% | `serper/client_test.go` |
| **Medium** | T-4: Complete Config Defaults (HL, LogLevel, Timeout) | Validates remaining config contract; requires `"time"` import | `cmd/serper/main_test.go` |
| **Medium** | T-5: Config Env Overrides | Tests all struct tags work end-to-end for non-default values | `cmd/serper/main_test.go` |

All high-value tests target real, untested code paths. None test standard library behavior or hypothetical scenarios. The corrected concurrency mock (T-2) and fresh-mock-per-subtest pattern (T-3) fix bugs from prior reports.
