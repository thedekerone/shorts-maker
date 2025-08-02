# ------------ Build stage ------------
FROM golang:1.22-alpine AS builder

# Enable modules, set up workdir
WORKDIR /app

COPY entrypoint.sh /usr/local/bin/
ENTRYPOINT ["sh","/usr/local/bin/entrypoint.sh","/bin/app"]
# Copy go.{mod,sum} first to leverage Docker layer-caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o server ./cmd/server

# ------------ Runtime stage ------------
# You can swap to `scratch` if you don’t need /etc/passwd, TLS certs, etc.
FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=builder /app/server .

# Let platforms like Coolify know which port we listen on
ENV PORT=8080
EXPOSE 8080

# Start the server
CMD ["./server"]

