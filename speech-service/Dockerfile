# syntax=docker/dockerfile:1.7
# Build context: raiz do repositório (precisa enxergar ./speech-service e ./frontend)
# Build stage — compila o binário estático
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY speech-service/go.mod speech-service/go.sum ./
RUN go mod download

COPY speech-service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /out/server .

# Runtime stage — imagem final enxuta
FROM alpine:3.20

RUN apk --no-cache add ca-certificates wget && \
    addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/server /usr/local/bin/server
COPY frontend/ ./public/

ENV STATIC_DIR=/app/public

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/server"]
