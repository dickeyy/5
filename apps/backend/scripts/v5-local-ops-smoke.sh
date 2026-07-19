#!/bin/sh
set -eu

base_url="${QUACK_BASE_URL:-http://127.0.0.1:8080}"

curl --fail --silent --show-error "${base_url}/livez"
curl --fail --silent --show-error "${base_url}/readyz"

if [ -n "${QUACK_METRICS_TOKEN:-}" ]; then
  printf 'header = "X-Quack-Metrics-Key: %s"\nurl = "%s/metrics"\nfail\nsilent\nshow-error\n' \
    "${QUACK_METRICS_TOKEN}" "${base_url}" | curl --config -
fi

curl --fail --silent --show-error -X OPTIONS \
  -H "Origin: ${QUACK_CORS_ORIGIN:-http://localhost:3000}" \
  -H "Access-Control-Request-Method: PUT" \
  "${base_url}/guilds/smoke/modules/honeypot/settings"
