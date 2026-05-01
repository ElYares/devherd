#!/usr/bin/env bash
set -euo pipefail

sudo apt-get update
sudo apt-get install -y caddy

printf 'caddy instalado. Verifica con: caddy version\n'

