package goclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Sentinel errors for HTTP response classification.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrInvalidRequest = errors.New("invalid request")
	ErrServerError    = errors.New("server error")
)

// HTTPClient wraps HTTP calls to the GoClaw REST API with auth and error handling.
type HTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewHTTPClient creates a new authenticated HTTP client for GoClaw.
func NewHTTPClient(baseURL, token string) *HTTPClient {
	return &HTTPClient{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

// Get performs an authenticated GET request.
func (c *HTTPClient) Get(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// Post performs an authenticated POST request with a JSON body.
func (c *HTTPClient) Post(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doJSON(ctx, http.MethodPost, path, body)
}

// Put performs an authenticated PUT request with a JSON body.
func (c *HTTPClient) Put(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doJSON(ctx, http.MethodPut, path, body)
}

// Patch performs an authenticated PATCH request with a JSON body.
func (c *HTTPClient) Patch(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doJSON(ctx, http.MethodPatch, path, body)
}

// Delete performs an authenticated DELETE request.
func (c *HTTPClient) Delete(ctx context.Context, path string) error {
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *HTTPClient) doJSON(ctx context.Context, method, path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}
	return c.do(ctx, method, path, bytes.NewReader(data))
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if err := classifyStatus(resp.StatusCode, respBody); err != nil {
		return nil, err
	}

	return respBody, nil
}

func classifyStatus(code int, body []byte) error {
	if code >= 200 && code < 300 {
		return nil
	}

	msg := string(body)
	switch {
	case code == 401 || code == 403:
		return fmt.Errorf("%w: %s", ErrUnauthorized, msg)
	case code == 404:
		return fmt.Errorf("%w: %s", ErrNotFound, msg)
	case code == 400 || code == 422:
		return fmt.Errorf("%w: %s", ErrInvalidRequest, msg)
	case code >= 500:
		return fmt.Errorf("%w: %s", ErrServerError, msg)
	default:
		return fmt.Errorf("unexpected status %d: %s", code, msg)
	}
}
