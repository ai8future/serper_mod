# Changelog

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
