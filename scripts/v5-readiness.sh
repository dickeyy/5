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
go build -o /tmp/quack-v5-readiness-v4-import ./cmd/quack-v4-import
go build -o /tmp/quack-v5-readiness-storage-verify ./cmd/quack-storage-verify

if [[ "$mode" == "--final" ]]; then
  if [[ -z "${QUACK_TEST_MYSQL_DSN:-}" ]]; then
    echo "final readiness requires QUACK_TEST_MYSQL_DSN; MySQL evidence was NOT EXECUTED" >&2
    exit 1
  fi
  if [[ -z "${QUACK_TEST_REDIS_URL:-}" ]]; then
    echo "final readiness requires QUACK_TEST_REDIS_URL; real Redis evidence was NOT EXECUTED" >&2
    exit 1
  fi

  QUACK_TEST_MYSQL_DSN="$QUACK_TEST_MYSQL_DSN" \
    QUACK_TEST_REDIS_URL="$QUACK_TEST_REDIS_URL" \
    go test ./...

  redis_probe_namespace="final-$(date -u +%Y%m%dT%H%M%S%N)-$$-${RANDOM}"
  redis_probe_token="$(date -u +%s%N)-$$-${RANDOM}-${RANDOM}"
  REDIS_URL="$QUACK_TEST_REDIS_URL" \
    QUACK_RECOVERY_NAMESPACE="$redis_probe_namespace" \
    QUACK_RECOVERY_TOKEN="$redis_probe_token" \
    /tmp/quack-v5-readiness-storage-verify redis-write
  REDIS_URL="$QUACK_TEST_REDIS_URL" \
    QUACK_RECOVERY_NAMESPACE="$redis_probe_namespace" \
    QUACK_RECOVERY_TOKEN="$redis_probe_token" \
    /tmp/quack-v5-readiness-storage-verify redis-verify
fi

git diff --check
