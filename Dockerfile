# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies (gcc, musl-dev needed for SQLite)
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary with CGO enabled (required for SQLite)
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o download-proxy .

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and wget for healthcheck
RUN apk --no-cache add ca-certificates wget

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/download-proxy .

# Copy migrations
COPY --from=builder /app/db/migrations ./db/migrations

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
CMD ["./download-proxy"]
