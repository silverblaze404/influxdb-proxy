package proxy

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// RequestThrottler limits concurrent requests to prevent overwhelming InfluxDB
type RequestThrottler struct {
	semaphore chan struct{}
	timeout   time.Duration
	metrics   *Metrics // Reference to proxy metrics
}

// NewRequestThrottler creates a new throttler with specified concurrency limit
func NewRequestThrottler(maxConcurrent int, timeout time.Duration) *RequestThrottler {
	return &RequestThrottler{
		semaphore: make(chan struct{}, maxConcurrent),
		timeout:   timeout,
	}
}

// SetMetrics sets the metrics reference for tracking throttling stats
func (t *RequestThrottler) SetMetrics(metrics *Metrics) {
	t.metrics = metrics
}

// Handler wraps an HTTP handler with throttling
func (t *RequestThrottler) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a context with timeout for the throttling
		ctx, cancel := context.WithTimeout(r.Context(), t.timeout)
		defer cancel()

		// Track concurrent connections
		if t.metrics != nil {
			atomic.AddInt64(&t.metrics.ConcurrentConnections, 1)
			defer atomic.AddInt64(&t.metrics.ConcurrentConnections, -1)
		}

		// Try to acquire a slot in the semaphore
		select {
		case t.semaphore <- struct{}{}:
			// Got a slot, process the request
			defer func() { <-t.semaphore }()
			next.ServeHTTP(w, r)
		case <-ctx.Done():
			// Timeout waiting for a slot
			if t.metrics != nil {
				atomic.AddInt64(&t.metrics.ThrottledRequests, 1)
			}
			http.Error(w, "Request throttled: too many concurrent requests", http.StatusServiceUnavailable)
			return
		}
	})
}
