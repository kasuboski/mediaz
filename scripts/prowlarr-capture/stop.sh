#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"

# Stops the recording stack, preserving all configuration and recordings in .state/.
# Never touches Tailscale Serve or any services outside this Compose project;
# removing an optional Serve route is manual (see README).

command -v docker >/dev/null 2>&1 || { echo 'error: docker is required' >&2; exit 1; }
docker compose stop
echo 'Recording stack stopped; .state/ preserved. Re-run start.sh to resume.'
