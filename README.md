# InfluxDB Proxy Server

A Go-based proxy server for InfluxDB that filters and rejects expensive queries to protect your database from performance issues.

## Features

- **Query Filtering**: Automatically rejects queries without time filters
- **Time Range Validation**: Prevents queries with excessively long time ranges
- **Performance Protection**: Guards against expensive operations that could impact database performance
- **Configurable Rules**: Customizable filtering rules via configuration file
- **Logging**: Comprehensive logging of filtered queries and proxy activity
- **Health Checks**: Built-in health check endpoints

## Quick Start

1. Clone the repository:
```bash
git clone https://github.com/<your-org>/gm-influxdb-proxy.git
cd gm-influxdb-proxy
```

2. Install dependencies:
```bash
go mod download
```

3. Configure the proxy by editing `config.yaml`:
```yaml
influxdb:
  url: "http://localhost:8086"
  username: ""
  password: ""

proxy:
  port: 8087
  max_time_range_hours: 168  # 7 days
  require_time_filter: true

logging:
  level: "info"
```

4. Run the proxy:
```bash
go run cmd/proxy/main.go
```

## Configuration

The proxy uses a YAML configuration file (`config.yaml`) with the following options:

- `influxdb.url`: URL of the target InfluxDB instance
- `influxdb.username`: InfluxDB username (optional)
- `influxdb.password`: InfluxDB password (optional)
- `proxy.port`: Port for the proxy server
- `proxy.max_time_range_hours`: Maximum allowed time range in hours
- `proxy.require_time_filter`: Whether to require time filters in queries
- `logging.level`: Log level (debug, info, warn, error)

## API Endpoints

- `POST /query` - Execute InfluxDB queries (with filtering)
- `GET /health` - Health check endpoint
- `GET /metrics` - Basic metrics endpoint

## Query Filtering Rules

The proxy applies the following filtering rules:

1. **Time Filter Requirement**: Queries must include a time filter (WHERE time > ... AND time < ...)
2. **Time Range Limit**: Time range cannot exceed the configured maximum
3. **Expensive Function Detection**: Blocks queries with expensive functions without proper constraints

## Development

### Project Structure

```
.
├── cmd/
│   └── proxy/          # Main application entry point
├── internal/
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
