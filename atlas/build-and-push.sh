#!/bin/bash
set -e
SECONDS=0

cd "$(dirname "$0")"
DB_VERSION=${TAG:-1.0.0}
echo "DB_VERSION: ${DB_VERSION}"

docker build --build-arg DB_VERSION=${DB_VERSION} -t hub.dragz.io/dragz-db:${DB_VERSION} .
docker push hub.dragz.io/dragz-db:${DB_VERSION}

echo "Elapsed time: $SECONDS seconds."

# TAG=1.0.1 bash build-and-push.sh