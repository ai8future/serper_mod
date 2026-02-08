package serper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockDoer captures the request and returns a canned response.
type mockDoer struct {
	req        *http.Request
	body       []byte
	statusCode int
	respBody   string
	err        error
}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	m.req = req
	if req.Body != nil {
		m.body, _ = io.ReadAll(req.Body)
	}
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.respBody)),
	}, nil
}

func mustNew(t *testing.T, apiKey string, opts ...Option) *Client {
	t.Helper()
	c, err := New(apiKey, opts...)
	if err != nil {
		t.Fatalf("New(%q): unexpected error: %v", apiKey, err)
	}
	return c
}

func TestNew_Defaults(t *testing.T) {
	c := mustNew(t, "test-key")
	if c.apiKey != "test-key" {
		t.Errorf("apiKey: got %q, want %q", c.apiKey, "test-key")
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL: got %q, want %q", c.baseURL, defaultBaseURL)
	}
	if c.doer == nil {
		t.Error("doer should not be nil")
	}
	httpClient, ok := c.doer.(*http.Client)
	if !ok {
		t.Fatal("doer should be an *http.Client")
	}
	if httpClient.Timeout != defaultTimeout {
		t.Errorf("timeout: got %v, want %v", httpClient.Timeout, defaultTimeout)
	}
}

func TestNew_EmptyAPIKey(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Errorf("error should mention 'API key', got: %v", err)
	}
}

func TestNew_InvalidBaseURL(t *testing.T) {
	_, err := New("key", WithBaseURL("://bad"))
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
	if !strings.Contains(err.Error(), "invalid base URL") {
		t.Errorf("error should mention 'invalid base URL', got: %v", err)
	}
}

func TestNew_WithOptions(t *testing.T) {
	mock := &mockDoer{}
	c := mustNew(t, "key",
		WithBaseURL("https://example.com"),
		WithDoer(mock),
	)
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://example.com")
	}
	if c.doer != mock {
		t.Error("doer should be the mock")
	}
}

func TestSearch_Success(t *testing.T) {
	respJSON := `{
		"searchParameters": {"q": "golang", "gl": "us", "hl": "en", "num": 10, "type": "search", "engine": "google"},
		"organic": [
			{"title": "Go Programming Language", "link": "https://go.dev", "snippet": "Go is an open source language.", "position": 1}
		]
	}`
	mock := &mockDoer{statusCode: 200, respBody: respJSON}
	c := mustNew(t, "my-api-key", WithDoer(mock), WithBaseURL("https://api.test"))

	resp, err := c.Search(context.Background(), &SearchRequest{Q: "golang"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify parsed response.
	if len(resp.Organic) != 1 {
		t.Fatalf("organic results: got %d, want 1", len(resp.Organic))
	}
	if resp.Organic[0].Title != "Go Programming Language" {
		t.Errorf("title: got %q, want %q", resp.Organic[0].Title, "Go Programming Language")
	}

	// Verify request URL.
	if mock.req == nil {
		t.Fatal("expected request to be captured")
	}
	wantURL := "https://api.test/search"
	if mock.req.URL.String() != wantURL {
		t.Errorf("URL: got %q, want %q", mock.req.URL.String(), wantURL)
	}

	// Verify headers.
	if got := mock.req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", got, "application/json")
	}
	if got := mock.req.Header.Get("X-API-KEY"); got != "my-api-key" {
		t.Errorf("X-API-KEY: got %q, want %q", got, "my-api-key")
	}
}

func TestSearch_HTTPError(t *testing.T) {
	mock := &mockDoer{statusCode: 401, respBody: `{"error":"unauthorized"}`}
	c := mustNew(t, "bad-key", WithDoer(mock))

	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
	if err == nil {
		t.Fatal("expected error for 401 status")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain '401', got: %v", err)
	}
}

func TestSearch_DoerError(t *testing.T) {
	mock := &mockDoer{err: fmt.Errorf("connection refused")}
	c := mustNew(t, "key", WithDoer(mock))

	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
	if err == nil {
		t.Fatal("expected error from doer")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error should propagate doer error, got: %v", err)
	}
}

func TestWithAPIKey_Override(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t,"default-key", WithDoer(mock))

	ctx := WithAPIKey(context.Background(), "override-key")
	_, _ = c.Search(ctx, &SearchRequest{Q: "test"})

	if mock.req == nil {
		t.Fatal("expected request to be captured")
	}
	if got := mock.req.Header.Get("X-API-KEY"); got != "override-key" {
		t.Errorf("X-API-KEY: got %q, want %q", got, "override-key")
	}
}

func TestWithAPIKey_EmptyDoesNotOverride(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t,"default-key", WithDoer(mock))

	ctx := WithAPIKey(context.Background(), "")
	_, _ = c.Search(ctx, &SearchRequest{Q: "test"})

	if mock.req == nil {
		t.Fatal("expected request to be captured")
	}
	if got := mock.req.Header.Get("X-API-KEY"); got != "default-key" {
		t.Errorf("X-API-KEY: got %q, want %q", got, "default-key")
	}
}

func TestImages_Success(t *testing.T) {
	respJSON := `{
		"searchParameters": {"q": "cats", "type": "images"},
		"images": [
			{"title": "Cute Cat", "imageUrl": "https://example.com/cat.jpg", "link": "https://example.com", "position": 1}
		]
	}`
	mock := &mockDoer{statusCode: 200, respBody: respJSON}
	c := mustNew(t,"key", WithDoer(mock), WithBaseURL("https://api.test"))

	resp, err := c.Images(context.Background(), &SearchRequest{Q: "cats"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Images) != 1 {
		t.Fatalf("images: got %d, want 1", len(resp.Images))
	}

	// Verify /images endpoint was called.
	wantURL := "https://api.test/images"
	if mock.req.URL.String() != wantURL {
		t.Errorf("URL: got %q, want %q", mock.req.URL.String(), wantURL)
	}
}

func TestNews_Success(t *testing.T) {
	respJSON := `{
		"searchParameters": {"q": "tech", "type": "news"},
		"news": [
			{"title": "Tech News", "link": "https://example.com/news", "snippet": "Latest tech.", "source": "TechCrunch", "date": "2025-01-01", "position": 1}
		]
	}`
	mock := &mockDoer{statusCode: 200, respBody: respJSON}
	c := mustNew(t,"key", WithDoer(mock), WithBaseURL("https://api.test"))

	resp, err := c.News(context.Background(), &SearchRequest{Q: "tech"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.News) != 1 {
		t.Fatalf("news: got %d, want 1", len(resp.News))
	}

	// Verify /news endpoint was called.
	wantURL := "https://api.test/news"
	if mock.req.URL.String() != wantURL {
		t.Errorf("URL: got %q, want %q", mock.req.URL.String(), wantURL)
	}
}

func TestGetBody_RetrySupport(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t,"key", WithDoer(mock))

	_, _ = c.Search(context.Background(), &SearchRequest{Q: "test"})

	if mock.req == nil {
		t.Fatal("expected request to be captured")
	}
	if mock.req.GetBody == nil {
		t.Fatal("expected GetBody to be set for retry support")
	}

	body, err := mock.req.GetBody()
	if err != nil {
		t.Fatalf("GetBody error: %v", err)
	}
	data, _ := io.ReadAll(body)
	if len(data) == 0 {
		t.Error("GetBody returned empty body")
	}

	// Verify the replayed body matches the original.
	var reqBody SearchRequest
	if err := json.Unmarshal(data, &reqBody); err != nil {
		t.Fatalf("unmarshal replayed body: %v", err)
	}
	if reqBody.Q != "test" {
		t.Errorf("replayed Q: got %q, want %q", reqBody.Q, "test")
	}
}

func TestSearch_ValidationError(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t,"key", WithDoer(mock))

	_, err := c.Search(context.Background(), &SearchRequest{Q: ""})
	if err == nil {
		t.Fatal("expected validation error for empty query")
	}
	if !strings.Contains(err.Error(), "query") {
		t.Errorf("error should mention 'query', got: %v", err)
	}
	if mock.req != nil {
		t.Error("no HTTP request should have been made for invalid input")
	}
}

func TestSearch_AppliesDefaults(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t,"key", WithDoer(mock))

	// Pass only Q; Num/GL/HL/Page should get defaults from prepareRequest.
	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sent SearchRequest
	if err := json.Unmarshal(mock.body, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if sent.Num != 10 {
		t.Errorf("Num: got %d, want 10", sent.Num)
	}
	if sent.GL != "us" {
		t.Errorf("GL: got %q, want %q", sent.GL, "us")
	}
	if sent.Page != 1 {
		t.Errorf("Page: got %d, want 1", sent.Page)
	}
}

func TestSearchRequest_SetDefaults(t *testing.T) {
	r := &SearchRequest{Q: "test"}
	r.SetDefaults()

	if r.Num != 10 {
		t.Errorf("Num: got %d, want 10", r.Num)
	}
	if r.GL != "us" {
		t.Errorf("GL: got %q, want %q", r.GL, "us")
	}
	if r.HL != "en" {
		t.Errorf("HL: got %q, want %q", r.HL, "en")
	}
	if r.Page != 1 {
		t.Errorf("Page: got %d, want 1", r.Page)
	}
}

func TestSearchRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     SearchRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid request",
			req:     SearchRequest{Q: "golang", Num: 10, Page: 1},
			wantErr: false,
		},
		{
			name:    "empty query",
			req:     SearchRequest{Q: "", Num: 10, Page: 1},
			wantErr: true,
			errMsg:  "query",
		},
		{
			name:    "num zero",
			req:     SearchRequest{Q: "test", Num: 0, Page: 1},
			wantErr: true,
			errMsg:  "num",
		},
		{
			name:    "num too high",
			req:     SearchRequest{Q: "test", Num: 200, Page: 1},
			wantErr: true,
			errMsg:  "num",
		},
		{
			name:    "page zero",
			req:     SearchRequest{Q: "test", Num: 10, Page: 0},
			wantErr: true,
			errMsg:  "page",
		},
		{
			name:    "negative page",
			req:     SearchRequest{Q: "test", Num: 10, Page: -1},
			wantErr: true,
			errMsg:  "page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
				t.Errorf("error should mention %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestPlaces_Success(t *testing.T) {
	respJSON := `{
		"searchParameters": {"q": "pizza", "type": "places"},
		"places": [
			{"title": "Pizza Place", "address": "123 Main St", "latitude": 40.7, "longitude": -74.0, "rating": 4.5, "ratingCount": 100, "category": "Restaurant", "position": 1}
		]
	}`
	mock := &mockDoer{statusCode: 200, respBody: respJSON}
	c := mustNew(t, "key", WithDoer(mock), WithBaseURL("https://api.test"))

	resp, err := c.Places(context.Background(), &SearchRequest{Q: "pizza"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Places) != 1 {
		t.Fatalf("places: got %d, want 1", len(resp.Places))
	}
	wantURL := "https://api.test/places"
	if mock.req.URL.String() != wantURL {
		t.Errorf("URL: got %q, want %q", mock.req.URL.String(), wantURL)
	}
}

func TestScholar_Success(t *testing.T) {
	respJSON := `{
		"searchParameters": {"q": "machine learning", "type": "scholar"},
		"organic": [
			{"title": "ML Paper", "link": "https://example.com/paper", "snippet": "A paper.", "publicationInfo": "Journal 2024", "citedBy": 50, "position": 1}
		]
	}`
	mock := &mockDoer{statusCode: 200, respBody: respJSON}
	c := mustNew(t, "key", WithDoer(mock), WithBaseURL("https://api.test"))

	resp, err := c.Scholar(context.Background(), &SearchRequest{Q: "machine learning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Organic) != 1 {
		t.Fatalf("scholar results: got %d, want 1", len(resp.Organic))
	}
	wantURL := "https://api.test/scholar"
	if mock.req.URL.String() != wantURL {
		t.Errorf("URL: got %q, want %q", mock.req.URL.String(), wantURL)
	}
}

func TestSearch_DoesNotMutateRequest(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t, "key", WithDoer(mock))

	req := &SearchRequest{Q: "test"}
	_, _ = c.Search(context.Background(), req)

	if req.Num != 0 {
		t.Errorf("caller's Num was mutated: got %d, want 0", req.Num)
	}
	if req.GL != "" {
		t.Errorf("caller's GL was mutated: got %q, want empty", req.GL)
	}
	if req.Page != 0 {
		t.Errorf("caller's Page was mutated: got %d, want 0", req.Page)
	}
}

// contextAwareDoer checks the request context before responding.
type contextAwareDoer struct {
	statusCode int
	respBody   string
}

func (d *contextAwareDoer) Do(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: d.statusCode,
		Body:       io.NopCloser(strings.NewReader(d.respBody)),
	}, nil
}

func TestSearch_ContextCancellation(t *testing.T) {
	doer := &contextAwareDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t, "key", WithDoer(doer))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.Search(ctx, &SearchRequest{Q: "test"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("error should mention context canceled, got: %v", err)
	}
}

func TestSearch_MalformedJSONResponse(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{not json`}
	c := mustNew(t, "key", WithDoer(mock))

	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
	// secval.ValidateJSON catches invalid JSON before json.Unmarshal.
	if !strings.Contains(err.Error(), "unsafe response body") {
		t.Errorf("error should mention 'unsafe response body', got: %v", err)
	}
}

func TestSearch_ErrorBodyTruncation(t *testing.T) {
	// Build an error body larger than maxErrorBodyBytes (1024).
	longBody := strings.Repeat("x", 2000)
	mock := &mockDoer{statusCode: 500, respBody: longBody}
	c := mustNew(t, "key", WithDoer(mock))

	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "...(truncated)") {
		t.Errorf("error should be truncated, got length %d: %s", len(errMsg), errMsg)
	}
	// The error body portion should be at most maxErrorBodyBytes + the truncation suffix.
	if len(errMsg) > 1200 {
		t.Errorf("error message too long (%d chars), truncation may not be working", len(errMsg))
	}
}

func TestSearch_HTTP500(t *testing.T) {
	mock := &mockDoer{statusCode: 500, respBody: `{"error":"internal server error"}`}
	c := mustNew(t, "key", WithDoer(mock))

	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain '500', got: %v", err)
	}
}

func TestSearch_HTTP503(t *testing.T) {
	mock := &mockDoer{statusCode: 503, respBody: `service unavailable`}
	c := mustNew(t, "key", WithDoer(mock))

	_, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
	if err == nil {
		t.Fatal("expected error for 503 status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should contain '503', got: %v", err)
	}
}

func TestSetDefaults_PreservesExplicitValues(t *testing.T) {
	r := &SearchRequest{Q: "test", Num: 50, GL: "de", HL: "fr", Page: 3}
	r.SetDefaults()

	if r.Num != 50 {
		t.Errorf("Num: got %d, want 50", r.Num)
	}
	if r.GL != "de" {
		t.Errorf("GL: got %q, want %q", r.GL, "de")
	}
	if r.HL != "fr" {
		t.Errorf("HL: got %q, want %q", r.HL, "fr")
	}
	if r.Page != 3 {
		t.Errorf("Page: got %d, want 3", r.Page)
	}
}

func TestValidate_BoundaryValues(t *testing.T) {
	tests := []struct {
		name    string
		req     SearchRequest
		wantErr bool
	}{
		{
			name:    "num=1 (min valid)",
			req:     SearchRequest{Q: "test", Num: 1, Page: 1},
			wantErr: false,
		},
		{
			name:    "num=100 (max valid)",
			req:     SearchRequest{Q: "test", Num: 100, Page: 1},
			wantErr: false,
		},
		{
			name:    "num=101 (just over max)",
			req:     SearchRequest{Q: "test", Num: 101, Page: 1},
			wantErr: true,
		},
		{
			name:    "page=1 (min valid)",
			req:     SearchRequest{Q: "test", Num: 10, Page: 1},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCheckConnectivity_Success(t *testing.T) {
	respJSON := `{
		"searchParameters": {"q": "test", "num": 1},
		"organic": []
	}`
	mock := &mockDoer{statusCode: 200, respBody: respJSON}
	c := mustNew(t, "key", WithDoer(mock))

	err := c.CheckConnectivity(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.req == nil {
		t.Fatal("expected request to be made")
	}
}

func TestCheckConnectivity_Failure(t *testing.T) {
	mock := &mockDoer{statusCode: 401, respBody: `{"error":"unauthorized"}`}
	c := mustNew(t, "bad-key", WithDoer(mock))

	err := c.CheckConnectivity(context.Background())
	if err == nil {
		t.Fatal("expected error for bad API key")
	}
}

func TestSearch_EmptyResponseBody(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{}`}
	c := mustNew(t, "key", WithDoer(mock))

	resp, err := c.Search(context.Background(), &SearchRequest{Q: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Organic != nil {
		t.Errorf("expected nil Organic for empty response, got %v", resp.Organic)
	}
}

func TestDoRequest_UsesPostMethod(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t, "key", WithDoer(mock))

	_, _ = c.Search(context.Background(), &SearchRequest{Q: "test"})

	if mock.req == nil {
		t.Fatal("expected request to be captured")
	}
	if mock.req.Method != http.MethodPost {
		t.Errorf("method: got %q, want %q", mock.req.Method, http.MethodPost)
	}
}

func TestSearch_OmitsEmptyLocationFromBody(t *testing.T) {
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t, "key", WithDoer(mock))

	_, _ = c.Search(context.Background(), &SearchRequest{Q: "test"})

	var sent map[string]any
	if err := json.Unmarshal(mock.body, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if _, exists := sent["location"]; exists {
		t.Error("empty location should be omitted from JSON body")
	}
}

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
	mock := &mockDoer{statusCode: 200, respBody: `{"organic":[]}`}
	c := mustNew(t, "default-key", WithDoer(mock))

	ctx := WithAPIKey(context.Background(), "outer-key")
	ctx = WithAPIKey(ctx, "inner-key")
	_, _ = c.Search(ctx, &SearchRequest{Q: "test"})

	if mock.req == nil {
		t.Fatal("expected request to be captured")
	}
	if got := mock.req.Header.Get("X-API-KEY"); got != "inner-key" {
		t.Errorf("X-API-KEY: got %q, want %q", got, "inner-key")
	}
}

func TestNew_LastOptionWins(t *testing.T) {
	mock := &mockDoer{}
	c := mustNew(t, "key",
		WithBaseURL("https://first.com"),
		WithBaseURL("https://second.com"),
		WithDoer(mock),
	)
	if c.baseURL != "https://second.com" {
		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://second.com")
	}
}
