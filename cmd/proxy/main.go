package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gm-influxdb-proxy/internal/banner"
	"gm-influxdb-proxy/internal/config"
	"gm-influxdb-proxy/internal/proxy"

	log "github.com/sirupsen/logrus"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to configuration file")
	version    = flag.Bool("version", false, "Show version information")
)

const (
	AppVersion = "1.0.0"
	AppName    = "influxdb-proxy"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Println(banner.GetVersion(AppName, AppVersion))
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging
	setupLogging(cfg.Logging)

	// Display trademark logo
	banner.Display(AppVersion)

	log.WithFields(log.Fields{
		"version": AppVersion,
		"config":  *configFile,
	}).Info("Starting InfluxDB Proxy Server")

	// Create proxy server
	proxyServer, err := proxy.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	// Setup HTTP server with optimized settings for high concurrency
	addr := fmt.Sprintf("%s:%d", cfg.Proxy.Host, cfg.Proxy.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      proxyServer.Handler(),
		ReadTimeout:  30 * time.Second,  // Time to read request
		WriteTimeout: 120 * time.Second, // Time to write response (increased for large responses)
		IdleTimeout:  120 * time.Second, // Keep-alive timeout (increased)
	}

	// Start server in a goroutine
	go func() {
		log.WithField("address", addr).Info("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Cleanup proxy server resources
	if err := proxyServer.Close(); err != nil {
		log.WithError(err).Error("Error during proxy cleanup")
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited")
}

func setupLogging(cfg config.LoggingConfig) {
	// Set log level
	level, err := log.ParseLevel(cfg.Level)
	if err != nil {
		log.Warnf("Invalid log level '%s', using 'info'", cfg.Level)
		level = log.InfoLevel
	}
	log.SetLevel(level)

	// Set log format
	if cfg.Format == "json" {
		log.SetFormatter(&log.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	}
}
