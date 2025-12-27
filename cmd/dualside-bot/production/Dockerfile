# Production Dual-Side Trading Bot
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bot ./cmd/dualside-bot/production

# Runtime image
FROM alpine:latest

WORKDIR /app

# Install ca-certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Copy binary
COPY --from=builder /bot /app/bot

# Create data directory
RUN mkdir -p /data

# Expose health check port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
ENTRYPOINT ["/app/bot"]

