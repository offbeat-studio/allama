# Use the official Golang image as the base image
FROM golang:1.21-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o allama ./cmd/allama

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./allama"]
