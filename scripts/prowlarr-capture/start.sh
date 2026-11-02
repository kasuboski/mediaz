#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
umask 077

# Loopback UI port; override with PROWLARR_PORT=<port> before running this script.
PROWLARR_PORT="${PROWLARR_PORT:-9696}"
export LOCAL_UID="$(id -u)" LOCAL_GID="$(id -g)" PROWLARR_PORT

fail() { printf 'error: %s\n' "$1" >&2; exit 1; }

# --- Prerequisites (fail fast with actionable messages) ----------------------
command -v docker >/dev/null 2>&1 || fail 'docker is required (https://docs.docker.com/get-docker/)'
docker compose version >/dev/null 2>&1 || fail 'the docker compose plugin (v2) is required'
command -v node >/dev/null 2>&1 || fail 'node (>= 18, with built-in fetch) is required for configure-proxy.mjs'
node -e 'if (typeof fetch !== "function" || typeof AbortSignal.timeout !== "function") process.exit(1)' >/dev/null 2>&1 || fail 'node >= 18 with fetch and AbortSignal.timeout is required'
node -e 'const p=Number(process.env.PROWLARR_PORT); if(!Number.isInteger(p)||p<1||p>65535)process.exit(1)' || fail 'PROWLARR_PORT must be a valid TCP port'
command -v curl >/dev/null 2>&1 || fail 'curl is required for readiness checks'
docker compose config --quiet || fail 'compose.yaml failed validation; check docker compose config output above'

# --- Private state directories ----------------------------------------------
mkdir -p .state/mitm .state/captures .state/prowlarr .state/export
chmod 700 .state .state/mitm .state/captures .state/prowlarr .state/export

# --- Bounded readiness waits -------------------------------------------------
# wait_for <description> <timeout_seconds> <command...>
wait_for() {
  local desc="$1" timeout="$2" deadline
  deadline=$((SECONDS + timeout))
  shift 2
  until "$@"; do
    (( SECONDS < deadline )) || fail "$desc was not ready within ${timeout}s"
    sleep 1
  done
}

ca_ready() { [[ -s .state/mitm/mitmproxy-ca-cert.pem ]]; }

prowlarr_ready() {
  curl --fail --silent --output /dev/null \
    --connect-timeout 2 --max-time 5 "http://127.0.0.1:${PROWLARR_PORT}/ping"
}

# --- Start -------------------------------------------------------------------
docker compose up -d mitmproxy
wait_for 'Recorder CA generation (diagnose with: docker compose logs mitmproxy)' 60 ca_ready

docker compose up -d prowlarr
wait_for "Prowlarr UI on 127.0.0.1:${PROWLARR_PORT} (diagnose with: docker compose logs prowlarr)" 120 prowlarr_ready

node configure-proxy.mjs
printf '\nProwlarr ready at http://127.0.0.1:%s\n' "$PROWLARR_PORT"
printf 'Raw private recordings: %s/.state/captures/\n' "$PWD"
