# Builder stage
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files from the src directory
COPY src/go.mod src/go.sum ./

# Install necessary build tools for CGO and SQLite
RUN apk add --no-cache gcc musl-dev

# Download dependencies
RUN go mod download

# Copy the source code from the src directory
COPY src/ ./

# Build the application with CGO enabled
ENV CGO_ENABLED=1
RUN go build -o allama -ldflags="-s -w" ./main.go

# Final stage
FROM alpine:latest

# Install CA certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Set the working directory inside the container
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/data /app/logs

# Copy the compiled binary from the builder stage
COPY --from=builder /app/allama .

# Set proper permissions
RUN chmod +x ./allama

# Create non-root user for security
RUN addgroup -g 1001 -S allama && \
    adduser -S allama -u 1001 -G allama

# Change ownership of app directory
RUN chown -R allama:allama /app

# Switch to non-root user
USER allama

# Expose the port the app runs on
EXPOSE 11435

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:11435/health || exit 1

# Command to run the executable
CMD ["./allama"]