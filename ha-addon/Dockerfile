# SmartPlug Controller - Home Assistant Add-on
# Multi-stage build for minimal image size

ARG BUILD_FROM=ghcr.io/home-assistant/aarch64-base:latest
ARG BUILD_ARCH=aarch64

# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the controller binary
ARG TARGETOS=linux
ARG TARGETARCH=arm64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o smartplug-controller ./cmd/smartplug-controller

# Stage 2: Runtime image
FROM ${BUILD_FROM}

# Install runtime dependencies
RUN apk add --no-cache \
    bash \
    jq \
    ca-certificates

# Copy binary from builder
COPY --from=builder /build/smartplug-controller /usr/local/bin/

# Copy run script
COPY ha-addon/run.sh /run.sh
RUN chmod +x /run.sh

# Create data directory
RUN mkdir -p /data/smartplug

# Expose web UI port
EXPOSE 8080

CMD ["/run.sh"]
