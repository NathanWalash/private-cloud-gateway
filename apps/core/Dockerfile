# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy all source first so go mod tidy can analyse imports and generate go.sum.
# Once go.sum is committed, this can be split into a separate download step for
# better layer caching (COPY go.mod go.sum ./ → RUN go mod download → COPY . .).
COPY . .

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cloud-core .

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/cloud-core .

# Data directory — mount a volume here in production.
RUN mkdir -p /data /backups

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["./cloud-core"]
