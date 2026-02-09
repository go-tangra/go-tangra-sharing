##################################
# Stage 1: Build Go executable
##################################

FROM golang:1.23-alpine AS builder

ARG APP_VERSION=1.0.0

# Enable toolchain auto-download for newer Go versions
ENV GOTOOLCHAIN=auto

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /src

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

# Build the server
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build -ldflags "-X main.version=${APP_VERSION} -s -w" \
    -o /src/bin/sharing-server \
    ./cmd/server

##################################
# Stage 2: Create runtime image
##################################

FROM alpine:3.20

ARG APP_VERSION=1.0.0

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Set timezone
ENV TZ=UTC

# Set working directory
WORKDIR /app

# Copy executable from builder
COPY --from=builder /src/bin/sharing-server /app/bin/sharing-server

# Copy configuration files
COPY --from=builder /src/configs/ /app/configs/

# Create non-root user
RUN addgroup -g 1000 sharing && \
    adduser -D -u 1000 -G sharing sharing && \
    chown -R sharing:sharing /app

# Switch to non-root user
USER sharing:sharing

# Expose gRPC and HTTP ports
EXPOSE 9600 9601

# Set default command
CMD ["/app/bin/sharing-server", "-c", "/app/configs"]

# Labels
LABEL org.opencontainers.image.title="Sharing Service" \
      org.opencontainers.image.description="Share secrets and documents via one-time email links" \
      org.opencontainers.image.version="${APP_VERSION}"
