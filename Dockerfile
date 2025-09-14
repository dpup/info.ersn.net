# Multi-stage build for ERSN Info Server

###############################################################################
# Stage 1: Build the Go application
###############################################################################
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

# Install required system dependencies for building
RUN apk add --no-cache git protobuf protobuf-dev make

# Install protoc-gen-go and protoc-gen-go-grpc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

# Cache Go modules by copying go.mod and go.sum first
COPY go.mod go.sum ./
RUN go mod download

# Copy proto files and generate protobuf code
COPY api/ ./api/

# Generate protobuf code directly (no need for Makefile at this stage)
RUN mkdir -p bin api/v1 && \
    PATH="/go/bin:${PATH}" protoc --proto_path=api/v1 \
        --proto_path=$(go list -f '{{ .Dir }}' -m github.com/googleapis/googleapis) \
        --go_out=api/v1 --go_opt=paths=source_relative \
        --go-grpc_out=api/v1 --go-grpc_opt=paths=source_relative \
        --grpc-gateway_out=api/v1 --grpc-gateway_opt=paths=source_relative \
        api/v1/*.proto

# Copy the rest of the application code
COPY . .

# Build the Go application with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /ersn-server ./cmd/server

###############################################################################
# Stage 2: Final lightweight runtime image
###############################################################################
FROM alpine:3.19

WORKDIR /app

# Add basic runtime dependencies and security updates
RUN apk add --no-cache ca-certificates tzdata && \
    apk upgrade

# Create a non-root user to run the application
RUN addgroup -S ersn && adduser -S ersn -G ersn

# Copy the binary from the build stage
COPY --from=go-builder /ersn-server /app/ersn-server

# Copy the configuration file
COPY --from=go-builder /app/prefab.yaml /app/prefab.yaml

# Set ownership to the non-root user
RUN chown -R ersn:ersn /app

# Switch to the non-root user
USER ersn

# Expose the application port
EXPOSE 8080

# Server configuration
ENV PORT=8080

# Prefab framework configuration
ENV PF__SERVER__HOST=0.0.0.0
ENV PF__SERVER__PORT=8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the application
ENTRYPOINT ["/app/ersn-server"]