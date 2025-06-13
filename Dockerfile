# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install git (needed for go mod download)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mailcast-service ./cmd/server

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/mailcast-service .

# Copy templates directory if it contains template files
COPY --from=builder /app/templates ./templates

# Change ownership to non-root user
RUN chown -R appuser:appgroup /root
USER appuser

# Expose port (adjust if your service uses a different port)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
CMD ["./mailcast-service"]