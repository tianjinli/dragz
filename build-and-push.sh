#!/bin/bash
set -e
SECONDS=0

cd "$(dirname "$0")"
APP_VERSION=${TAG:-1.0.0}
echo "APP_VERSION: ${APP_VERSION}"

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags \
"-s -w -X github.com/tianjinli/dragz/pkg/appkit.Version=${APP_VERSION}" \
-o ./bin/main ./cmd/dragz

cat <<EOF | docker build --build-arg APP_VERSION=${APP_VERSION} -t hub.dragz.io/dragz:${APP_VERSION} -f - .
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache tzdata
COPY bin/main bootstrap.yaml /app/
COPY assets /app/assets
CMD ["/app/main"]
EOF

docker push hub.dragz.io/dragz:${APP_VERSION} && rm -rf ./bin/

echo "Elapsed time: $SECONDS seconds."

# TAG=1.0.1 bash build-push-docker.sh