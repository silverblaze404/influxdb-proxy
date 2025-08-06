Makefile.PHONY: build run test clean docker

# Build the application
build:
	go build -o influxdb-proxy cmd/proxy/main.go

# Run the application
run:
	go run cmd/proxy/main.go

# Run with custom config
run-config:
	go run cmd/proxy/main.go -config config.yaml

# Run tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f influxdb-proxy
	rm -f coverage.out coverage.html
	go clean -cache

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build Docker image
docker-build:
	docker build -t influxdb-proxy .

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Install development tools
dev-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Generate example queries for testing
example-queries:
	@echo "Valid query:"
	@echo "curl -X POST 'http://localhost:8087/query' -d 'q=SELECT value FROM cpu WHERE time > now() - 1h AND time < now()&db=mydb'"
	@echo ""
	@echo "Invalid query (no time filter):"
	@echo "curl -X POST 'http://localhost:8087/query' -d 'q=SELECT * FROM cpu&db=mydb'"
	@echo ""
	@echo "Health check:"
	@echo "curl http://localhost:8087/proxy_health"
	@echo ""
	@echo "Metrics:"
	@echo "curl http://localhost:8087/proxy_metrics"

# Sanity checks
sanity:
	go clean -cache
	go fmt ./...
	go test ./... -v
	go build -o influxdb-proxy cmd/proxy/main.go
