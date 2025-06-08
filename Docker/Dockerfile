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

# Install CA certificates
RUN apk add --no-cache ca-certificates

# Set the working directory inside the container
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/allama .

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./allama"]
