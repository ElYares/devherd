#!/usr/bin/env bash
set -euo pipefail

TARGET_BIN="${HOME}/.local/bin/devherd"

if [[ -f "${TARGET_BIN}" ]]; then
  rm "${TARGET_BIN}"
  printf 'devherd eliminado de %s\n' "${TARGET_BIN}"
else
  printf 'devherd no esta instalado en %s\n' "${TARGET_BIN}"
fi

