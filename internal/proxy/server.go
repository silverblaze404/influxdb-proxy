package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"influxdb-proxy/internal/config"
	"influxdb-proxy/internal/filter"
	"influxdb-proxy/internal/influxdb"
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

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// Creates a new proxy server
func NewServer(cfg *config.Config) (*Server, error) {
	// Create InfluxDB client with performance configuration
	influxClient := influxdb.NewClient(
		cfg.InfluxDB.URL(),
		cfg.InfluxDB.Username,
		cfg.InfluxDB.Password,
		time.Duration(cfg.Proxy.InfluxDBClient.TimeoutSeconds)*time.Second,
		cfg.Proxy.InfluxDBClient,
	)

	// Test connection to InfluxDB
	if err := influxClient.Health(); err != nil {
		log.Warnf("InfluxDB health check failed: %v", err)
	} else {
		log.Info("Successfully connected to InfluxDB")
	}

	queryFilter := filter.NewQueryFilter(cfg.Proxy.FilteringRules)

	// Log blacklisted IPs if any are configured
	if len(cfg.Proxy.BlacklistedIPs) > 0 {
		log.WithFields(log.Fields{
			"blacklisted_ips": cfg.Proxy.BlacklistedIPs,
		}).Info("Blacklisted IPs configured")
	}

	// Log whitelisted IPs if any are configured
	if len(cfg.Proxy.WhitelistedIPs) > 0 {
		log.WithFields(log.Fields{
			"whitelisted_ips": cfg.Proxy.WhitelistedIPs,
		}).Info("Whitelisted IPs configured")
	}

	return &Server{
		config:       cfg,
		influxClient: influxClient,
		queryFilter:  queryFilter,
		metrics:      &Metrics{},
	}, nil
}

// Handler returns the HTTP handler for the proxy server
func (s *Server) Handler() http.Handler {
	// Create the main router
	mainRouter := mux.NewRouter()

	// Create application router with all our routes
	appRouter := mux.NewRouter()

	appRouter.HandleFunc("/query", s.handleQuery).Methods("POST", "GET")
	appRouter.HandleFunc("/proxy_health", s.handleHealth).Methods("GET")
	if s.config.Metrics.Enabled {
		appRouter.HandleFunc("/proxy_metrics", s.handleMetrics).Methods("GET")
	}
	appRouter.PathPrefix("/").HandlerFunc(s.handleForward)

	// Mount the application router with or without base path
	if s.config.Proxy.BasePath == "/" {
		mainRouter.PathPrefix("/").Handler(appRouter)
	} else {
		mainRouter.PathPrefix(s.config.Proxy.BasePath).Handler(http.StripPrefix(s.config.Proxy.BasePath, appRouter))
	}

	mainRouter.Use(s.loggingMiddleware)

	return mainRouter
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.metrics.TotalQueries, 1)

	clientIP := s.getClientIP(r)

	// Extract query from request for filtering
	query, _, err := s.extractQuery(r)
	if err != nil {
		atomic.AddInt64(&s.metrics.Errors, 1)
		log.WithError(err).WithFields(log.Fields{
			"client_ip": clientIP,
			"method":    r.Method,
			"path":      r.URL.Path,
		}).Error("Failed to extract query from request")
		s.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	if query == "" {
		atomic.AddInt64(&s.metrics.Errors, 1)
		log.WithFields(log.Fields{
			"client_ip": clientIP,
			"method":    r.Method,
			"path":      r.URL.Path,
		}).Warn("Request missing required query parameter 'q'")
		s.sendError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	log.WithFields(log.Fields{
		"query":     query,
		"client_ip": clientIP,
	}).Info("Processing query")

	var isBlacklisted = s.config.Proxy.IsIPBlacklisted(clientIP)
	if isBlacklisted {
		log.Info("Query from blacklisted IP - will proceed applying filters even if filtering is disabled")
	}

	// Check if query filtering is disabled
	if s.config.Proxy.IsQueryFilteringDisabled() && !isBlacklisted {
		log.Info("Query filtering disabled - forwarding directly")
		atomic.AddInt64(&s.metrics.AllowedQueries, 1)
		s.forwardToInfluxDB(w, r, clientIP)
		return
	}

	// Check if client IP is whitelisted - if so, bypass filtering
	if s.config.Proxy.IsIPWhitelisted(clientIP) {
		log.Info("Query from whitelisted IP - bypassing filters")
		atomic.AddInt64(&s.metrics.AllowedQueries, 1)
		s.forwardToInfluxDB(w, r, clientIP)
		return
	}

	// Validate query using filter for non-whitelisted IPs
	result := s.queryFilter.ValidateQuery(query)
	if !result.Allowed {
		atomic.AddInt64(&s.metrics.BlockedQueries, 1)
		log.WithFields(log.Fields{
			"client_ip": clientIP,
			"reason":    result.Reason,
			"query":     query,
		}).Info("Query blocked by filtering rules")
		s.sendError(w, http.StatusForbidden, fmt.Sprintf("Query blocked: %s", result.Reason))
		return
	}

	atomic.AddInt64(&s.metrics.AllowedQueries, 1)
	s.forwardToInfluxDB(w, r, clientIP)
}

func (s *Server) handleForward(w http.ResponseWriter, r *http.Request) {
	clientIP := s.getClientIP(r)
	s.forwardToInfluxDB(w, r, clientIP)
}

// Forwards the request to InfluxDB and handles the response
func (s *Server) forwardToInfluxDB(w http.ResponseWriter, r *http.Request, clientIP string) {
	resp, err := s.influxClient.ForwardRequest(r)
	if err != nil {
		atomic.AddInt64(&s.metrics.Errors, 1)
		log.WithError(err).WithFields(log.Fields{
			"client_ip": clientIP,
			"path":      r.URL.Path,
		}).Error("Failed to forward request to InfluxDB")

		// Return a generic 502 Bad Gateway without custom JSON formatting
		// to maintain transparency when InfluxDB is unreachable
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Bad Gateway: InfluxDB unreachable"))
		return
	}
	defer resp.Body.Close()

	// Copy all response data exactly as they are
	maps.Copy(w.Header(), resp.Header)

	w.WriteHeader(resp.StatusCode)

	// Copy response body exactly as returned by InfluxDB
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"client_ip": clientIP,
			"path":      r.URL.Path,
		}).Error("Failed to copy response body")
		// Ensure response body is drained to allow connection reuse
		io.Copy(io.Discard, resp.Body)
	}

	log.WithFields(log.Fields{
		"client_ip": clientIP,
		"method":    r.Method,
		"path":      r.URL.Path,
		"status":    resp.StatusCode,
	}).Debug("Request forwarded successfully")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	}

	if err := s.influxClient.Health(); err != nil {
		health["status"] = "unhealthy"
		health["influxdb_error"] = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]any{
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
		return r.URL.Query().Get("q"), r.URL.Query().Get("db"), nil
	}

	// For POST requests, we need to preserve the body for forwarding
	var bodyBytes []byte
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			return "", "", fmt.Errorf("failed to read request body: %w", err)
		}
		// After reading, the body stream is consumed (at EOF)
		// Replace it with a fresh reader so it can be forwarded later
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	contentType := r.Header.Get("Content-Type")

	if contentType == "application/x-www-form-urlencoded" {
		values, err := url.ParseQuery(string(bodyBytes))
		if err != nil {
			return "", "", fmt.Errorf("failed to parse form-encoded request body: %w", err)
		}
		return values.Get("q"), values.Get("db"), nil
	}

	if contentType == "application/json" {
		var req struct {
			Query    string `json:"q"`
			Database string `json:"db"`
		}
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&req); err != nil {
			return "", "", fmt.Errorf("failed to parse JSON request body: %w", err)
		}
		return req.Query, req.Database, nil
	}

	// Default to form parsing for backward compatibility
	values, err := url.ParseQuery(string(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("failed to parse request body as form data: %w", err)
	}
	return values.Get("q"), values.Get("db"), nil
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

		clientIP := s.getClientIP(r)

		wrapper := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		log.WithFields(log.Fields{
			"client_ip":   clientIP,
			"remote_addr": r.RemoteAddr,
			"method":      r.Method,
			"url":         r.URL.Path,
			"status":      wrapper.statusCode,
			"user_agent":  r.UserAgent(),
			"duration":    duration,
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

// Extracts the real client IP from the request
func (s *Server) getClientIP(r *http.Request) string {
	// Try to get the first IP from X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Split by comma and get the first IP (original client)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host
}

// Close performs cleanup of server resources
func (s *Server) Close() error {
	if s.influxClient != nil {
		s.influxClient.Close()
	}
	return nil
}
