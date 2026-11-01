FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/admin-api ./cmd/admin-api

FROM alpine:3.23
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/admin-api /app/admin-api
COPY etc/admin-api.docker.yaml /app/etc/admin-api.yaml
RUN mkdir -p /app/data/uploads && chown -R app:app /app
USER app
EXPOSE 5280
ENTRYPOINT ["/app/admin-api", "-f", "/app/etc/admin-api.yaml"]
