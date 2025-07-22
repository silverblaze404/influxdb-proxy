package influxdb

import (
	"bytes"
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
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Query executes a query against InfluxDB
func (c *Client) Query(query, database string) (*http.Response, error) {
	// Prepare the request URL
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	u.Path = "/query"

	// Set query parameters
	params := url.Values{}
	params.Set("q", query)
	if database != "" {
		params.Set("db", database)
	}
	if c.username != "" {
		params.Set("u", c.username)
	}
	if c.password != "" {
		params.Set("p", c.password)
	}

	// Create the request
	req, err := http.NewRequest("POST", u.String(), bytes.NewBufferString(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	log.WithFields(log.Fields{
		"url":      u.String(),
		"database": database,
		"query":    query,
	}).Debug("Executing query against InfluxDB")

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
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

	req, err := http.NewRequest("GET", u.String(), nil)
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
