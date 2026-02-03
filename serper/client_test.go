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

func TestNew_Defaults(t *testing.T) {
	c := New("test-key")
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

func TestNew_WithOptions(t *testing.T) {
	mock := &mockDoer{}
	c := New("key",
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
	c := New("my-api-key", WithDoer(mock), WithBaseURL("https://api.test"))

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
	c := New("bad-key", WithDoer(mock))

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
	c := New("key", WithDoer(mock))

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
	c := New("default-key", WithDoer(mock))

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
	c := New("default-key", WithDoer(mock))

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
	c := New("key", WithDoer(mock), WithBaseURL("https://api.test"))

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
	c := New("key", WithDoer(mock), WithBaseURL("https://api.test"))

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
	c := New("key", WithDoer(mock))

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
	c := New("key", WithDoer(mock))

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
	c := New("key", WithDoer(mock))

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
