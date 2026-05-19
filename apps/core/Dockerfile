# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# go.sum is committed — download deps first for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# ── Test stage ────────────────────────────────────────────────────────────────
FROM builder AS tester
RUN CGO_ENABLED=0 go test -v -count=1 ./...

# ── Release stage ─────────────────────────────────────────────────────────────
FROM builder AS release
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cloud-core .

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=release /build/cloud-core .

# Data directory — mount a volume here in production.
RUN mkdir -p /data /backups

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["./cloud-core"]
