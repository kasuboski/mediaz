#!/usr/bin/env bash
# Scan the index snapshot only: never recursively scan the lab or raw captures.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
umask 077
mkdir -p .state
chmod 700 .state
state="$PWD/.state"
root="$(git rev-parse --show-toplevel)"
snapshot="$(mktemp -d "$state/scan-staged.XXXXXX")"
trap 'rm -rf -- "$snapshot"' EXIT
cd -- "$root"
git diff --cached --name-only --diff-filter=ACMRT -z |
  git checkout-index --stdin -z --prefix="$snapshot/"

# Pin the scanner; do not verify credentials against live services. Docker's
# network isolation is a second guard against accidental outbound verification.
image='trufflesecurity/trufflehog:3.99.7@sha256:0d003fc3abc4e1565eca8462b992001314de51451e96e3c44de80d116794d5d1'
set +e
docker run --rm --network none \
  --mount "type=bind,src=$snapshot,dst=/scan,readonly" "$image" \
  filesystem --no-update --no-verification --no-color --json \
  --results=verified,unknown,unverified,filtered_unverified \
  --fail --fail-on-scan-errors /scan \
  >"$state/trufflehog-results.jsonl" 2>"$state/trufflehog-log.txt"
status=$?
set -e
node --input-type=module - "$state/trufflehog-results.jsonl" <<'JS'
import { readFile } from 'node:fs/promises';
const lines=(await readFile(process.argv[2],'utf8')).split('\n').filter(Boolean);
const counts={};
for (const line of lines) {
  let item;
  try { item=JSON.parse(line); } catch { console.error('Scanner output could not be parsed; inspect private logs.'); process.exit(1); }
  const name=item.DetectorName ?? 'unknown';
  counts[name]=(counts[name] ?? 0)+1;
}
console.log(JSON.stringify({findings:lines.length,detectors:counts}));
JS
if (( status != 0 )); then
  echo 'Secret scan failed or found candidates. Inspect private .state/trufflehog-* files; never publish their raw contents.' >&2
  exit "$status"
fi
echo 'TruffleHog 3.99.7: staged snapshot clean; credential verification and container networking disabled.'
