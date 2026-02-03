package serper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL    = "https://google.serper.dev"
	defaultTimeout    = 30 * time.Second
	maxResponseBytes  = 10 * 1024 * 1024 // 10 MB
	maxErrorBodyBytes = 1024             // truncate error bodies in messages
)

// Doer executes HTTP requests. Satisfied by *http.Client, call.Client, and test mocks.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client is a Serper.dev API client.
type Client struct {
	apiKey  string
	baseURL string
	doer    Doer
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithDoer sets the HTTP client used for requests.
func WithDoer(d Doer) Option {
	return func(c *Client) { c.doer = d }
}

// New creates a Serper client with the given API key and options.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		doer:    &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// apiKeyContextKey is the context key for API key overrides.
type apiKeyContextKey struct{}

// WithAPIKey returns a new context that overrides the client's default API key.
// If key is empty, the original context is returned unchanged.
func WithAPIKey(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}
	return context.WithValue(ctx, apiKeyContextKey{}, key)
}

// getAPIKey returns the API key for a request, checking context first.
func (c *Client) getAPIKey(ctx context.Context) string {
	if key, ok := ctx.Value(apiKeyContextKey{}).(string); ok && key != "" {
		return key
	}
	return c.apiKey
}

// prepareRequest applies defaults and validates the search request.
func prepareRequest(req *SearchRequest) error {
	req.SetDefaults()
	return req.Validate()
}

// Search performs a web search via Serper.dev.
func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if err := prepareRequest(req); err != nil {
		return nil, err
	}
	var resp SearchResponse
	if err := c.doRequest(ctx, "/search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Images performs an image search via Serper.dev.
func (c *Client) Images(ctx context.Context, req *SearchRequest) (*ImagesResponse, error) {
	if err := prepareRequest(req); err != nil {
		return nil, err
	}
	var resp ImagesResponse
	if err := c.doRequest(ctx, "/images", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// News performs a news search via Serper.dev.
func (c *Client) News(ctx context.Context, req *SearchRequest) (*NewsResponse, error) {
	if err := prepareRequest(req); err != nil {
		return nil, err
	}
	var resp NewsResponse
	if err := c.doRequest(ctx, "/news", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Places performs a places search via Serper.dev.
func (c *Client) Places(ctx context.Context, req *SearchRequest) (*PlacesResponse, error) {
	if err := prepareRequest(req); err != nil {
		return nil, err
	}
	var resp PlacesResponse
	if err := c.doRequest(ctx, "/places", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Scholar performs a scholar search via Serper.dev.
func (c *Client) Scholar(ctx context.Context, req *SearchRequest) (*ScholarResponse, error) {
	if err := prepareRequest(req); err != nil {
		return nil, err
	}
	var resp ScholarResponse
	if err := c.doRequest(ctx, "/scholar", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CheckConnectivity verifies the API key and connectivity to Serper.dev.
func (c *Client) CheckConnectivity(ctx context.Context) error {
	req := &SearchRequest{Q: "test", Num: 1}
	_, err := c.Search(ctx, req)
	return err
}

// doRequest performs an HTTP request to the Serper.dev API.
func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody, respBody any) error {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("serper: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("serper: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", c.getAPIKey(ctx))

	// Allow retry middleware to replay the body on subsequent attempts.
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(jsonBody)), nil
	}

	resp, err := c.doer.Do(req)
	if err != nil {
		return fmt.Errorf("serper: do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("serper: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		msg := string(body)
		if len(msg) > maxErrorBodyBytes {
			msg = msg[:maxErrorBodyBytes] + "...(truncated)"
		}
		return fmt.Errorf("serper: HTTP %d: %s", resp.StatusCode, msg)
	}

	if err := json.Unmarshal(body, respBody); err != nil {
		return fmt.Errorf("serper: unmarshal response: %w", err)
	}

	return nil
}
