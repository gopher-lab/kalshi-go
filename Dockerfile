# Build stage
FROM golang:latest AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the autorun bot
RUN CGO_ENABLED=0 GOOS=linux go build -o /lahigh-autorun ./cmd/lahigh-autorun/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /lahigh-autorun /app/lahigh-autorun

# Set timezone to LA
ENV TZ=America/Los_Angeles

# Default command
ENTRYPOINT ["/app/lahigh-autorun"]
CMD ["-event", "KXHIGHLAX-25DEC27"]

