# Multi-stage build for spicedb-kubeapi-proxy integration
FROM golang:1.25-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /workspace

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application for linux/amd64
# RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -o server \
    ./cmd/server

# Final stage
FROM alpine:3.18

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /workspace/server .

# Create non-root user
RUN addgroup -g 65532 -S nonroot && \
    adduser -u 65532 -S nonroot -G nonroot

USER nonroot:nonroot

ENTRYPOINT ["/server"]
