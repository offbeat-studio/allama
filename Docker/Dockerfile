# Use the official Golang image as the base image
FROM golang:1.24-alpine

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
RUN go build -o allama ./main.go

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./allama"]
