# =============================================================================
# rigrun Dockerfile - Multi-stage build for minimal production image
# =============================================================================
# Build: docker build -t rigrun .
# Run:   docker run -p 8787:8787 rigrun
#
# IMPORTANT NOTES:
# - This image does NOT include Ollama. You need to either:
#   1. Run Ollama on your host and use --network host (Linux only)
#   2. Run Ollama in a separate container (see docker-compose.yml)
#   3. Use cloud-only mode with OPENROUTER_KEY environment variable
#
# - For GPU acceleration, use nvidia-docker or --gpus all flag
#   (but note: this rigrun container doesn't need GPU - Ollama does)
# =============================================================================

# -----------------------------------------------------------------------------
# Build Stage - Compile rigrun using rust:alpine for smaller image
# -----------------------------------------------------------------------------
FROM rust:alpine AS builder

# Install build dependencies for musl-based static linking
# musl-dev: C library for static linking
# openssl-dev/openssl-libs-static: For TLS/HTTPS support (reqwest)
# pkgconfig: For finding libraries during compilation
RUN apk add --no-cache \
    musl-dev \
    openssl-dev \
    openssl-libs-static \
    pkgconfig

# Set OpenSSL to use static linking
ENV OPENSSL_STATIC=1
ENV OPENSSL_LIB_DIR=/usr/lib
ENV OPENSSL_INCLUDE_DIR=/usr/include

WORKDIR /app

# Copy only dependency files first for better layer caching
# This allows Docker to cache the dependency build if only source changes
COPY Cargo.toml Cargo.lock ./

# Create a dummy main.rs to build dependencies
# This trick allows caching of compiled dependencies
RUN mkdir -p src && \
    echo 'fn main() { println!("dummy"); }' > src/main.rs && \
    echo 'pub fn dummy() {}' > src/lib.rs

# Build dependencies only (this layer gets cached)
RUN cargo build --release && \
    rm -rf src target/release/deps/rigrun* target/release/rigrun*

# Now copy the actual source code
COPY src ./src

# Build the actual application
# touch ensures cargo detects source changes
RUN touch src/main.rs src/lib.rs && \
    cargo build --release

# Strip the binary to reduce size (saves ~50% typically)
RUN strip target/release/rigrun

# -----------------------------------------------------------------------------
# Runtime Stage - Minimal alpine image for production
# -----------------------------------------------------------------------------
FROM alpine:3.19

# Install only runtime dependencies
# ca-certificates: For HTTPS connections to OpenRouter
# tini: Proper init system for container (handles signals correctly)
RUN apk add --no-cache \
    ca-certificates \
    tini

# Create non-root user for security
RUN adduser -D -u 1000 rigrun

# Create config directory with proper permissions
RUN mkdir -p /home/rigrun/.rigrun && \
    chown -R rigrun:rigrun /home/rigrun/.rigrun

# Copy the compiled binary from builder stage
COPY --from=builder /app/target/release/rigrun /usr/local/bin/rigrun

# Ensure binary is executable
RUN chmod +x /usr/local/bin/rigrun

# Switch to non-root user
USER rigrun
WORKDIR /home/rigrun

# Set default port (can be overridden with -e PORT=xxxx)
ENV RIGRUN_PORT=8787

# Expose the server port
EXPOSE 8787

# Health check - verify server is responding
# Checks the health endpoint every 30 seconds
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8787/health || exit 1

# Use tini as init system for proper signal handling
ENTRYPOINT ["/sbin/tini", "--"]

# Default command - start the server
# Override with: docker run rigrun rigrun status
CMD ["rigrun"]
