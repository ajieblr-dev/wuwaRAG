# Multi-stage Dockerfile for Wuthering Waves Lore RAG Service

# Stage 1: Build binaries
FROM golang:1.24-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build API server and CLI ingestion tool
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/ingest ./cmd/ingest

# Stage 2: Minimal runtime image
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Create unprivileged user
RUN addgroup -S wuwa && adduser -S wuwa -G wuwa

COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/ingest /app/ingest
COPY --chown=wuwa:wuwa web /app/web

USER wuwa

EXPOSE 8080

ENTRYPOINT ["/app/api"]
