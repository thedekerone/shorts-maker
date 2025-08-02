# ------------ Build stage ------------
FROM golang:1.24.0 AS builder

WORKDIR /app

COPY entrypoint.sh /usr/local/bin/
ENTRYPOINT ["sh","/usr/local/bin/entrypoint.sh","/bin/app"]
# 1. Install build deps, copy go.{mod,sum}, download modules
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg  \
    go mod download

# 2. Copy source and build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o server ./cmd/server

# ---------- Runtime stage ----------
FROM alpine:3.20

# Install FFmpeg runtime binaries (≈7 MB) & certs
RUN apk add --no-cache ffmpeg ca-certificates

WORKDIR /app
COPY --from=builder /app/server .

ENV PORT=8080
EXPOSE 8080

CMD ["./server"]
