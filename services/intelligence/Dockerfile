# Build stage
FROM python:3.13-slim AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    python3-dev \
    && rm -rf /var/lib/apt/lists/*

COPY services/intelligence/requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

COPY services/intelligence/ .

# Final stage
FROM python:3.13-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

COPY --from=builder /root/.local /home/appuser/.local
COPY --from=builder /app /app

ENV PATH=/home/appuser/.local/bin:$PATH
ENV PYTHONPATH=/app

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8001

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8001/health || exit 1

ENTRYPOINT ["python", "-m", "app.main"]