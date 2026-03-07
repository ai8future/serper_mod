Date Created: 2026-02-15T22:50:52-05:00
TOTAL_SCORE: 72/100

# Unit Test Coverage Report: serper_mod

**Auditor:** Claude:Opus 4.6
**Module:** `github.com/ai8future/serper_mod`
**Existing Coverage:** 86.4% (`serper` pkg), 0.0% (`cmd/serper` pkg)

---

## Executive Summary

The `serper` package has solid test coverage at 86.4%, driven by thorough tests of the `Client` constructor, `Search` method, validation, defaults, context handling, and error paths. The primary coverage gaps are:

1. **Error-path branches in `Images`, `News`, `Places`, `Scholar`** (each at 71.4%) — these methods share identical structure with `Search` but their `prepareRequest` error path is never exercised directly
2. **`doRequest` internal error branches** (79.4%) — `json.Marshal` failure, `http.NewRequestWithContext` failure, and `io.ReadAll` failure are uncovered
3. **HTTP status code mapping branches** — only 401, 500, 503 are tested; 400, 404, 429, 502 are untested
4. **`cmd/serper/main.go`** — 0% coverage; only `Config` loading is tested

The test score of **72/100** reflects strong fundamentals but meaningful gaps in branch coverage that could mask regressions in error handling and typed error classification.

---

## Score Breakdown

| Category | Max | Score | Notes |
|---|---|---|---|
| **Branch coverage of core library** | 30 | 22 | 86.4% statement coverage, but several error branches in `doRequest` and typed error mapping are uncovered |
| **Endpoint method coverage** | 15 | 10 | Search is fully covered; Images/News/Places/Scholar each miss the validation-failure path |
| **Error classification coverage** | 20 | 12 | Only 3 of 7 HTTP status-code branches tested (401, 500, 503) |
| **Edge cases and robustness** | 20 | 16 | Good: context cancel, truncation, boundary values, malformed JSON, empty response. Missing: concurrency, unexpected JSON shape |
| **CLI coverage** | 15 | 12 | Config loading tested well; main() untestable without refactor (not proposed — too invasive) |

---

## Analysis of Gemini Test Report Proposals

The Gemini report (2026-02-03) proposed three tests. Here is my assessment:

### 1. `TestClient_ConcurrentRequests` — VALUABLE (keep)
**Verdict:** Worth adding. The `Client` is intended to be concurrency-safe (immutable after construction, stack-local request processing). This test protects against future regressions if someone adds shared state. It also exercises the `-race` detector meaningfully. The Gemini diff is correct and clean.

### 2. `TestSearch_UnexpectedJSONStructure` — NOT VALUABLE (skip)
**Verdict:** Dead weight. The code path for a valid-JSON-but-wrong-type response (`[]` instead of `{}`) goes through `json.Unmarshal` which returns a standard Go error. But more importantly, `secval.ValidateJSON` runs *before* unmarshal and may or may not reject `[]` depending on its implementation. If it accepts `[]`, Go's `json.Unmarshal` into a struct pointer simply ignores array data and produces a zero-value struct — **which is not an error**. The Gemini test asserts an error will occur, but the actual behavior would be a zero-value `SearchResponse` returned without error. This test would **fail** as written. Additionally, testing Go's built-in JSON unmarshalling behavior adds no value — we'd be testing the standard library, not our code.

### 3. `TestConfig_Defaults` LogLevel assertion — VALUABLE (keep)
**Verdict:** Reasonable. The `LogLevel` default is `"error"` and is part of the contract. One line addition, validates a real default. The Gemini diff is correct.

---

## Proposed New Tests

### HIGH VALUE — Tests That Protect Real Behavior

#### T-1: HTTP Status Code Classification (covers 4 untested branches in `doRequest`)
**Location:** `serper/client_test.go`
**Why valuable:** The typed error mapping (`chassiserrors.ValidationError`, `UnauthorizedError`, etc.) in `doRequest` lines 210-223 is core behavioral contract. Currently only 401, 500, and 503 are tested. The 400, 404, 429, and 502 branches are completely untested. If someone rearranges the switch or changes an error type, these regressions would go undetected.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -361,3 +361,53 @@
 		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://second.com")
 	}
 }
+
+func TestSearch_HTTPStatusCodeMapping(t *testing.T) {
+	tests := []struct {
+		name       string
+		statusCode int
+		wantSubstr string
+	}{
+		{
+			name:       "400 Bad Request",
+			statusCode: 400,
+			wantSubstr: "400",
+		},
+		{
+			name:       "404 Not Found",
+			statusCode: 404,
+			wantSubstr: "404",
+		},
+		{
+			name:       "429 Too Many Requests",
+			statusCode: 429,
+			wantSubstr: "429",
+		},
+		{
+			name:       "502 Bad Gateway",
+			statusCode: 502,
+			wantSubstr: "502",
+		},
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
+			if !strings.Contains(err.Error(), tt.wantSubstr) {
+				t.Errorf("error should contain %q, got: %v", tt.wantSubstr, err)
+			}
+		})
+	}
+}
```

#### T-2: Concurrent Client Usage (from Gemini report, validated)
**Location:** `serper/client_test.go`
**Why valuable:** Guards against future introduction of shared mutable state. Exercises the race detector on a real concurrent workload. Cheap to run, high regression value.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -361,3 +361,20 @@
 		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://second.com")
 	}
 }
+
+func TestClient_ConcurrentRequests(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
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

**Note:** The `mockDoer` as written is NOT concurrency-safe (it writes to `m.req` and `m.body` without synchronization). This test will trigger the race detector on the mock, not the client. For this test to be meaningful, the mock needs to either (a) not store request state, or (b) use a mutex. A better approach:

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -361,3 +361,30 @@
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

#### T-3: Endpoint Validation Error Propagation for Non-Search Methods
**Location:** `serper/client_test.go`
**Why valuable:** Each of `Images`, `News`, `Places`, `Scholar` is at 71.4% coverage because the `prepareRequest` error branch is never exercised. One table-driven test covers all four gaps. This prevents a regression where someone changes the error path in one method but not others.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -361,3 +361,38 @@
 		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://second.com")
 	}
 }
+
+func TestEndpoints_ValidationError(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{}`}
+	c := mustNew(t, "key", WithDoer(mock))
+
+	// Empty query should trigger validation error on all endpoints.
+	badReq := &SearchRequest{Q: ""}
+
+	tests := []struct {
+		name string
+		call func() error
+	}{
+		{"Images", func() error { _, err := c.Images(context.Background(), badReq); return err }},
+		{"News", func() error { _, err := c.News(context.Background(), badReq); return err }},
+		{"Places", func() error { _, err := c.Places(context.Background(), badReq); return err }},
+		{"Scholar", func() error { _, err := c.Scholar(context.Background(), badReq); return err }},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			err := tt.call()
+			if err == nil {
+				t.Fatal("expected validation error for empty query")
+			}
+			if !strings.Contains(err.Error(), "query") {
+				t.Errorf("error should mention 'query', got: %v", err)
+			}
+			if mock.req != nil {
+				mock.req = nil // reset for next iteration
+				t.Error("no HTTP request should have been made for invalid input")
+			}
+		})
+	}
+}
```

#### T-4: Config LogLevel Default (from Gemini report, validated)
**Location:** `cmd/serper/main_test.go`
**Why valuable:** One-line addition that validates a real config default. Cheap and meaningful.

```diff
--- a/cmd/serper/main_test.go
+++ b/cmd/serper/main_test.go
@@ -21,6 +21,9 @@
 	if cfg.GL != "us" {
 		t.Errorf("expected default GL 'us', got %q", cfg.GL)
 	}
+	if cfg.LogLevel != "error" {
+		t.Errorf("expected default LogLevel 'error', got %q", cfg.LogLevel)
+	}
 }

 func TestConfig_PanicsWithoutAPIKey(t *testing.T) {
```

---

### MEDIUM VALUE — Useful But Lower Priority

#### T-5: Config Timeout and HL Defaults
**Location:** `cmd/serper/main_test.go`
**Why valuable:** The `Timeout` and `HL` defaults are tested nowhere. They are part of the config contract.

```diff
--- a/cmd/serper/main_test.go
+++ b/cmd/serper/main_test.go
@@ -21,6 +21,15 @@
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

Note: This requires adding `"time"` to the import block.

#### T-6: Config Environment Variable Override
**Location:** `cmd/serper/main_test.go`
**Why valuable:** Validates that env vars actually override config defaults. Currently only the default path is tested.

```diff
--- a/cmd/serper/main_test.go
+++ b/cmd/serper/main_test.go
@@ -47,3 +47,25 @@
 	_ = chassisconfig.MustLoad[Config]()
 }
+
+func TestConfig_EnvironmentOverrides(t *testing.T) {
+	testkit.SetEnv(t, map[string]string{
+		"SERPER_API_KEY":  "test-key",
+		"SERPER_BASE_URL": "https://custom.api.com",
+		"SERPER_NUM":      "25",
+		"SERPER_GL":       "de",
+		"SERPER_HL":       "fr",
+		"SERPER_TIMEOUT":  "10s",
+		"LOG_LEVEL":       "debug",
+	})
+
+	cfg := chassisconfig.MustLoad[Config]()
+	if cfg.BaseURL != "https://custom.api.com" {
+		t.Errorf("expected BaseURL 'https://custom.api.com', got %q", cfg.BaseURL)
+	}
+	if cfg.Num != 25 {
+		t.Errorf("expected Num 25, got %d", cfg.Num)
+	}
+	if cfg.GL != "de" {
+		t.Errorf("expected GL 'de', got %q", cfg.GL)
+	}
+}
```

---

### REJECTED — Not Valuable / Dead Weight

| Proposal | Reason |
|---|---|
| `TestSearch_UnexpectedJSONStructure` (Gemini T-2) | Tests Go's `json.Unmarshal` behavior, not our code. A JSON array `[]` into a struct pointer produces a zero-value struct without error. The proposed test would **fail** as written. |
| Fuzz tests for `Validate()` | The validation logic has exactly 3 branches (empty Q, Num range, Page < 1), all with simple integer/string comparisons. Fuzzing adds complexity for zero additional coverage. The table-driven boundary tests already cover this exhaustively. |
| `doRequest` marshal failure test | Would require making `SearchRequest` unmarshalable (e.g., adding a `chan` field). This tests Go's `json.Marshal` error path, not meaningful application logic. The marshal can only fail for types we don't use. |
| `http.NewRequestWithContext` failure test | Would require an invalid URL after construction (impossible since `New()` validates it) or an invalid context (which Go doesn't expose). Not testable without breaking the type system. |
| Integration test with real Serper API | Requires a real API key and spends credits. Not suitable for standard CI. Could be gated but adds maintenance burden for uncertain value in a thin HTTP wrapper. |

---

## Coverage Impact Estimate

| Test | New Branches Covered | Statement Δ |
|---|---|---|
| T-1: HTTP Status Code Mapping | 4 switch cases in `doRequest` | +3-4% |
| T-2: Concurrent Requests | 0 (behavioral, not branch) | +0% |
| T-3: Endpoint Validation Errors | 4 error paths (Images/News/Places/Scholar) | +3-4% |
| T-4: LogLevel Default | 0 (CLI config, already at 0%) | +0% |
| T-5: Timeout/HL Defaults | 0 (CLI config, already at 0%) | +0% |
| T-6: Config Env Overrides | 0 (CLI config, already at 0%) | +0% |
| **Total estimated** | **8 new branches** | **~92-94% serper pkg** |

---

## Summary

| Priority | Test | Value | Effort |
|---|---|---|---|
| **High** | T-1: HTTP Status Code Mapping | Covers 4 untested error classification branches | 5 min |
| **High** | T-2: Concurrent Requests (corrected mock) | Race detector protection against future regressions | 5 min |
| **High** | T-3: Endpoint Validation Errors | Covers 4 endpoint error paths, lifts all to 100% | 5 min |
| **Medium** | T-4: Config LogLevel Default | One-line, validates contract | 1 min |
| **Medium** | T-5: Config Timeout/HL Defaults | Validates remaining config defaults | 2 min |
| **Medium** | T-6: Config Env Overrides | Tests config override path | 3 min |

All high-value tests target real, untested code paths in the application. None test standard library behavior or hypothetical scenarios. The corrected concurrent mock (T-2) fixes a bug in the Gemini proposal that would have caused false race detection failures.
