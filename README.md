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
  host: "localhost"
  port: 8086
  username: ""
  password: ""
  database: ""

proxy:
  port: 8087
  host: "localhost"
  max_query_timeout: 30       # seconds
  whitelisted_ips:            # IPs that bypass all filtering rules
    # - "192.168.1.100"       # Example: specific IP
    # - "10.0.0.0/8"          # Example: CIDR range
    # - "127.0.0.1"           # Example: localhost
  performance:
    max_idle_conns: 200           # Total idle connections to InfluxDB
    max_idle_conns_per_host: 50   # Idle connections per InfluxDB host
    max_conns_per_host: 100       # Max concurrent connections per InfluxDB host
    read_timeout_seconds: 30      # HTTP read timeout
    write_timeout_seconds: 120    # HTTP write timeout (for large responses)
    idle_timeout_seconds: 120     # HTTP idle timeout (keep-alive)
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
  path: "/proxy_metrics"
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
- `proxy.max_query_timeout`: Maximum query timeout in seconds
- `proxy.whitelisted_ips`: List of IP addresses/CIDR ranges that bypass all filtering rules

### Performance Tuning

- `proxy.performance.max_idle_conns`: Total idle connections to InfluxDB
- `proxy.performance.max_idle_conns_per_host`: Idle connections per InfluxDB host
- `proxy.performance.max_conns_per_host`: Max concurrent connections per host
- `proxy.performance.read_timeout_seconds`: HTTP read timeout for requests
- `proxy.performance.write_timeout_seconds`: HTTP write timeout for large responses
- `proxy.performance.idle_timeout_seconds`: HTTP idle timeout for keep-alive connections

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
