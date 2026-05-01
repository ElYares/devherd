#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_DIR="${HOME}/.local/bin"

mkdir -p "${TARGET_DIR}"

(
  cd "${ROOT_DIR}"
  go build -o "${TARGET_DIR}/devherd" ./cmd/devherd
)

printf 'devherd instalado en %s/devherd\n' "${TARGET_DIR}"
