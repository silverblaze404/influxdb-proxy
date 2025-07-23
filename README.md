# InfluxDB Proxy Server

A Go-based proxy server for InfluxDB v1 that filters and rejects expensive queries to protect your database from performance issues.

## Features

- **Query Filtering**: Automatically rejects queries without time filters
- **Time Range Validation**: Prevents queries with excessively long time ranges
- **Performance Protection**: Guards against expensive operations that could impact database performance
- **Configurable Rules**: Customizable filtering rules via configuration file
- **Logging**: Comprehensive logging of filtered queries and proxy activity
- **Health Checks**: Built-in health check endpoints

## Quick Start

Clone the repository:

```bash
git clone https://github.com/greyorange-labs/gm-influxdb-proxy.git
cd gm-influxdb-proxy
```

Install dependencies:

```bash
go mod download
```

Configure the proxy by editing `config.yaml`:

```yaml
influxdb:
  url: "http://localhost:8086"
  username: ""
  password: ""
  database: ""

proxy:
  port: 8087
  host: "0.0.0.0"
  max_query_timeout: 30
  performance:
    max_idle_conns: 200
    max_idle_conns_per_host: 50
    max_conns_per_host: 100
    read_timeout_seconds: 30
    write_timeout_seconds: 120
    idle_timeout_seconds: 120
  filtering_rules:
    require_time_filter: true
    max_time_range_hours: 168  # 7 days
    block_wildcard_select: false
    block_unlimited_group_by: true
    block_expensive_shows: true
    max_show_series_limit: 10000
    blocked_functions:
      - "count(*)"
    blocked_statements:
      - "CREATE"
      - "DROP"
      - "DELETE"
    allowed_statements:
      - "SELECT"
      - "SHOW DATABASES"
      - "SHOW MEASUREMENTS"

logging:
  level: "info"
  format: "json"

metrics:
  enabled: true
  path: "/proxy_metrics"
```

Run the proxy:

```bash
go run cmd/proxy/main.go
```

## Configuration

The proxy uses a YAML configuration file (`config.yaml`) with the following main sections:

### InfluxDB Connection

- `influxdb.url`: URL of the target InfluxDB instance
- `influxdb.username`: InfluxDB username (optional)
- `influxdb.password`: InfluxDB password (optional)
- `influxdb.database`: Default database name (optional)

### Proxy Settings

- `proxy.port`: Port for the proxy server (default: 8087)
- `proxy.host`: Host interface to bind to (default: "0.0.0.0")
- `proxy.max_query_timeout`: Maximum query timeout in seconds

### Performance Tuning

- `proxy.performance.max_idle_conns`: Total idle connections to InfluxDB
- `proxy.performance.max_idle_conns_per_host`: Idle connections per InfluxDB host
- `proxy.performance.max_conns_per_host`: Max concurrent connections per host

### Filtering Rules

- `proxy.filtering_rules.require_time_filter`: Require time filters in queries
- `proxy.filtering_rules.max_time_range_hours`: Maximum allowed time range
- `proxy.filtering_rules.block_wildcard_select`: Block SELECT * without LIMIT
- `proxy.filtering_rules.block_unlimited_group_by`: Block GROUP BY without LIMIT
- `proxy.filtering_rules.blocked_functions`: List of blocked functions
- `proxy.filtering_rules.blocked_statements`: List of blocked SQL statements

### Logging

- `logging.level`: Log level (debug, info, warn, error)
- `logging.format`: Log format (json or text)

## API Endpoints

- `POST /query` - Execute InfluxDB queries (with filtering)
- `GET /proxy_health` - Health check endpoint  
- `GET /proxy_metrics` - Prometheus-style metrics endpoint (if enabled)
- All other REST endpoints exposed by influxdb v1

## Query Filtering Rules

The proxy applies the following filtering rules:

1. **Time Filter Requirement**: Queries must include a time filter (WHERE time > ... AND time < ...)
2. **Time Range Limit**: Time range cannot exceed the configured maximum
3. **Expensive Function Detection**: Blocks queries with expensive functions without proper constraints

## Development

### Project Structure

```text
.
├── cmd/
│   └── proxy/          # Main application entry point
├── internal/
│   ├── banner/         # Application banner and logo
│   ├── config/         # Configuration management
│   ├── filter/         # Query filtering logic
│   ├── proxy/          # HTTP proxy implementation
│   └── influxdb/       # InfluxDB client wrapper
├── pkg/                # Public packages (if any)
├── config.yaml         # Default configuration
├── docker-compose.yml  # For testing with InfluxDB
└── README.md
```

### Building

```bash
go build -o influxdb-proxy cmd/proxy/main.go
```

### Testing

```bash
go test ./...
```

## Docker Support

Use the included `docker-compose.yml` to run the proxy with a test InfluxDB instance:

```bash
docker-compose up
```

## License

MIT License
