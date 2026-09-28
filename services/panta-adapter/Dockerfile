# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
COPY services/panta-adapter/go.mod services/panta-adapter/go.sum ./services/panta-adapter/
COPY packages/types/go.mod ./packages/types/

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o panta-adapter ./services/panta-adapter/cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o smoke-test ./services/panta-adapter/cmd/smoke-test

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

COPY --from=builder /app/panta-adapter .
COPY --from=builder /app/smoke-test .

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

ENTRYPOINT ["./panta-adapter"]