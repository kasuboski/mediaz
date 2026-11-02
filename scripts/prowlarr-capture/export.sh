#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
umask 077
# No network, CA keys, Prowlarr database, or host Python environment needed.
mkdir -p .state/captures .state/export
chmod 700 .state .state/captures .state/export
image='mitmproxy/mitmproxy:12.2.1@sha256:743b6cdc817211d64bc269f5defacca8d14e76e647fc474e5c7244dbcb645141'
args=(python /tool/export.py "$@")
if [[ "${1:-}" == '--test' ]]; then
  [[ $# == 1 ]] || { echo '--test takes no additional arguments' >&2; exit 1; }
  args=(python -m unittest discover -s /tool -p 'test_*.py' -v)
fi
docker run --rm --network none --read-only \
  --user "$(id -u):$(id -g)" --env PYTHONDONTWRITEBYTECODE=1 \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --mount "type=bind,src=$PWD/export.py,dst=/tool/export.py,readonly" \
  --mount "type=bind,src=$PWD/sanitize.py,dst=/tool/sanitize.py,readonly" \
  --mount "type=bind,src=$PWD/test_sanitize.py,dst=/tool/test_sanitize.py,readonly" \
  --mount "type=bind,src=$PWD/.state/captures,dst=/captures,readonly" \
  --mount "type=bind,src=$PWD/.state/export,dst=/fixtures" \
  --entrypoint /usr/local/bin/python "$image" "${args[@]:1}"
