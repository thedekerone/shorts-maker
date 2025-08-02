# ---------- Build stage ----------
FROM golang:1.24.0 AS builder

WORKDIR /app

# 1. Leverage layer caching for deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg \
    go mod download

# 2. Compile the server
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /bin/app ./cmd/server

# ---------- Runtime stage ----------
FROM alpine:3.20

# Install FFmpeg runtime + TLS certs
RUN apk add --no-cache ffmpeg ca-certificates

# Copy the compiled binary and the entry-point script
COPY --from=builder /bin/app /bin/app
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Conventional env-var for Google libraries
# (override or mount a secret in Coolify as needed)
ENV GOOGLE_APPLICATION_CREDENTIALS=/var/secrets/google/key.json \
    PORT=8080

WORKDIR /app        # optional
EXPOSE 8080

# Script runs first; it should finish by exec-ing the Go binary
ENTRYPOINT ["sh","/usr/local/bin/entrypoint.sh","/bin/app"]

