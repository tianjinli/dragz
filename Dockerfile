# first stage: build
FROM golang:1.25 AS builder
WORKDIR /app

COPY bootstrap.* go.* /app/
COPY assets /app/assets/
COPY "cmd" "/app/cmd/"
COPY internal /app/internal
COPY pkg /app/pkg

ARG APP_VERSION=0.0.1
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags \
    "-s -w -X github.com/tianjinli/dragz/pkg/appkit.Version=$APP_VERSION" -o main ./cmd/dragz

# second stage: run
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache tzdata
COPY --from=builder /app/main /app/bootstrap.yaml /app/
COPY --from=builder /app/assets /app/assets

CMD ["/app/main"]

# TAG=$(git describe --tags --abbrev=0 | sed 's/^v//')
# docker build --build-arg APP_VERSION=$TAG -t hub.dragz.io/dragz:$TAG .
# docker push hub.dragz.io/dragz:$TAG

# kubectl set image deployment/dragz-deployment server=hub.dragz.io/dragz:1.0.1 -n dragz-prod