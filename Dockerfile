FROM node:24-alpine AS frontend
WORKDIR /app/admin
COPY admin/package*.json ./
RUN npm ci
COPY admin/ ./
RUN npm run build

FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go vet ./cmd/... ./internal/...
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o groupbot ./cmd/bot/

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata curl
RUN adduser -D -g '' appuser

COPY --from=builder /app/groupbot .
COPY --from=builder /app/prompts prompts/
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/protocols.json .
COPY --from=builder /app/messages.yaml .
COPY --from=frontend /app/admin/dist admin/dist

RUN mkdir -p /app/data && chown -R appuser:appuser /app
USER appuser
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
  CMD curl -f http://localhost:8081/health || exit 1
ENTRYPOINT ["/app/groupbot"]
