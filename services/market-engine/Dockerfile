# Build stage
FROM rust:1.81-slim AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

COPY services/market-engine/Cargo.toml services/market-engine/Cargo.lock ./
COPY services/market-engine/src ./src

RUN cargo build --release

# Final stage
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libssl3 \
    && rm -rf /var/lib/apt/lists/*

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

COPY --from=builder /app/target/release/market-engine .

RUN chown -R appuser:appgroup /app

USER appuser

# Health check - the binary supports a health subcommand
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ./market-engine health || exit 1

ENTRYPOINT ["./market-engine"]