#!/bin/bash
set -e

NETWORK="realty-net"
APP_IMAGE="mobile-api"
APP_CONTAINER="mobile-api"
LOG_DIR="/tmp/mobile-api-logs"

echo "==> Building image: $APP_IMAGE"
docker build -t "$APP_IMAGE" .

echo "==> Stopping existing container (if any)"
docker rm -f "$APP_CONTAINER" 2>/dev/null || true

echo "==> Creating log directory: $LOG_DIR"
mkdir -p "$LOG_DIR"

if [ ! -f "$(pwd)/jwt_secret" ]; then
  echo "==> Generating jwt_secret (first run)"
  (umask 077; openssl rand -base64 48 > "$(pwd)/jwt_secret")
fi

echo "==> Starting container: $APP_CONTAINER"
docker run -d \
  --name "$APP_CONTAINER" \
  --network "$NETWORK" \
  --restart unless-stopped \
  -v "realtyranker_sqlitedata:/data" \
  -p 8090:8090 \
  -p 9097:9097 \
  -v "$(pwd)/config.yaml:/app/config.yaml:ro" \
  -v "$(pwd)/jwt_secret:/etc/mobile-api/jwt_secret:ro" \
  -v "$LOG_DIR:/var/log/mobile-api" \
  "$APP_IMAGE"

echo ""
echo "Useful commands:"
echo "  Logs:    docker logs -f $APP_CONTAINER"
echo "  Metrics: curl http://localhost:9097/metrics"
echo "  Health:  curl http://localhost:8090/healthz"
echo "  Stop:    docker stop $APP_CONTAINER"
