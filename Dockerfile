# -------- Build stage --------
FROM golang:1.25.5-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Enable Go modules
WORKDIR /app

# Copy go mod files first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
# - trimpath: removes local paths
# - ldflags "-s -w": smaller binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o app ./cmd/templates-service

# -------- Runtime stage --------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/app /app/app

# Use non-root user (distroless default)
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/app"]
