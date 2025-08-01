# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code (excluding scripts folder)
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY config.yaml .
COPY filtering_rules.yaml .

# Build the application
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o influxdb-proxy cmd/proxy/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/influxdb-proxy .
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/filtering_rules.yaml .

# Expose port
EXPOSE 8087

# Run the proxy
CMD ["./influxdb-proxy"]
