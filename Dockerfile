FROM golang:1.24-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum to leverage Docker caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o diet-bot ./cmd/bot

# Use a minimal alpine image for the final stage
FROM alpine:latest

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/diet-bot .

# Copy config file (if exists)
COPY --from=builder /app/config ./config

# Make the binary executable
RUN chmod +x ./diet-bot

# Expose port for HTTP webhook server
EXPOSE 8080

# Run the application
CMD ["./diet-bot"]