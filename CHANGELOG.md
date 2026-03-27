# Changelog

## [1.8.7] - 2026-03-27
- test: add HTTP status code mapping tests (400, 403, 404, 429, 502)
- test: add concurrent client usage test with stateless mock and race detector
- test: add nil request guard test
- test: add nil doer guard test
- test: add trailing slash normalization test
- test: add response size overflow test
- test: add whitespace query validation test
- Coverage: serper pkg 86.1% → 93.1%
- Agent: Claude:Opus 4.6

## [1.8.6] - 2026-03-27
- fix: nil request guard in prepareRequest (prevents panic on nil *SearchRequest)
- fix: nil doer guard in New() (prevents deferred panic when WithDoer(nil) is passed)
- fix: trailing slash normalization on baseURL (prevents double-slash URLs)
- fix: HTTP 403 Forbidden now maps to ForbiddenError instead of InternalError
- fix: response size overflow detection (clear error instead of misleading JSON parse failure)
- fix: whitespace-only queries now rejected by Validate()
- fix: validation errors now use typed chassiserrors.ValidationError for consistency
- fix: error body sanitized for log injection (control chars replaced with spaces)
- refactor: generic doSearch[T] helper eliminates 7x endpoint method duplication
- feat: CLI now exposes SERPER_LOCATION env var for geo-targeted searches
- chore: add *.out to .gitignore for coverage file hygiene
- chore: fix pre-existing gofmt drift in client_test.go
- Agent: Claude:Opus 4.6

## [1.8.5] - 2026-03-26
- GO-BEST-PRACTICES conformance: Makefile with cross-platform build targets (build-linux, build-darwin, build-all), launcher script, binary naming, LDFLAGS with version injection, CGO_ENABLED=0 static builds
- Agent: Claude:Opus 4.6

## [1.8.4] - 2026-03-22
- Upgrade chassis-go from v9 to v10: update all import paths, go.mod require/replace, RequireMajor(10), VERSION.chassis
- Agent: Claude:Opus 4.6

## [1.8.3] - 2026-03-08
- Upgrade chassis-go from v8 to v9: update all import paths, go.mod require/replace, RequireMajor(9), VERSION.chassis
- Agent: Claude:Opus 4.6

## [1.8.2] - 2026-03-08
- Fix go.mod version: correct chassis-go/v8 version from v7.0.0 to v8.0.0
- (Claude Code:Opus 4.6)

## [1.8.1] - 2026-03-07
- Sync uncommitted changes

## 1.8.0

- feat: upgrade chassis-go from v5.0.0 to v6.0.0
- chore: migrate all imports from `github.com/ai8future/chassis-go/v5` to `github.com/ai8future/chassis-go/v6`
- chore: update version gate from `RequireMajor(5)` to `RequireMajor(6)` in main.go and main_test.go
- chore: update go.mod dependency to `github.com/ai8future/chassis-go/v6 v6.0.0` (Claude:Opus 4.6)


## 1.7.0

- feat: add Shopping and Videos search support — `ShoppingResponse`, `ShoppingResult`, `VideosResponse`, `VideoResult` types in types.go; `Shopping()` and `Videos()` client methods hitting `/shopping` and `/videos` endpoints (Claude:Sonnet 4.6)

## 1.6.0

- docs: rewrite README.md with deeper architecture analysis, detailed data flow diagram, expanded coverage documentation, local development instructions, and design decision rationale (Claude:Opus 4.6)

## 1.5.0

- docs: add comprehensive README.md with full API reference, architecture diagram, usage examples, error handling guide, and CLI configuration (Claude:Opus 4.6)

## 1.4.0

- feat: upgrade chassis-go from v4.0.0 to v5.0.0
- chore: migrate all imports from `github.com/ai8future/chassis-go` to `github.com/ai8future/chassis-go/v5`
- chore: update version gate from `RequireMajor(4)` to `RequireMajor(5)` in main.go and main_test.go
- chore: update go.mod dependency to `github.com/ai8future/chassis-go/v5 v5.0.0`
- chore: write VERSION.chassis file (5.0.0) (Claude:Opus 4.6)

## 1.2.0

- fix(breaking): `New()` now returns `(*Client, error)` and rejects empty API keys
- fix: `New()` validates base URL after options are applied (catches invalid URLs at construction)
- fix: Client methods no longer mutate the caller's `*SearchRequest` (copy before applying defaults)
- fix: CLI no longer applies a redundant `context.WithTimeout` that could conflict with retry timeouts
- fix: `CheckConnectivity` doc now notes it makes a billable API request
- test: Added `TestNew_EmptyAPIKey`, `TestNew_InvalidBaseURL` for constructor validation
- test: Added `TestPlaces_Success`, `TestScholar_Success` for previously uncovered endpoints
- test: Added `TestSearch_DoesNotMutateRequest` to verify no caller side effects
- test: Added `TestSearch_ContextCancellation`, `TestSearch_MalformedJSONResponse`
- test: Added `TestSearch_ErrorBodyTruncation`, `TestSearch_HTTP500`, `TestSearch_HTTP503`
- test: Added `TestSetDefaults_PreservesExplicitValues`, `TestValidate_BoundaryValues`
- test: Added `TestCheckConnectivity_Success`, `TestCheckConnectivity_Failure`
- test: Added `TestSearch_EmptyResponseBody`, `TestDoRequest_UsesPostMethod`
- test: Added `TestSearch_OmitsEmptyLocationFromBody`, `TestSearch_IncludesLocationWhenSet`
- test: Added `TestWithAPIKey_InnerOverridesOuter`, `TestNew_LastOptionWins`

## 1.1.0

- fix: Validate() now rejects Num < 1 and Page < 1 (previously allowed 0)
- fix: Error message for Num validation now matches actual boundary check
- fix: Client methods (Search, Images, News, Places, Scholar) now call SetDefaults/Validate before making HTTP requests
- fix: CLI joins all args for multi-word queries (`serper hello world` now works)
- fix: Removed `replace` directive from go.mod; use `go.work` for local dev instead
- fix: Added go.sum for reproducible builds
- fix: Default HTTP client now has 30s timeout instead of no timeout
- fix: Response body reads capped at 10MB via io.LimitReader
- fix: Error response bodies truncated to 1KB in error messages
- fix: Updated `interface{}` to `any` in doRequest signature
- test: Added tests for client-level validation, defaults application, Num=0 and Page=0 rejection
- test: Updated TestNew_Defaults to verify timeout on default HTTP client

## 1.0.0

- Initial project setup with VERSION, CHANGELOG, AGENTS.md, and standard directories
- Existing code: Go module with serper package and CLI demo
