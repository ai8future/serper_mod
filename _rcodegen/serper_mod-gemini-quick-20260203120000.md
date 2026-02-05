Date Created: 2026-02-03 12:00:00
TOTAL_SCORE: 95/100

## 1. AUDIT

The codebase is well-structured, follows idiomatic Go patterns, and has excellent test coverage. The use of functional options for configuration and interfaces for testing (`Doer`) is commendable.

**Issues Found:**
1.  **Missing API Parameters**: The `SearchRequest` struct lacks support for common search parameters like `tbs` (time range) and `safe` (safe search filtering). This limits the library's utility for users requiring time-boxed or safe results.

## 2. TESTS

We should verify that the new parameters are correctly marshaled into the request body.

**serper/client_test.go**
```go
<<<<
func TestSearch_IncludesLocationWhenSet(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t, "key", WithDoer(mock))

	_, _ = c.Search(context.Background(), &SearchRequest{Q: "test", Location: "New York"})

	var sent map[string]any
	if err := json.Unmarshal(mock.body, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	loc, exists := sent["location"]
	if !exists {
		t.Fatal("location should be present in JSON body")
	}
	if loc != "New York" {
		t.Errorf("location: got %q, want %q", loc, "New York")
	}
}

func TestWithAPIKey_InnerOverridesOuter(t *testing.T) {
====
func TestSearch_IncludesLocationWhenSet(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t, "key", WithDoer(mock))

	_, _ = c.Search(context.Background(), &SearchRequest{Q: "test", Location: "New York"})

	var sent map[string]any
	if err := json.Unmarshal(mock.body, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	loc, exists := sent["location"]
	if !exists {
		t.Fatal("location should be present in JSON body")
	}
	if loc != "New York" {
		t.Errorf("location: got %q, want %q", loc, "New York")
	}
}

func TestSearch_IncludesTbsAndSafe(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t, "key", WithDoer(mock))

	_, _ = c.Search(context.Background(), &SearchRequest{
		Q:    "test",
		Tbs:  "qdr:w",
		Safe: "active",
	})

	var sent map[string]any
	if err := json.Unmarshal(mock.body, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	if got, _ := sent["tbs"].(string); got != "qdr:w" {
		t.Errorf("tbs: got %q, want %q", got, "qdr:w")
	}
	if got, _ := sent["safe"].(string); got != "active" {
		t.Errorf("safe: got %q, want %q", got, "active")
	}
}

func TestWithAPIKey_InnerOverridesOuter(t *testing.T) {
>>>>
```

## 3. FIXES

Add the missing fields to `SearchRequest`.

**serper/types.go**
```go
<<<<
	HL       string `json:"hl,omitempty"`
	Location string `json:"location,omitempty"`
	Page     int    `json:"page,omitempty"`
}

// SearchResponse represents the response from Serper.dev search endpoint.
====
	HL       string `json:"hl,omitempty"`
	Location string `json:"location,omitempty"`
	Page     int    `json:"page,omitempty"`
	Tbs      string `json:"tbs,omitempty"`  // Time range (e.g., "qdr:d" for past day)
	Safe     string `json:"safe,omitempty"` // Safe search ("active" or "off")
}

// SearchResponse represents the response from Serper.dev search endpoint.
>>>>
```

## 4. REFACTOR

1.  **Error Handling**: The `doRequest` method manually constructs an error message from the response body. This could be extracted into a `parseError(resp *http.Response) error` helper function to keep `doRequest` cleaner and potentially support typed error responses if the API provides structured error JSON.
2.  **Validation**: `Validate()` could be expanded to check against a known list of valid `GL` (Country) and `HL` (Language) codes, although this might be too restrictive and high-maintenance.
3.  **Constants**: The default values in `SetDefaults` (like `10` for Num) could be extracted as exported constants (e.g., `DefaultResultsNum`) to allow users to reference them.
