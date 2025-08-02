FROM golang:1.24.0 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/app ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache \
        ffmpeg \
        ca-certificates \
        fontconfig \
        font-roboto-flex && \
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

