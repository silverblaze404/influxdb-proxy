package config

import (
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v2"
)

// Config represents the application configuration
type Config struct {
	InfluxDB InfluxDBConfig `yaml:"influxdb"`
	Proxy    ProxyConfig    `yaml:"proxy"`
	Logging  LoggingConfig  `yaml:"logging"`
	Metrics  MetricsConfig  `yaml:"metrics"`
}

// InfluxDBConfig contains InfluxDB connection settings
type InfluxDBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// URL returns the complete InfluxDB URL
func (c *InfluxDBConfig) URL() string {
	return fmt.Sprintf("http://%s:%d", c.Host, c.Port)
}

// ProxyConfig contains proxy server settings
type ProxyConfig struct {
	Port                  int                  `yaml:"port"`
	Host                  string               `yaml:"host"`
	WhitelistedIPs        []string             `yaml:"whitelisted_ips"`         // IPs that bypass filtering
	DisableQueryFiltering bool                 `yaml:"disable_query_filtering"` // Disable query filtering (default: false - filtering enabled)
	FilteringRules        FilteringRules       `yaml:"filtering_rules"`
	InfluxDBClient        InfluxDBClientConfig `yaml:"influxdb_client"`
	ServerTimeouts        ServerTimeouts       `yaml:"server_timeouts"`
}

// ServerTimeouts contains HTTP server timeout settings
type ServerTimeouts struct {
	ReadTimeoutSeconds  int `yaml:"read_timeout_seconds"`  // Default: 2x InfluxDB client read timeout
	WriteTimeoutSeconds int `yaml:"write_timeout_seconds"` // Default: 2x InfluxDB client write timeout
	IdleTimeoutSeconds  int `yaml:"idle_timeout_seconds"`  // Default: 2x InfluxDB client idle timeout
}

// InfluxDBClientConfig contains InfluxDB HTTP client performance-related settings
type InfluxDBClientConfig struct {
	TimeoutSeconds           int `yaml:"timeout_seconds"`            // Max timeout (default: 30 seconds)
	IdleConnectionPoolSize   int `yaml:"idle_connection_pool_size"`  // How many idle connections to keep (default: 200)
	MaxConcurrentConnections int `yaml:"max_concurrent_connections"` // Max active connections (default: 400)
	ReadTimeoutSeconds       int `yaml:"read_timeout_seconds"`       // Default: 30
	IdleTimeoutSeconds       int `yaml:"idle_timeout_seconds"`       // Default: 120
}

// FilteringRules contains configurable query filtering options
type FilteringRules struct {
	RequireTimeFilter     bool     `yaml:"require_time_filter"`
	MaxTimeRangeHours     int      `yaml:"max_time_range_hours"`
	BlockWildcardSelect   bool     `yaml:"block_wildcard_select"`
	BlockUnlimitedGroupBy bool     `yaml:"block_unlimited_group_by"`
	BlockExpensiveShows   bool     `yaml:"block_expensive_shows"`
	MaxShowSeriesLimit    int      `yaml:"max_show_series_limit"`
	BlockedFunctions      []string `yaml:"blocked_functions"`
	BlockedStatements     []string `yaml:"blocked_statements"`
	AllowedMeasurements   []string `yaml:"allowed_measurements"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// Load reads and parses the configuration file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func setDefaults(config *Config) {
	if config.Proxy.Host == "" {
		config.Proxy.Host = "0.0.0.0"
	}
	if config.Proxy.Port == 0 {
		config.Proxy.Port = 8087
	}
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}
	if config.Metrics.Path == "" {
		config.Metrics.Path = "/proxy_metrics"
	}

	// Set InfluxDB client defaults
	client := &config.Proxy.InfluxDBClient
	if client.TimeoutSeconds == 0 {
		client.TimeoutSeconds = 120
	}
	if client.IdleConnectionPoolSize == 0 {
		client.IdleConnectionPoolSize = 200
	}
	if client.MaxConcurrentConnections == 0 {
		client.MaxConcurrentConnections = 400
	}
	if client.ReadTimeoutSeconds == 0 {
		client.ReadTimeoutSeconds = 60
	}
	if client.IdleTimeoutSeconds == 0 {
		client.IdleTimeoutSeconds = 120
	}

	// Set server timeout defaults (2x client timeouts for safety margin)
	serverTimeouts := &config.Proxy.ServerTimeouts
	if serverTimeouts.ReadTimeoutSeconds == 0 {
		serverTimeouts.ReadTimeoutSeconds = client.ReadTimeoutSeconds * 2
	}
	if serverTimeouts.WriteTimeoutSeconds == 0 {
		serverTimeouts.WriteTimeoutSeconds = 120
	}
	if serverTimeouts.IdleTimeoutSeconds == 0 {
		serverTimeouts.IdleTimeoutSeconds = client.IdleTimeoutSeconds * 2
	}

	// Set filtering rule defaults
	rules := &config.Proxy.FilteringRules
	if rules.MaxTimeRangeHours == 0 {
		rules.MaxTimeRangeHours = 720 // 30 days
	}
	if rules.MaxShowSeriesLimit == 0 {
		rules.MaxShowSeriesLimit = 10000
	}
}

func validate(config *Config) error {
	if config.InfluxDB.Host == "" {
		return fmt.Errorf("influxdb.host is required")
	}
	if config.InfluxDB.Port < 1 || config.InfluxDB.Port > 65535 {
		return fmt.Errorf("influxdb.port must be between 1 and 65535")
	}
	if config.Proxy.Port < 1 || config.Proxy.Port > 65535 {
		return fmt.Errorf("proxy.port must be between 1 and 65535")
	}
	if config.Proxy.FilteringRules.MaxTimeRangeHours < 0 {
		return fmt.Errorf("filtering_rules.max_time_range_hours must be positive")
	}
	if config.Proxy.FilteringRules.MaxShowSeriesLimit < 0 {
		return fmt.Errorf("filtering_rules.max_show_series_limit must be positive")
	}
	if config.Proxy.InfluxDBClient.TimeoutSeconds < 0 {
		return fmt.Errorf("influxdb_client.timeout_seconds must be positive")
	}
	if config.Proxy.InfluxDBClient.IdleConnectionPoolSize < 0 {
		return fmt.Errorf("influxdb_client.idle_connection_pool_size must be positive")
	}
	if config.Proxy.InfluxDBClient.MaxConcurrentConnections < 0 {
		return fmt.Errorf("influxdb_client.max_concurrent_connections must be positive")
	}
	if config.Proxy.InfluxDBClient.ReadTimeoutSeconds < 0 {
		return fmt.Errorf("influxdb_client.read_timeout_seconds must be positive")
	}
	if config.Proxy.InfluxDBClient.IdleTimeoutSeconds < 0 {
		return fmt.Errorf("influxdb_client.idle_timeout_seconds must be positive")
	}
	if config.Proxy.ServerTimeouts.ReadTimeoutSeconds < 0 {
		return fmt.Errorf("server_timeouts.read_timeout_seconds must be positive")
	}
	if config.Proxy.ServerTimeouts.WriteTimeoutSeconds < 0 {
		return fmt.Errorf("server_timeouts.write_timeout_seconds must be positive")
	}
	if config.Proxy.ServerTimeouts.IdleTimeoutSeconds < 0 {
		return fmt.Errorf("server_timeouts.idle_timeout_seconds must be positive")
	}
	return nil
}

// IsIPWhitelisted checks if the given IP address is in the whitelist
// Supports both individual IP addresses and CIDR ranges
func (p *ProxyConfig) IsIPWhitelisted(clientIP string) bool {
	if len(p.WhitelistedIPs) == 0 {
		return false
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, whitelistedEntry := range p.WhitelistedIPs {
		// Check if it's a CIDR range
		if _, ipNet, err := net.ParseCIDR(whitelistedEntry); err == nil {
			if ipNet.Contains(ip) {
				return true
			}
		} else {
			// Check if it's an individual IP
			if whitelistedIP := net.ParseIP(whitelistedEntry); whitelistedIP != nil {
				if ip.Equal(whitelistedIP) {
					return true
				}
			}
		}
	}

	return false
}

// IsQueryFilteringDisabled returns true if query filtering is disabled
func (p *ProxyConfig) IsQueryFilteringDisabled() bool {
	return p.DisableQueryFiltering
}
