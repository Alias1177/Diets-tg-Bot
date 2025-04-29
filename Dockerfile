# Dockerfile
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY ../diet-bot .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fitness-bot ./cmd/bot

# Use alpine for final image
FROM alpine:latest

# Add CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates && \
    update-ca-certificates

# Add unprivileged user
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy built binary (removed the config copy that was causing the error)
COPY --from=builder /app/fitness-bot /app/

# Set proper permissions
RUN chown -R appuser:appuser /app

# Switch to unprivileged user
USER appuser

# Run application
CMD ["./fitness-bot"]