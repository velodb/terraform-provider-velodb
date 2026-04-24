package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// FormationClient is the HTTP client for the Formation (VeloDB) API.
type FormationClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewFormationClient creates a new API client.
func NewFormationClient(host, apiKey string, maxRetries int, timeout time.Duration) *FormationClient {
	scheme := "https"
	// Allow http for local development
	if isLocalHost(host) {
		scheme = "http"
	}

	return &FormationClient{
		BaseURL: fmt.Sprintf("%s://%s", scheme, host),
		HTTPClient: &http.Client{
			Timeout: timeout,
			Transport: &formationTransport{
				base:       http.DefaultTransport,
				apiKey:     apiKey,
				maxRetries: maxRetries,
			},
		},
	}
}

func isLocalHost(host string) bool {
	h := host
	if u, err := url.Parse("//" + host); err == nil {
		h = u.Hostname()
	}
	return h == "localhost" || h == "127.0.0.1" || h == "0.0.0.0" || h == "::1"
}

// --- HTTP helpers ---

func (c *FormationClient) get(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	return c.HTTPClient.Do(req)
}

func (c *FormationClient) post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doJSON(ctx, http.MethodPost, path, body)
}

func (c *FormationClient) patch(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doJSON(ctx, http.MethodPatch, path, body)
}

func (c *FormationClient) put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doJSON(ctx, http.MethodPut, path, body)
}

func (c *FormationClient) delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.HTTPClient.Do(req)
}

func (c *FormationClient) doJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, &buf)
	if err != nil {
		return nil, err
	}
	return c.HTTPClient.Do(req)
}

// addPagination adds page and size query parameters.
func addPagination(q url.Values, page, size int) {
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if size > 0 {
		q.Set("size", strconv.Itoa(size))
	}
}
