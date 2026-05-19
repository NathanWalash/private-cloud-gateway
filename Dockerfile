# Private Cloud Gateway — multi-stage build
#
# Stage 1: build the Vite web app
# Stage 2: download Go dependencies (cached layer)
# Stage 3: compile Go binary with embedded web app
# Stage 4: test stage (used by CI: docker build --target tester)
# Stage 5: minimal runtime image

# ── Web app ───────────────────────────────────────────────────────────────────
FROM node:22-alpine AS web-builder

WORKDIR /web

# Use npm ci for reproducible installs in Docker (no pnpm approval quirks).
# pnpm is used locally for development.
COPY apps/web/package.json apps/web/pnpm-lock.yaml ./
RUN npm install --legacy-peer-deps

COPY apps/web/ .
RUN npm run build

# ── Go dependencies ───────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS go-deps

WORKDIR /build
COPY apps/core/go.mod apps/core/go.sum ./
RUN go mod download

# ── Go builder ────────────────────────────────────────────────────────────────
FROM go-deps AS builder

COPY apps/core/ .

# Replace placeholder with the real built web app
COPY --from=web-builder /web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cloud-core .

# ── Test stage (CI: docker build --target tester) ─────────────────────────────
# -race requires CGO; the race detector runs in the go-test CI job instead.
FROM builder AS tester
RUN CGO_ENABLED=0 go test -v -count=1 ./...

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/cloud-core .
RUN mkdir -p /data /backups

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["./cloud-core"]
