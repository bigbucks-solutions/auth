# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o auth .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/auth .

# Copy necessary files (adjust based on your needs)
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/migrations/*.sql ./migrations/

# Create directory for profile pics if needed
RUN mkdir -p /app/profile_pics

# Expose ports (adjust based on your application)
EXPOSE 8080 50051

# Run the application
CMD ["./auth"]
