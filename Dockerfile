# Build stage
FROM golang:1.26.1-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
ARG BUILDTIME
ARG GITSHA

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILDTIME} -X main.GitSHA=${GITSHA}" \
    -o agent-harness ./cmd/agent-harness

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates git

# Create non-root user
RUN adduser -D -u 1000 agent

# Copy binary
COPY --from=builder /build/agent-harness /usr/local/bin/agent-harness

# Set permissions
RUN chmod +x /usr/local/bin/agent-harness

# Switch to non-root user
USER agent

# Set working directory
WORKDIR /workspace

ENTRYPOINT ["agent-harness"]
