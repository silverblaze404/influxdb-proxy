# InfluxDB Proxy Server

A Go-based proxy server for InfluxDB v1 that filters and rejects expensive queries to protect your database from performance issues.

## Features

- **Query Filtering**: Automatically rejects queries without time filters
- **Time Range Validation**: Prevents queries with excessively long time ranges
- **Measurement Control**: Optional whitelist of allowed measurement names
- **Configurable Rules**: Customizable filtering rules via configuration file
- **Performance Protection**: Guards against expensive operations that could impact database performance
- **Logging**: Comprehensive logging of filtered queries and proxy activity
- **Health Checks**: Built-in health check endpoints
- **Metrics**: Prometheus-style metrics endpoint for monitoring
- **IP Whitelisting**: Configure trusted IP addresses/CIDR ranges that bypass all filtering rules

## Quick Start

Clone the repository:

```bash
git clone https://github.com/greyorange-labs/influxdb-proxy.git
cd influxdb-proxy
```

Install dependencies:

```bash
go mod download
```

Configure the proxy by editing `config.yaml`:

```yaml
influxdb:
  host: "localhost"
  port: 8086
  username: ""
  password: ""
  database: ""

proxy:
  port: 8087
  host: "localhost"
  whitelisted_ips:            # IPs that bypass all filtering rules
    # - "192.168.1.100"       # Example: specific IP
    # - "10.0.0.0/8"          # Example: CIDR range
    # - "127.0.0.1"           # Example: localhost
  influxdb_client:
    timeout_seconds: 120              # Max timeout (default: 120 seconds)
    idle_connection_pool_size: 200    # How many idle connections to keep to InfluxDB
    max_concurrent_connections: 400   # Max active connections to InfluxDB
    read_timeout_seconds: 60          # HTTP read timeout
    idle_timeout_seconds: 120         # HTTP idle timeout (keep-alive)
  server_timeouts:                    # HTTP server timeout settings (defaults to 2x client timeouts)
    # read_timeout_seconds: 60    # Server read timeout (default: 2x client read timeout)
    # write_timeout_seconds: 240  # Server write timeout (default: 2x client write timeout)  
    # idle_timeout_seconds: 240   # Server idle timeout (default: 2x client idle timeout)
  filtering_rules:
    # Time-based filtering
    require_time_filter: true
    max_time_range_hours: 720          # 30 days max time range

    # Performance filtering
    block_wildcard_select: false       # Block SELECT * without LIMIT
    block_unlimited_group_by: false    # Block GROUP BY without LIMIT or time bucketing
    block_expensive_shows: false       # Block SHOW SERIES without LIMIT
    max_show_series_limit: 10000       # Max LIMIT for SHOW SERIES queries
    
    # Measurement-based filtering
    allowed_measurements:              # If not empty, only allow queries on these measurements
      # - "cpu"                        # Example: allow queries on 'cpu' measurement
      # - "memory"                     # Example: allow queries on 'memory' measurement
      # - "disk"                       # Example: allow queries on 'disk' measurement
    
    # Function/statement filtering
    blocked_functions:
      # - "count(*)"                   # Block count(*) without WHERE clause
    blocked_statements:
      - "DELETE"                      # Block DELETE statements
      - "DROP"                        # Block DROP statements

logging:
  level: "info"
  format: "json"  # json or text

metrics:
  enabled: true
```

Run the proxy:

```bash
go run cmd/proxy/main.go
```

## Configuration

The proxy uses a YAML configuration file (`config.yaml`) with the following main sections:

### InfluxDB Connection

- `influxdb.host`: Hostname of the target InfluxDB instance
- `influxdb.port`: Port of the target InfluxDB instance
- `influxdb.username`: InfluxDB username (optional)
- `influxdb.password`: InfluxDB password (optional)
- `influxdb.database`: Default database name (optional)

### Proxy Settings

- `proxy.port`: Port for the proxy server (default: 8087)
- `proxy.host`: Host interface to bind to (default: "localhost")
- `proxy.disable_query_filtering`: Disable all query filtering (default: false)
- `proxy.whitelisted_ips`: List of IP addresses/CIDR ranges that bypass all filtering rules

### InfluxDB Client Configuration

- `proxy.influxdb_client.timeout_seconds`: Maximum timeout in seconds (default: 120)
- `proxy.influxdb_client.idle_connection_pool_size`: Number of idle connections to keep to InfluxDB (default: 200)
- `proxy.influxdb_client.max_concurrent_connections`: Maximum active connections to InfluxDB (default: 400)
- `proxy.influxdb_client.read_timeout_seconds`: HTTP read timeout for requests (default: 60)
- `proxy.influxdb_client.idle_timeout_seconds`: HTTP idle timeout for keep-alive connections (default: 120)

### Server Timeout Configuration

- `proxy.server_timeouts.read_timeout_seconds`: HTTP server read timeout (default: 2x InfluxDB client read timeout)
- `proxy.server_timeouts.write_timeout_seconds`: HTTP server write timeout (default: 2x InfluxDB client write timeout)
- `proxy.server_timeouts.idle_timeout_seconds`: HTTP server idle timeout (default: 2x InfluxDB client idle timeout)

### Filtering Rules

- `proxy.filtering_rules.require_time_filter`: Require time filters in queries
- `proxy.filtering_rules.max_time_range_hours`: Maximum allowed time range (default: 720 hours/30 days)
- `proxy.filtering_rules.block_wildcard_select`: Block SELECT * without LIMIT
- `proxy.filtering_rules.block_unlimited_group_by`: Block GROUP BY without LIMIT or time bucketing
- `proxy.filtering_rules.block_expensive_shows`: Block SHOW SERIES without LIMIT
- `proxy.filtering_rules.max_show_series_limit`: Maximum LIMIT for SHOW SERIES queries
- `proxy.filtering_rules.allowed_measurements`: Whitelist of allowed measurement names (if specified, only these are allowed)
- `proxy.filtering_rules.blocked_functions`: List of blocked functions
- `proxy.filtering_rules.blocked_statements`: List of blocked SQL statements (all others allowed by default)

### Logging

- `logging.level`: Log level (debug, info, warn, error)
- `logging.format`: Log format (json or text)

### Metrics

- `metrics.enabled`: Enable/disable metrics collection (endpoint: /proxy_metrics)

## API Endpoints

- `POST /query` - Execute InfluxDB queries (with filtering)
- `GET /proxy_health` - Health check endpoint  
- `GET /proxy_metrics` - Prometheus-style metrics endpoint (if enabled)
- All other REST endpoints exposed by influxdb v1

## Query Filtering Rules

The proxy applies the following filtering rules:

1. **IP Whitelisting**: IPs in the `whitelisted_ips` list bypass all filtering rules
2. **Time Filter Requirement**: Queries must include a time filter (WHERE time > ... AND time < ...)
3. **Time Range Limit**: Time range cannot exceed the configured maximum (default: 30 days)
4. **Measurement Filtering**: If `allowed_measurements` is configured, only queries on those measurements are allowed
5. **Expensive Function Detection**: Blocks queries with expensive functions without proper constraints
6. **SHOW Series Limits**: Controls SHOW SERIES queries with configurable limits
7. **Wildcard SELECT Protection**: Can block SELECT * queries without LIMIT clauses
8. **GROUP BY Protection**: Can block unlimited GROUP BY queries
9. **Statement Blocking**: Only blocks statements explicitly listed in `blocked_statements` (all others allowed by default)

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
