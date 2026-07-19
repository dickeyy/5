#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
backend_root="${repo_root}/apps/backend"
contract_path="${repo_root}/contracts/http/swagger.yaml"
output_dir="$(mktemp -d)"

cleanup() {
	rm -rf "${output_dir}"
}
trap cleanup EXIT

cd "${backend_root}"
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init \
	--generalInfo main.go \
	--dir cmd/quack,internal/httpapi,internal/quack,internal/modules \
	--parseInternal \
	--output "${output_dir}" \
	--outputTypes yaml

mv "${output_dir}/swagger.yaml" "${contract_path}"
