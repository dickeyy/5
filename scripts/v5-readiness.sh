#!/usr/bin/env bash
set -euo pipefail

mode="${1:---local}"
case "$mode" in
  --local|--final) ;;
  *)
    echo "usage: $0 [--local|--final]" >&2
    exit 2
    ;;
esac

export GOCACHE="${GOCACHE:-/tmp/quack-v5-readiness-gocache}"

go test ./internal/readiness ./internal/quack ./internal/store ./internal/httpapi/... ./internal/discordbot/... ./internal/moduleintegration ./internal/modules/...
go test -race ./internal/quack ./internal/store ./internal/httpapi/platform ./internal/discordbot/interactions ./internal/moduleintegration ./internal/modules/...
go test ./...
go vet ./...
go build -o /tmp/quack-v5-readiness-quack ./cmd/quack
go build -o /tmp/quack-v5-readiness-migrate ./cmd/quack-migrate

if [[ "$mode" == "--final" ]]; then
  if [[ -z "${QUACK_TEST_MYSQL_DSN:-}" ]]; then
    echo "final readiness requires QUACK_TEST_MYSQL_DSN; MySQL evidence was NOT EXECUTED" >&2
    exit 1
  fi
  QUACK_TEST_MYSQL_DSN="$QUACK_TEST_MYSQL_DSN" go test ./internal/store ./internal/modules/... ./internal/readiness

  if [[ -z "${QUACK_TEST_REDIS_URL:-}" ]]; then
    echo "final readiness requires QUACK_TEST_REDIS_URL; real Redis evidence was NOT EXECUTED" >&2
    exit 1
  fi
  if ! command -v redis-cli >/dev/null 2>&1; then
    echo "final readiness requires redis-cli for the external Redis probe" >&2
    exit 1
  fi
  redis-cli -u "$QUACK_TEST_REDIS_URL" --no-auth-warning PING | grep -qx PONG
fi

git diff --check
