Date Created: [2026-02-17 15:07:59 CET (+0100)]
TOTAL_SCORE: [81]/100

# serper_mod Refactor & Maintainability Review

## Scope
Quick, non-invasive review of the current codebase with emphasis on code quality, duplication, and maintainability. No source code was modified.

Reviewed files:
- `serper/client.go`
- `serper/types.go`
- `serper/client_test.go`
- `cmd/serper/main.go`
- `cmd/serper/main_test.go`
- `go.mod`
- `CHANGELOG.md`

Validation snapshot:
- `go test ./...` passes.
- `go vet ./...` passes.
- `go test ./... -cover` shows:
- `serper`: 86.4% statements
- `cmd/serper`: 0.0% statements
- `gofmt -l` reports formatting drift in:
- `serper/client_test.go`
- `cmd/serper/main.go`

## Score Breakdown (100 points)
- Core design and API boundary: 22/25
- Duplication and structure: 15/25
- Testing and verification: 22/25
- Tooling and maintainability hygiene: 22/25

## What Is Working Well
- Clear client abstraction with injectable transport (`Doer`) for testability in `serper/client.go:25` and `serper/client.go:45`.
- Good request lifecycle discipline: defaulting + validation on a copied request to avoid caller mutation in `serper/client.go:89` and `serper/client.go:91`.
- Defensive response handling with body size cap and error-body truncation in `serper/client.go:199` and `serper/client.go:206`.
- Strong endpoint test coverage in `serper/client_test.go` with broad HTTP and behavior scenarios.
- Changelog/version hygiene exists and appears actively maintained.

## High-Value Refactor Opportunities

### 1) Add nil-request guard before `prepareRequest` dereference
- Location: `serper/client.go:91` (`cp := *req`).
- Issue: passing `nil` request will panic instead of returning a typed/clear error.
- Value: eliminates panic path and hardens API boundary.
- Effort: low.

### 2) Remove endpoint method duplication with a single typed helper
- Locations: `Search`, `Images`, `News`, `Places`, `Scholar` in `serper/client.go:100`, `serper/client.go:113`, `serper/client.go:126`, `serper/client.go:139`, `serper/client.go:152`.
- Issue: same prep + call + decode pattern repeated 5x.
- Value: lower maintenance cost and lower drift risk when adding/modifying endpoints.
- Effort: low to medium.

### 3) Normalize URL joining for base URL overrides
- Location: `serper/client.go:180` (`c.baseURL+endpoint`).
- Issue: manual string concatenation can produce malformed/double-slash paths when custom base URL includes trailing slash/path segments.
- Value: safer custom deployments and fewer subtle request path bugs.
- Effort: low.

### 4) Improve CLI testability by extracting a `run(...)` function
- Location: `cmd/serper/main.go`.
- Issue: logic is embedded in `main()` and exits directly, which drives `cmd/serper` to 0% statement coverage.
- Value: better regression protection for argument handling, error paths, and output formatting.
- Effort: medium.

### 5) Reduce test duplication via table-driven endpoint tests
- Location: repeated success tests in `serper/client_test.go` (endpoint-specific blocks around lines ~196, ~221, ~397, ~420).
- Issue: repeated setup/assert patterns increase file size and maintenance overhead.
- Value: easier future endpoint additions and clearer intent.
- Effort: medium.

### 6) Enforce formatting/lint in CI to keep codebase mechanically clean
- Evidence: `gofmt -l` currently reports files.
- Value: avoids noisy diffs and style drift; improves review quality.
- Effort: low.

## Maintainability Risks to Track
- Large single test file (`serper/client_test.go`, 725 lines) can become hard to navigate and reason about over time.
- CLI defaults duplicate library defaults (`SERPER_NUM`, `SERPER_GL`, `SERPER_HL` vs `SearchRequest.SetDefaults`), which can drift if one side changes.
- Error-type mapping is implemented, but tests mostly assert by string content instead of specific error category behavior.

## Suggested Refactor Sequence
1. Harden request boundary (nil guard).
2. Introduce common typed endpoint helper and migrate all 5 endpoint methods.
3. Normalize URL joining.
4. Extract `run(args, env, stdout, stderr)` from CLI and add behavior tests.
5. Convert repeated endpoint tests to compact table-driven form.
6. Add/enable formatting and lint checks in CI.

## Expected Score Lift After Refactor
If the six items above are completed, expected score improvement is roughly +10 to +14 points (projected range: 91-95/100), mostly from reduced duplication, improved safety, and better CLI verification.
