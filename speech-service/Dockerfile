# syntax=docker/dockerfile:1.7
# Build stage — compila o binário estático
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /out/server .

# Runtime stage — imagem final enxuta (~15MB)
FROM alpine:3.20

RUN apk --no-cache add ca-certificates && \
    addgroup -S app && adduser -S -G app app

COPY --from=builder /out/server /usr/local/bin/server

USER app
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
