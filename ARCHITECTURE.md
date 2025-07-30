# InfluxDB Proxy Architecture

## Overview

The InfluxDB Proxy is a Go-based middleware server that sits between clients and InfluxDB v1 to filter and control expensive queries. It protects the database from performance issues by rejecting queries that don't meet configured filtering rules.

## Architecture Diagram

```text
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│   Client    │────▶│  InfluxDB Proxy  │────▶│  InfluxDB   │
│ Application │     │   (Port 8087)    │     │ (Port 8086) │
└─────────────┘     └──────────────────┘     └─────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ IP Whitelist    │
                    │   Check         │
                    └─────────────────┘
                             │
                    ┌────────┴────────┐
                    │                 │
                    ▼                 │
              ┌──────────┐            │
              │  Bypass  │            │
              │   All    │            │
              │ Filters  │            │
              └──────────┘            │
                                      ▼
                             ┌─────────────────┐
                             │ Query Filtering │
                             │     Engine      │
                             │  (Powered by    │
                             │    InfluxQL)    │
                             └─────────────────┘
                                      │
                             ┌────────┴────────┐
                             │                 │
                             ▼                 ▼
                       ┌──────────┐    ┌─────────────┐
                       │  Allow   │    │   Block &   │
                       │   and    │    │   Return    │
                       │ Forward  │    │   Error     │
                       └──────────┘    └─────────────┘
```

## Core Components

### Proxy Server

Located in `internal/proxy/server.go`, the proxy server handles:

- **HTTP Server**: Listens on port 8087 (configurable)
- **Request Router**: Uses Gorilla Mux for routing
- **Query Handler**: Processes `/query` endpoint with filtering
- **Forward Handler**: Forwards non-query requests directly to InfluxDB
- **Metrics Collection**: Tracks query statistics

### Query Filter

Located in `internal/filter/query_filter.go`, the filter engine provides:

- **Query Parser**: Uses InfluxDB's official InfluxQL library to parse and analyze queries
- **Rule Engine**: Applies configurable filtering rules
- **Time Validation**: Ensures queries have time filters and reasonable time ranges
- **Statement Filtering**: Blocks only statements explicitly listed in configuration (blacklist approach)
- **Function Blocking**: Prevents expensive functions like `count(*)`

### InfluxDB Client

Located in `internal/influxdb/client.go`, the client manages:

- **HTTP Client**: Optimized for high concurrency with connection pooling
- **Query Execution**: Forwards allowed queries to InfluxDB
- **Request Forwarding**: Passes through non-query requests transparently
- **Health Checks**: Monitors InfluxDB connectivity

### Configuration Manager

Located in `internal/config/config.go`, configuration handles:

- **YAML Configuration**: Loads settings from `config.yaml`
- **Filtering Rules**: Configurable query filtering parameters
- **InfluxDB Client Configuration**: HTTP client optimization settings for connecting to InfluxDB
- **Logging Settings**: Structured logging configuration

## Request Flow

### Query Processing

1. **Request Reception**: Client sends query to proxy on port 8087
2. **IP Whitelist Check**: Check if client IP is in the whitelisted IPs list
   - **If Whitelisted**: Skip all filtering rules and forward directly to InfluxDB
   - **If Not Whitelisted**: Proceed to query filtering
3. **Query Extraction**: Proxy extracts query string and database from request
4. **Query Parsing**: InfluxDB's InfluxQL library parses and analyzes the query structure
5. **Rule Validation**: Query filter applies configured rules:
   - Requires time filters
   - Validates time range (max 30 days by default)
   - Checks allowed measurements (if configured)
   - Blocks expensive operations
   - Blocks only statements explicitly listed in `blocked_statements`
6. **Decision**:
   - **If Allowed**: Forward to InfluxDB and return response
   - **If Blocked**: Return HTTP 403 with error message
7. **Metrics Update**: Track query statistics

### Non-Query Processing

1. **Direct Forward**: Proxy forwards request directly to InfluxDB
2. **Response Relay**: Returns InfluxDB response unchanged
3. **Error Handling**: Returns 502 if InfluxDB is unreachable

## Key Features

### Query Protection

- **IP Whitelisting**: Configurable IP addresses/CIDR ranges that bypass all filtering rules
- **Time Filter Enforcement**: Blocks queries without WHERE time clauses
- **Time Range Limits**: Prevents queries spanning excessive time periods (default: 30 days)
- **Measurement Filtering**: Optional whitelist of allowed measurement names
- **Statement Control**: Uses blacklist approach - blocks only explicitly configured statements
- **Function Filtering**: Blocks expensive aggregation functions
- **SHOW Series Control**: Configurable limits for SHOW SERIES queries
- **Wildcard SELECT Protection**: Optional blocking of SELECT * without LIMIT
- **GROUP BY Protection**: Optional blocking of unlimited GROUP BY queries

### InfluxDB Client Optimization

- **Connection Pooling**: Configurable idle connection pool optimized for single InfluxDB host
- **Concurrent Processing**: Handles multiple queries simultaneously with tunable connection limits
- **Timeout Management**: Configurable read, write, and idle timeouts for InfluxDB connections
- **Keep-Alive**: Maintains persistent connections to InfluxDB with configurable idle timeout

### Monitoring & Observability

- **Structured Logging**: JSON/text formatted logs with query details
- **Metrics Endpoint**: `/proxy_metrics` provides query statistics
- **Health Checks**: `/proxy_health` for monitoring proxy status
- **Request Tracing**: Detailed logging of blocked/allowed queries

## Configuration

The proxy is configured via `config.yaml` with these main sections:

- **InfluxDB Connection**: Target database host, port and credentials
- **Proxy Settings**: Port, host, timeout, whitelisted IPs, and query filtering controls
- **InfluxDB Client Parameters**: Connection pooling and timeout settings optimized for single-host InfluxDB connections
- **Server Timeout Parameters**: HTTP server timeout configuration with smart defaults (2x client timeouts)
- **Filtering Rules**: Query validation and blocking criteria including:
  - Time-based filtering (require time filters, max time range)
  - Performance filtering (wildcard SELECT, unlimited GROUP BY, expensive SHOW queries)
  - Measurement-based filtering (allowed measurement whitelist)
  - Function/statement filtering (blocked functions and statements)
- **Query Filtering Control**: Global disable option (`disable_query_filtering`) to bypass all filtering
- **Logging**: Log level and format settings
- **Metrics**: Enable/disable metrics collection

## Security Considerations

- **Query Injection Protection**: InfluxDB's InfluxQL parser validates syntax and structure
- **Resource Protection**: Prevents expensive queries that could impact performance
- **IP-based Access Control**: Bypass filtering rules on the basis of whitelised ips and blacklisted ips configuration
- **Statement Restriction**: Blocks only explicitly configured statements (blacklist approach)
- **Time-based Filtering**: Ensures queries are bounded to prevent full table scans
- **Measurement Access Control**: Optional whitelist to restrict access to specific measurements

## Filtering Strategy

The proxy uses a **blacklist-based filtering approach** for statement control:

- **Default Behavior**: All statement types are allowed by default
- **Explicit Blocking**: Only statements listed in `blocked_statements` configuration are denied
- **Simple Logic**: If a statement type is in the blocked list → reject, otherwise → allow

This approach provides:

- **Simplicity**: Only one configuration list to manage
- **Flexibility**: Easy to allow new statement types without configuration changes
- **Intuitive Behavior**: Everything works unless explicitly blocked

Example configuration:

```yaml
filtering_rules:
  blocked_statements:
    - "DELETE"    # Block DELETE operations
    - "CREATE"    # Block CREATE operations
  # All other statements (SELECT, SHOW, DROP, etc.) are automatically allowed
```

## Deployment

The proxy is designed to be deployed as a sidecar or gateway service:

- Docker support with multi-stage builds
- Kubernetes deployment ready
- Graceful shutdown handling
- Health check endpoints for load balancers
