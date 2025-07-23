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
                    │ Query Filtering │
                    │     Engine      |
                    |  (Powered by    │
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
- **Statement Filtering**: Blocks dangerous statements (CREATE, DROP, DELETE)
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
- **Performance Tuning**: HTTP client and server optimization settings
- **Logging Settings**: Structured logging configuration

## Request Flow

### Query Processing

1. **Request Reception**: Client sends query to proxy on port 8087
2. **Query Extraction**: Proxy extracts query string and database from request
3. **Query Parsing**: InfluxDB's InfluxQL library parses and analyzes the query structure
4. **Rule Validation**: Query filter applies configured rules:
   - Requires time filters
   - Validates time range (max 7 days by default)
   - Blocks expensive operations
   - Prevents dangerous statements
5. **Decision**:
   - **If Allowed**: Forward to InfluxDB and return response
   - **If Blocked**: Return HTTP 403 with error message
6. **Metrics Update**: Track query statistics

### Non-Query Processing

1. **Direct Forward**: Proxy forwards request directly to InfluxDB
2. **Response Relay**: Returns InfluxDB response unchanged
3. **Error Handling**: Returns 502 if InfluxDB is unreachable

## Key Features

### Query Protection

- **Time Filter Enforcement**: Blocks queries without WHERE time clauses
- **Time Range Limits**: Prevents queries spanning excessive time periods
- **Statement Control**: Allows only safe read operations
- **Function Filtering**: Blocks expensive aggregation functions

### Performance Optimization

- **Connection Pooling**: Efficient HTTP client with connection reuse
- **Concurrent Processing**: Handles multiple queries simultaneously
- **Timeout Management**: Configurable query timeouts
- **Keep-Alive**: Maintains persistent connections to InfluxDB

### Monitoring & Observability

- **Structured Logging**: JSON/text formatted logs with query details
- **Metrics Endpoint**: `/proxy_metrics` provides query statistics
- **Health Checks**: `/proxy_health` for monitoring proxy status
- **Request Tracing**: Detailed logging of blocked/allowed queries

## Configuration

The proxy is configured via `config.yaml` with these main sections:

- **InfluxDB Connection**: Target database URL and credentials
- **Proxy Settings**: Port, host, and performance parameters
- **Filtering Rules**: Query validation and blocking criteria
- **Logging**: Log level and format settings
- **Metrics**: Enable/disable metrics collection

## Security Considerations

- **Query Injection Protection**: InfluxDB's InfluxQL parser validates syntax and structure
- **Resource Protection**: Prevents expensive queries that could impact performance
- **Statement Restriction**: Blocks potentially dangerous database operations
- **Time-based Filtering**: Ensures queries are bounded to prevent full table scans

## Deployment

The proxy is designed to be deployed as a sidecar or gateway service:

- Docker support with multi-stage builds
- Kubernetes deployment ready
- Graceful shutdown handling
- Health check endpoints for load balancers
