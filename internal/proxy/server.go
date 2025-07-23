package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"gm-influxdb-proxy/internal/config"
	"gm-influxdb-proxy/internal/filter"
	"gm-influxdb-proxy/internal/influxdb"
)

// Server represents the proxy server
type Server struct {
	config       *config.Config
	influxClient *influxdb.Client
	queryFilter  *filter.QueryFilter
	metrics      *Metrics
}

// Metrics holds basic metrics for the proxy
type Metrics struct {
	TotalQueries   int64 `json:"total_queries"`
	BlockedQueries int64 `json:"blocked_queries"`
	AllowedQueries int64 `json:"allowed_queries"`
	Errors         int64 `json:"errors"`
}

// QueryRequest represents a query request
type QueryRequest struct {
	Query    string `json:"q"`
	Database string `json:"db"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewServer creates a new proxy server
func NewServer(cfg *config.Config) (*Server, error) {
	// Create InfluxDB client
	influxClient := influxdb.NewClient(
		cfg.InfluxDB.URL(),
		cfg.InfluxDB.Username,
		cfg.InfluxDB.Password,
		time.Duration(cfg.Proxy.MaxQueryTimeout)*time.Second,
	)

	// Test connection to InfluxDB
	if err := influxClient.Health(); err != nil {
		log.Warnf("InfluxDB health check failed: %v", err)
	} else {
		log.Info("Successfully connected to InfluxDB")
	}

	// Create query filter
	queryFilter := filter.NewQueryFilter(cfg.Proxy.FilteringRules)

	return &Server{
		config:       cfg,
		influxClient: influxClient,
		queryFilter:  queryFilter,
		metrics:      &Metrics{},
	}, nil
}

// Handler returns the HTTP handler for the proxy server
func (s *Server) Handler() http.Handler {
	r := mux.NewRouter()

	// Query endpoint with filtering (highest priority)
	r.HandleFunc("/query", s.handleQuery).Methods("POST", "GET")

	// Proxy health check endpoint
	r.HandleFunc("/proxy_health", s.handleHealth).Methods("GET")

	// Proxy metrics endpoint (if enabled)
	if s.config.Metrics.Enabled {
		r.HandleFunc(s.config.Metrics.Path, s.handleMetrics).Methods("GET")
	}

	// Catch-all handler for all other InfluxDB endpoints - forward directly
	r.PathPrefix("/").HandlerFunc(s.handleForward)

	// Add logging middleware
	r.Use(s.loggingMiddleware)

	return r
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.metrics.TotalQueries, 1)

	// Extract query from request
	query, database, err := s.extractQuery(r)
	if err != nil {
		atomic.AddInt64(&s.metrics.Errors, 1)
		s.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	if query == "" {
		atomic.AddInt64(&s.metrics.Errors, 1)
		s.sendError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	// Use configured database if not specified in request
	if database == "" && s.config.InfluxDB.Database != "" {
		database = s.config.InfluxDB.Database
	}

	log.WithFields(log.Fields{
		"query":    query,
		"database": database,
	}).Debug("Processing query")

	// Validate query using filter
	result := s.queryFilter.ValidateQuery(query)
	if !result.Allowed {
		atomic.AddInt64(&s.metrics.BlockedQueries, 1)
		log.WithFields(log.Fields{
			"query":  query,
			"reason": result.Reason,
		}).Info("Query blocked")

		s.sendError(w, http.StatusForbidden, fmt.Sprintf("Query blocked: %s", result.Reason))
		return
	}

	atomic.AddInt64(&s.metrics.AllowedQueries, 1)

	// Forward query to InfluxDB
	resp, err := s.influxClient.Query(query, database)
	if err != nil {
		atomic.AddInt64(&s.metrics.Errors, 1)
		log.WithError(err).Error("Failed to execute query against InfluxDB")
		s.sendError(w, http.StatusInternalServerError, "Failed to execute query")
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.WithError(err).Error("Failed to copy response body")
	}

	log.WithFields(log.Fields{
		"query":  query,
		"status": resp.StatusCode,
	}).Debug("Query executed successfully")
}

func (s *Server) handleForward(w http.ResponseWriter, r *http.Request) {
	// Forward all non-query requests directly to InfluxDB
	resp, err := s.influxClient.ForwardRequest(r)
	if err != nil {
		atomic.AddInt64(&s.metrics.Errors, 1)
		log.WithError(err).WithField("path", r.URL.Path).Error("Failed to forward request to InfluxDB")

		// Return a generic 502 Bad Gateway without custom JSON formatting
		// to maintain transparency when InfluxDB is unreachable
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Bad Gateway: InfluxDB unreachable"))
		return
	}
	defer resp.Body.Close()

	// Copy ALL response headers exactly as they are
	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	// Set status code exactly as returned by InfluxDB
	w.WriteHeader(resp.StatusCode)

	// Copy response body exactly as returned by InfluxDB
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.WithError(err).WithField("path", r.URL.Path).Error("Failed to copy response body")
		// Note: We can't change the response at this point since headers are already sent
	}

	log.WithFields(log.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"status": resp.StatusCode,
	}).Debug("Request forwarded successfully")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	}

	// Check InfluxDB health
	if err := s.influxClient.Health(); err != nil {
		health["status"] = "unhealthy"
		health["influxdb_error"] = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"proxy_metrics": map[string]int64{
			"total_queries":   atomic.LoadInt64(&s.metrics.TotalQueries),
			"blocked_queries": atomic.LoadInt64(&s.metrics.BlockedQueries),
			"allowed_queries": atomic.LoadInt64(&s.metrics.AllowedQueries),
			"errors":          atomic.LoadInt64(&s.metrics.Errors),
		},
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (s *Server) extractQuery(r *http.Request) (string, string, error) {
	if r.Method == "GET" {
		// Extract from query parameters
		return r.URL.Query().Get("q"), r.URL.Query().Get("db"), nil
	}

	// For POST requests, check content type
	contentType := r.Header.Get("Content-Type")

	if contentType == "application/x-www-form-urlencoded" {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			return "", "", fmt.Errorf("failed to parse form: %w", err)
		}
		return r.FormValue("q"), r.FormValue("db"), nil
	}

	if contentType == "application/json" {
		// Parse JSON body
		var req QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", fmt.Errorf("failed to parse JSON: %w", err)
		}
		return req.Query, req.Database, nil
	}

	// Default to form parsing for backward compatibility
	if err := r.ParseForm(); err != nil {
		return "", "", fmt.Errorf("failed to parse form: %w", err)
	}
	return r.FormValue("q"), r.FormValue("db"), nil
}

func (s *Server) sendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := ErrorResponse{Error: message}
	json.NewEncoder(w).Encode(errorResp)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapper := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		log.WithFields(log.Fields{
			"method":      r.Method,
			"url":         r.URL.Path,
			"status":      wrapper.statusCode,
			"duration":    duration,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}).Info("HTTP request processed")
	})
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
