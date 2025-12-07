FROM golang:1.24.0-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download
COPY . .
RUN apt-get update && apt-get install -y --no-install-recommends build-essential pkg-config && \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /bin/app ./cmd/server && \
    apt-get purge -y build-essential pkg-config && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
        ffmpeg \
        ca-certificates \
        fontconfig \
        fonts-roboto \
        libsqlite3-0 && \
    rm -rf /var/lib/apt/lists/* && \
    fc-cache -f -v
COPY --from=builder /bin/app /bin/app
COPY --from=builder /app/assets /app/assets
COPY --from=builder /app/handlers /app/handlers
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
ENV GOOGLE_APPLICATION_CREDENTIALS=/var/secrets/google/key.json \
    PORT=8080
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["sh", "/usr/local/bin/entrypoint.sh", "/bin/app"]

