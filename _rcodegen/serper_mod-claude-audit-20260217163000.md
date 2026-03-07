Date Created: 2026-02-17T16:30:00-05:00
TOTAL_SCORE: 86/100

# Audit Report: serper_mod

**Auditor:** Claude:Opus 4.6
**Version Audited:** 1.4.0
**Module:** `github.com/ai8future/serper_mod`
**Go Version:** 1.25.5
**Chassis Dependency:** chassis-go v5.0.0

---

## Executive Summary

serper_mod is a compact, well-structured Go client library and CLI for the Serper.dev search API. The codebase spans ~490 lines of source across 3 files (client.go, types.go, cmd/serper/main.go) with 775 lines of tests. It supports web, images, news, places, and scholar search endpoints. The code is idiomatic Go with good separation of concerns, defensive coding (request copy-before-mutate, response size limits, JSON validation), and strong library test coverage (86.4%). Key deductions come from a concurrency safety gap on the Client struct, SSRF potential via `WithBaseURL`, 0% CLI coverage, missing CI/CD, and a few validation edge cases.

---

## Score Breakdown

| Category | Max | Score | Notes |
|---|---|---|---|
| **Security** | 25 | 21 | SSRF via WithBaseURL, API key plaintext in memory, no scheme enforcement |
| **Correctness & Bug Risk** | 25 | 22 | Client not concurrency-safe, negative Num passes after SetDefaults, no GL/HL length validation |
| **Code Quality** | 20 | 18 | Clean idiomatic Go, DRY opportunity in 5 endpoint methods, some error wrapping inconsistency |
| **Testing** | 15 | 13 | 86.4% lib coverage, 0% CLI main(), no fuzz or integration tests |
| **Project Hygiene** | 15 | 12 | No CI/CD, no LICENSE, no Makefile/taskfile, coverage files tracked in git |

---

## Detailed Findings

---

### SECURITY

#### SEC-1: SSRF via `WithBaseURL` — Medium
**Location:** `serper/client.go:40-42, 63-65`
**Description:** `WithBaseURL` accepts any URL string. While `url.ParseRequestURI` validates syntax, it does not reject internal/private network addresses (e.g. `http://169.254.169.254/latest/meta-data`, `http://localhost:9200`). If a multi-tenant service allows users to configure the base URL, this becomes an SSRF vector. The current validation also permits non-HTTP schemes like `ftp://` or `file://`.
**Recommendation:** Enforce HTTPS-only (with an explicit opt-out for testing), or at minimum validate the scheme.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -62,6 +62,12 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
 		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
 	}
+	u, _ := url.Parse(c.baseURL)
+	if u != nil && u.Scheme != "https" && u.Scheme != "http" {
+		return nil, fmt.Errorf("serper: base URL must use http or https scheme, got %q", u.Scheme)
+	}
 	return c, nil
 }
```

#### SEC-2: API Key Stored as Plaintext String — Low
**Location:** `serper/client.go:31`
**Description:** The `apiKey` field is a plain `string`. In Go, strings are immutable and cannot be zeroed after use, meaning the API key persists in process memory until garbage collected. This is a known Go limitation and not easily avoidable, but is worth noting for high-security deployments.
**Recommendation:** Document this limitation. For future consideration, accept a `func() string` callback that retrieves the key on demand from a secure source (vault, env re-read).

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -29,6 +29,7 @@ type Doer interface {

 // Client is a Serper.dev API client.
+// Note: the API key is held in memory as a Go string and cannot be securely zeroed.
 type Client struct {
 	apiKey  string
 	baseURL string
```

#### SEC-3: No Rate Limiting or Request Throttling — Low
**Location:** `serper/client.go` (general)
**Description:** The client has no built-in rate limiting. While the CLI uses `call.WithRetry`, the library itself does not protect against accidental API key exhaustion from tight loops. The server returns 429 which is mapped to `RateLimitError`, but there is no client-side prevention.
**Recommendation:** Consider documenting that callers should implement their own rate limiting, or add an optional `WithRateLimit` option.

No diff — documentation-only recommendation.

---

### CORRECTNESS & BUG RISK

#### BUG-1: Client Struct is Not Concurrency-Safe — Medium
**Location:** `serper/client.go:30-34`
**Description:** The `Client` struct stores `apiKey`, `baseURL`, and `doer` as simple fields with no synchronization. While these are set at construction time and not modified after, Go's memory model does not guarantee visibility of writes to fields across goroutines unless there is a happens-before relationship. In practice, sharing a `*Client` across goroutines is safe because `New()` returns after all writes complete, and no methods mutate the struct. However, this is undocumented and the struct is technically open to mutation via the exported `Option` pattern if someone calls options post-construction.
**Recommendation:** Document that `Client` is safe for concurrent use after construction. Consider making fields private (they already are) and ensuring no mutation after `New`.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -29,6 +29,8 @@ type Doer interface {

 // Client is a Serper.dev API client.
+// A Client is safe for concurrent use by multiple goroutines after creation.
+// Do not modify a Client after calling New.
 type Client struct {
 	apiKey  string
 	baseURL string
```

#### BUG-2: Negative Num Bypasses Validation via SetDefaults — Low
**Location:** `serper/types.go:148-149, 168-169`
**Description:** `SetDefaults` only fills `Num` when it is `0`. A caller passing `Num: -5` will skip the default assignment but then fail validation (`Num < 1`). This is correct behavior — it properly rejects invalid input. However, the zero-value semantic is ambiguous: `Num: 0` is treated as "use default" rather than an error, which could surprise callers who explicitly set `Num: 0`.
**Recommendation:** Document that `Num: 0` means "use default (10)" rather than an error.

```diff
--- a/serper/types.go
+++ b/serper/types.go
@@ -145,6 +145,7 @@ type ScholarResult struct {

 // SetDefaults applies default values to a SearchRequest.
+// Zero values for Num, GL, HL, and Page are treated as "not set" and filled with defaults.
 func (r *SearchRequest) SetDefaults() {
 	if r.Num == 0 {
 		r.Num = 10
```

#### BUG-3: No Validation on GL/HL Length — Low
**Location:** `serper/types.go:164-175`
**Description:** `GL` and `HL` are locale/language codes that should be 2-character ISO codes (e.g. "us", "en"). The `Validate` method does not check their format or length. Arbitrary strings like `GL: "this is not a country code"` are accepted and sent to the API.
**Recommendation:** Add basic length validation (2-5 chars) or regex check for ISO format.

```diff
--- a/serper/types.go
+++ b/serper/types.go
@@ -172,5 +172,11 @@ func (r *SearchRequest) Validate() error {
 	if r.Page < 1 {
 		return fmt.Errorf("page must be 1 or greater")
 	}
+	if len(r.GL) > 5 {
+		return fmt.Errorf("gl must be a valid country code (max 5 characters)")
+	}
+	if len(r.HL) > 5 {
+		return fmt.Errorf("hl must be a valid language code (max 5 characters)")
+	}
 	return nil
 }
```

#### BUG-4: CheckConnectivity Costs API Credits — Low
**Location:** `serper/client.go:167-171`
**Description:** `CheckConnectivity` performs a real search (`Q: "test", Num: 1`) that consumes API credits. This is documented in the comment, which is good, but callers may not notice the doc comment. There is no free health-check endpoint on Serper.dev, so this is unavoidable.
**Recommendation:** Already documented — no change needed. Consider making the function name more explicit, e.g. `CheckConnectivityWithSearch`.

No diff — documentation is adequate.

---

### CODE QUALITY

#### CQ-1: Repetitive Endpoint Methods — Low
**Location:** `serper/client.go:101-163`
**Description:** `Search`, `Images`, `News`, `Places`, `Scholar` are nearly identical: call `prepareRequest`, call `doRequest` with a different endpoint and response type. The only variation is the endpoint path and response type. This is 5 copies of the same 7-line pattern.
**Recommendation:** Consider a generic helper. However, since Go generics would require Go 1.18+ (already met) and the current approach is clear and explicit, this is a minor style concern, not a bug.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -99,6 +99,17 @@ func prepareRequest(req *SearchRequest) (*SearchRequest, error) {
 	return &cp, nil
 }

+// doSearch is a generic helper that validates, sends, and decodes a search request.
+func doSearch[T any](c *Client, ctx context.Context, endpoint string, req *SearchRequest) (*T, error) {
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
-func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
-	prepared, err := prepareRequest(req)
-	if err != nil {
-		return nil, err
-	}
-	var resp SearchResponse
-	if err := c.doRequest(ctx, "/search", prepared, &resp); err != nil {
-		return nil, err
-	}
-	return &resp, nil
+func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
+	return doSearch[SearchResponse](c, ctx, "/search", req)
 }
```

#### CQ-2: Error Wrapping Inconsistency — Low
**Location:** `serper/client.go:176-177 vs 209-222`
**Description:** Some errors use `fmt.Errorf` with `%w` wrapping (lines 176, 181, 195, 201, 231), while HTTP status errors use chassis error constructors that accept a string detail. This means HTTP errors are typed (via chassis) but not wrapped with `%w`, while transport errors are wrapped but untyped. This is intentional for the chassis error taxonomy but creates an inconsistent `errors.Is`/`errors.As` experience for callers.
**Recommendation:** Document which errors support `errors.Is` vs `errors.As` with chassis types.

No diff — documentation-only.

#### CQ-3: `maxResponseBytes` Read Without Draining — Low
**Location:** `serper/client.go:199`
**Description:** `io.LimitReader` limits the read to 10MB, but if the actual response is larger, the remaining bytes are left unread. In HTTP/1.1, this can prevent connection reuse since the body isn't fully drained. For HTTP/2 (likely used with google.serper.dev), this is less of an issue as streams can be reset.
**Recommendation:** Consider draining or discarding remaining bytes after the read, or using `http.MaxBytesReader` which returns an error.

```diff
--- a/serper/client.go
+++ b/serper/client.go
@@ -196,7 +196,9 @@ func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBo
 	}
 	defer resp.Body.Close()

-	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
+	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
+	body, err := io.ReadAll(limited)
 	if err != nil {
 		return fmt.Errorf("serper: read response: %w", err)
 	}
+	if int64(len(body)) > maxResponseBytes {
+		return fmt.Errorf("serper: response body exceeds %d bytes", maxResponseBytes)
+	}
```

---

### TESTING

#### TEST-1: CLI main() Has 0% Coverage — Medium
**Location:** `cmd/serper/main.go:31-78`
**Description:** The test file `cmd/serper/main_test.go` only tests `Config` struct loading via `chassisconfig.MustLoad`. The actual `main()` function (query parsing, client creation, search execution, JSON output) has 0% test coverage.
**Recommendation:** Extract the core logic from `main()` into a testable `run(args []string, stdout, stderr io.Writer) error` function. Test the extracted function with mock doers.

```diff
--- a/cmd/serper/main.go
+++ b/cmd/serper/main.go
@@ -29,8 +29,9 @@ type Config struct {
 }

-func main() {
+func run(args []string, stdout, stderr io.Writer) error {
 	chassis.RequireMajor(5)
 	cfg := chassisconfig.MustLoad[Config]()
 	logger := logz.New(cfg.LogLevel)
 	logger.Info("starting", "chassis_version", chassis.Version)

-	if len(os.Args) < 2 {
-		fmt.Fprintln(os.Stderr, "usage: serper <query>")
-		os.Exit(1)
+	if len(args) < 2 {
+		return fmt.Errorf("usage: serper <query>")
 	}
-	query := strings.Join(os.Args[1:], " ")
+	query := strings.Join(args[1:], " ")

 	// ... rest of logic using stdout/stderr instead of os.Stdout/os.Stderr ...
+}
+
+func main() {
+	if err := run(os.Args, os.Stdout, os.Stderr); err != nil {
+		fmt.Fprintln(os.Stderr, "error:", err)
+		os.Exit(1)
+	}
 }
```

#### TEST-2: No Error-Path Tests for Images/News/Places/Scholar — Low
**Location:** `serper/client_test.go`
**Description:** Error-path tests (validation errors, HTTP errors, doer errors) only target `Search`. The `Images`, `News`, `Places`, and `Scholar` methods only have success-path tests. While they share the same `prepareRequest`/`doRequest` code path, an accidental regression in one method would go undetected.
**Recommendation:** Add at least one validation-error test for each non-Search endpoint.

```diff
--- a/serper/client_test.go
+++ b/serper/client_test.go
@@ -441,6 +441,30 @@ func TestScholar_Success(t *testing.T) {
 	}
 }

+func TestImages_ValidationError(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"images":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+	_, err := c.Images(context.Background(), &SearchRequest{Q: ""})
+	if err == nil {
+		t.Fatal("expected validation error for empty query")
+	}
+	if mock.req != nil {
+		t.Error("no HTTP request should have been made for invalid input")
+	}
+}
+
+func TestNews_ValidationError(t *testing.T) {
+	mock := &mockDoer{statusCode: 200, respBody: `{"news":[]}`}
+	c := mustNew(t, "key", WithDoer(mock))
+	_, err := c.News(context.Background(), &SearchRequest{Q: ""})
+	if err == nil {
+		t.Fatal("expected validation error for empty query")
+	}
+	if mock.req != nil {
+		t.Error("no HTTP request should have been made for invalid input")
+	}
+}
```

#### TEST-3: No Fuzz Tests — Low
**Location:** N/A
**Description:** No fuzz tests exist for `SearchRequest.Validate()` or JSON unmarshaling of response types. Fuzz testing could uncover edge cases in validation logic or unexpected panics during deserialization.
**Recommendation:** Add basic fuzz targets.

```diff
--- /dev/null
+++ b/serper/client_test.go (append)
@@ -726,0 +727,15 @@
+func FuzzSearchRequest_Validate(f *testing.F) {
+	f.Add("test", 10, "us", "en", 1)
+	f.Add("", 0, "", "", 0)
+	f.Add("q", -1, "xx", "yy", -1)
+	f.Fuzz(func(t *testing.T, q string, num int, gl, hl string, page int) {
+		r := &SearchRequest{Q: q, Num: num, GL: gl, HL: hl, Page: page}
+		r.SetDefaults()
+		_ = r.Validate() // must not panic
+	})
+}
```

---

### PROJECT HYGIENE

#### HYG-1: No CI/CD Pipeline — Medium
**Location:** N/A
**Description:** There are no GitHub Actions workflows, Makefile, or taskfile for automated testing, linting, or building. All quality assurance depends on manual execution.
**Recommendation:** Add a `.github/workflows/ci.yml` with `go test -race ./...`, `go vet ./...`, and optionally `staticcheck`.

#### HYG-2: No LICENSE File — Medium
**Location:** N/A
**Description:** The repository has no LICENSE file. This means the code is technically "all rights reserved" by default, which may not be the intent for a GitHub-hosted module.
**Recommendation:** Add an appropriate LICENSE file (MIT, Apache-2.0, etc.).

#### HYG-3: Coverage Output Files Tracked in Working Directory — Low
**Location:** `coverage.out`, `cmd_coverage.out`
**Description:** Coverage output files are present in the repo root and appear in `git status` as untracked/modified. They should either be gitignored or generated into a temporary directory.
**Recommendation:** Add `*.out` or `coverage.out` to `.gitignore`.

```diff
--- a/.gitignore
+++ b/.gitignore
@@ -46,3 +46,6 @@
 # Project-specific
 /cmd/serper/serper
+
+# Test coverage
+*.out
```

#### HYG-4: `go.work` Gitignored but Present — Informational
**Location:** `go.work`, `.gitignore:29-30`
**Description:** `go.work` and `go.work.sum` are listed in `.gitignore` but exist in the working tree. This is expected for local development workspace files, but they show up in `git status` differently depending on whether they were added before the gitignore rule.
**Recommendation:** No action needed — this is standard Go workspace practice.

---

## Test Results Summary

```
$ go test ./... -count=1 -race
ok  github.com/ai8future/serper_mod/cmd/serper    1.388s
ok  github.com/ai8future/serper_mod/serper         1.682s

$ go vet ./...
(clean — no issues)
```

### Coverage by Function

| Function | Coverage |
|---|---|
| `WithBaseURL` | 100.0% |
| `WithDoer` | 100.0% |
| `New` | 100.0% |
| `WithAPIKey` | 100.0% |
| `getAPIKey` | 100.0% |
| `prepareRequest` | 100.0% |
| `Search` | 100.0% |
| `Images` | 71.4% |
| `News` | 71.4% |
| `Places` | 71.4% |
| `Scholar` | 71.4% |
| `CheckConnectivity` | 100.0% |
| `doRequest` | 79.4% |
| `SetDefaults` | 100.0% |
| `Validate` | 100.0% |
| **Total** | **86.4%** |

CLI `main()`: **0.0%**

---

## Consolidated Patch

Below is a single combined diff of all recommended changes. Apply with `git apply`:

```diff
diff --git a/serper/client.go b/serper/client.go
index abc1234..def5678 100644
--- a/serper/client.go
+++ b/serper/client.go
@@ -29,6 +29,8 @@ type Doer interface {

 // Client is a Serper.dev API client.
+// A Client is safe for concurrent use by multiple goroutines after creation.
+// Note: the API key is held in memory as a Go string and cannot be securely zeroed.
 type Client struct {
 	apiKey  string
 	baseURL string
@@ -62,6 +64,10 @@ func New(apiKey string, opts ...Option) (*Client, error) {
 	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
 		return nil, fmt.Errorf("serper: invalid base URL %q: %w", c.baseURL, err)
 	}
+	if u, _ := url.Parse(c.baseURL); u != nil && u.Scheme != "https" && u.Scheme != "http" {
+		return nil, fmt.Errorf("serper: base URL must use http or https scheme, got %q", u.Scheme)
+	}
 	return c, nil
 }

diff --git a/serper/types.go b/serper/types.go
index abc1234..def5678 100644
--- a/serper/types.go
+++ b/serper/types.go
@@ -145,6 +145,7 @@ type ScholarResult struct {

 // SetDefaults applies default values to a SearchRequest.
+// Zero values for Num, GL, HL, and Page are treated as "not set" and filled with defaults.
 func (r *SearchRequest) SetDefaults() {
 	if r.Num == 0 {
 		r.Num = 10
@@ -172,5 +173,11 @@ func (r *SearchRequest) Validate() error {
 	if r.Page < 1 {
 		return fmt.Errorf("page must be 1 or greater")
 	}
+	if len(r.GL) > 5 {
+		return fmt.Errorf("gl must be a valid country code (max 5 characters)")
+	}
+	if len(r.HL) > 5 {
+		return fmt.Errorf("hl must be a valid language code (max 5 characters)")
+	}
 	return nil
 }

diff --git a/.gitignore b/.gitignore
index abc1234..def5678 100644
--- a/.gitignore
+++ b/.gitignore
@@ -46,3 +46,6 @@
 # Project-specific
 /cmd/serper/serper
+
+# Test coverage
+*.out
```

---

## Risk Matrix

| ID | Severity | Category | Finding | Exploitable? |
|---|---|---|---|---|
| SEC-1 | Medium | Security | SSRF via WithBaseURL | Only if base URL is user-configurable |
| SEC-2 | Low | Security | API key plaintext in memory | Requires memory dump access |
| SEC-3 | Low | Security | No client-side rate limiting | Self-inflicted API exhaustion |
| BUG-1 | Medium | Correctness | Undocumented concurrency safety | No — safe in practice |
| BUG-2 | Low | Correctness | Num=0 semantics ambiguous | No — just confusing |
| BUG-3 | Low | Correctness | No GL/HL length validation | Garbage in, garbage out |
| BUG-4 | Low | Correctness | CheckConnectivity costs credits | By design |
| CQ-1 | Low | Quality | Repetitive endpoint methods | Not a bug |
| CQ-2 | Low | Quality | Error wrapping inconsistency | Not a bug |
| CQ-3 | Low | Quality | LimitReader without drain | Connection reuse impact |
| TEST-1 | Medium | Testing | 0% CLI coverage | Untested code paths |
| TEST-2 | Low | Testing | No error tests for non-Search endpoints | Partial coverage gap |
| TEST-3 | Low | Testing | No fuzz tests | Missing hardening |
| HYG-1 | Medium | Hygiene | No CI/CD | Manual-only QA |
| HYG-2 | Medium | Hygiene | No LICENSE file | Legal ambiguity |
| HYG-3 | Low | Hygiene | Coverage files in repo | Clutter |

---

## Overall Assessment

**Grade: 86/100** — This is a well-written, focused Go client library. The code demonstrates strong fundamentals: proper request copying to avoid caller mutation, response body size limiting, JSON safety validation via secval, typed error mapping from HTTP status codes, context propagation, and retry body support. The main areas for improvement are around hardening (scheme validation, GL/HL validation), test coverage expansion (CLI main, error paths for non-Search endpoints), and project infrastructure (CI/CD, LICENSE). No critical bugs or high-severity security issues were found.
