# Changelog

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
