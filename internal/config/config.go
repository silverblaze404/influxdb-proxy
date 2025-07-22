package config

import (
	"fmt"
	"io/ioutil"

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
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// ProxyConfig contains proxy server settings
type ProxyConfig struct {
	Port            int               `yaml:"port"`
	Host            string            `yaml:"host"`
	MaxQueryTimeout int               `yaml:"max_query_timeout"` // seconds
	FilteringRules  FilteringRules    `yaml:"filtering_rules"`
	Performance     PerformanceConfig `yaml:"performance"`
}

// PerformanceConfig contains performance-related settings
type PerformanceConfig struct {
	MaxIdleConns        int `yaml:"max_idle_conns"`          // Default: 200
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host"` // Default: 50
	MaxConnsPerHost     int `yaml:"max_conns_per_host"`      // Default: 100
	ReadTimeoutSeconds  int `yaml:"read_timeout_seconds"`    // Default: 30
	WriteTimeoutSeconds int `yaml:"write_timeout_seconds"`   // Default: 120
	IdleTimeoutSeconds  int `yaml:"idle_timeout_seconds"`    // Default: 120
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
	AllowedStatements     []string `yaml:"allowed_statements"`
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
	data, err := ioutil.ReadFile(filename)
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
	if config.Proxy.MaxQueryTimeout == 0 {
		config.Proxy.MaxQueryTimeout = 30 // 30 seconds
	}
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "text"
	}
	if config.Metrics.Path == "" {
		config.Metrics.Path = "/metrics"
	}

	// Set filtering rule defaults
	rules := &config.Proxy.FilteringRules
	if rules.MaxTimeRangeHours == 0 {
		rules.MaxTimeRangeHours = 168 // 7 days
	}
	if rules.MaxShowSeriesLimit == 0 {
		rules.MaxShowSeriesLimit = 10000
	}
	// Default allowed statements if none specified
	if len(rules.AllowedStatements) == 0 {
		rules.AllowedStatements = []string{
			"SELECT", "SHOW DATABASES", "SHOW MEASUREMENTS",
			"SHOW TAG KEYS", "SHOW TAG VALUES", "SHOW FIELD KEYS",
			"SHOW SERIES",
		}
	}
}

func validate(config *Config) error {
	if config.InfluxDB.URL == "" {
		return fmt.Errorf("influxdb.url is required")
	}
	if config.Proxy.Port < 1 || config.Proxy.Port > 65535 {
		return fmt.Errorf("proxy.port must be between 1 and 65535")
	}
	if config.Proxy.FilteringRules.MaxTimeRangeHours < 1 {
		return fmt.Errorf("filtering_rules.max_time_range_hours must be positive")
	}
	return nil
}
