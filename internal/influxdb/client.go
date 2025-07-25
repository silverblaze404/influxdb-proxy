package influxdb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

// Client wraps HTTP client for InfluxDB communication
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient creates a new InfluxDB client
func NewClient(baseURL, username, password string, timeout time.Duration) *Client {
	// Configure HTTP client for high concurrency and performance
	transport := &http.Transport{
		MaxIdleConns:          200,              // Total idle connections across all hosts
		MaxIdleConnsPerHost:   50,               // Idle connections per host (increased)
		MaxConnsPerHost:       100,              // Max connections per host (doubled)
		IdleConnTimeout:       90 * time.Second, // Keep connections alive
		DisableCompression:    true,             // Disable compression for better performance
		TLSHandshakeTimeout:   10 * time.Second, // TLS handshake timeout
		ExpectContinueTimeout: 1 * time.Second,  // Expect: 100-continue timeout
		ResponseHeaderTimeout: 30 * time.Second, // Header read timeout
		DisableKeepAlives:     false,            // Enable keep-alive
		ForceAttemptHTTP2:     false,            // Stick to HTTP/1.1 for better connection reuse
	}

	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// ForwardRequest forwards any HTTP request to InfluxDB
func (c *Client) ForwardRequest(r *http.Request) (*http.Response, error) {
	// Prepare the target URL
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Preserve the original path and query parameters
	u.Path = r.URL.Path
	u.RawQuery = r.URL.RawQuery

	// Add authentication if configured and not already present
	if c.username != "" && c.password != "" {
		q := u.Query()
		// Only add auth if not already present in the original request
		if q.Get("u") == "" && q.Get("p") == "" {
			q.Set("u", c.username)
			q.Set("p", c.password)
			u.RawQuery = q.Encode()
		}
	}

	// Create new request with the same method and body
	var body io.Reader
	if r.Body != nil {
		// The body might have already been processed by the server (e.g., for query extraction)
		// In that case, just use the body as-is without re-reading it
		body = r.Body
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Copy headers from original request (except Host)
	for k, v := range r.Header {
		if k != "Host" {
			req.Header[k] = v
		}
	}

	log.WithFields(log.Fields{
		"method": r.Method,
		"url":    u.String(),
		"path":   r.URL.Path,
	}).Debug("Forwarding request to InfluxDB")

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to forward request: %w", err)
	}

	return resp, nil
}

// Health checks if InfluxDB is healthy
func (c *Client) Health() error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	u.Path = "/health"

	log.WithField("url", u.String()).Debug("Checking InfluxDB health")

	req, err := http.NewRequestWithContext(context.Background(), "GET", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to ping InfluxDB: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("InfluxDB health check failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Close closes the HTTP client and cleans up resources
func (c *Client) Close() {
	// Close idle connections to prevent resource leaks
	c.httpClient.CloseIdleConnections()
}
