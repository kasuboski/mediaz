#!/usr/bin/env bash
# Single-package mutation feedback. Exit 0=clean, 1=findings, 2=incomplete.
set -Eeuo pipefail

fail() { printf 'INCOMPLETE: %s\n' "$*" >&2; exit 2; }
trap 'fail "Unexpected runner error on line $LINENO; no quality conclusion."' ERR

if [[ $# != 1 || ! $1 =~ ^\./[a-zA-Z0-9_-]+(/[a-zA-Z0-9_-]+)*$ ]]; then
  fail 'Usage: make mutate PKG=./pkg/pagination (one explicit package; no ./...)'
fi
pkg=$1
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
cd -- "$root"
for tool in go gremlins jq timeout mktemp head tail sort; do
  command -v "$tool" >/dev/null || fail "Missing $tool; run through nix develop."
done
[[ -f go.mod && -d $pkg ]] || fail "Not a local package: $pkg"
case "$pkg/" in
  */mocks/*|*/gen/*|*/sqlcdb/*) fail 'Generated packages and mocks are not mutation targets.' ;;
esac
budget=${MUTATE_TIMEOUT:-300}
[[ $budget =~ ^[1-9][0-9]{0,4}$ ]] || fail 'MUTATE_TIMEOUT must be 1–99999 seconds.'
deadline=$((SECONDS + budget))
bounded() {
  local remaining=$((deadline - SECONDS))
  (( remaining > 0 )) || return 124
  timeout --kill-after=5s "${remaining}s" "$@"
}

# Keep build tags and other caller flags, but never reuse test-result cache.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-count=1"
# Do not let personal Gremlins config/env silently enable diff/dry-run modes,
# change operators, or set the broken v0.6.0 test-cpu flag.
for name in ${!GREMLINS_@}; do unset "$name"; done

base="$root/.artifacts/mutation/${pkg#./}"
mkdir -p -- "$base"
artifacts=$(mktemp -d "$base/run.XXXXXX")
printf 'Package: %s\nArtifacts: %s\n' "$pkg" "${artifacts#"$root/"}"
trap 'fail "Interrupted; partial artifacts: $artifacts"' INT TERM

if ! bounded go list -json "$pkg" > "$artifacts/package.json" 2> "$artifacts/baseline.log"; then
  tail -n 30 "$artifacts/baseline.log" >&2
  fail 'Package discovery failed or exceeded the time budget.'
fi
package_dir=$(jq -r .Dir "$artifacts/package.json")
[[ $package_dir == "$root/${pkg#./}" ]] || fail 'Target resolves outside the requested package/module (symlinks and nested modules are unsupported).'

printf 'Checking ordinary tests...\n'
if ! bounded go test "$pkg" >> "$artifacts/baseline.log" 2>&1; then
  tail -n 30 "$artifacts/baseline.log" >&2
  fail 'Ordinary tests failed or exceeded the time budget; mutations were not run.'
fi

# Gremlins walks descendants even for a package path. Exclude all subfolders,
# inactive build-tag/platform files, generated headers, and *.gen.go explicitly.
args=(--exclude-files '/' --exclude-files '\.gen\.go$')
for file in "$pkg"/*.go; do
  [[ -f $file ]] || continue
  name=${file##*/}
  [[ $name != *_test.go ]] || continue
  generated=false
  while IFS= read -r line; do
    [[ $line != package\ * ]] || break
    if [[ $line == '// Code generated '*' DO NOT EDIT.' ]]; then
      generated=true
      break
    fi
  done < "$file"
  if [[ $generated == true ]] ||
     ! jq -e --arg name "$name" '((.GoFiles // []) + (.CgoFiles // [])) | index($name) != null' "$artifacts/package.json" >/dev/null; then
    # RE2 literal quoting, not a shell regexp constructed from the filename.
    args+=(--exclude-files "^\Q${name}\E$")
  fi
done
printf '{}\n' > "$artifacts/config.yaml"
report="$artifacts/report.json"
printf 'Mutating %s (2 workers; total budget %ss)...\n' "$pkg" "$budget"
if ! bounded gremlins unleash "$pkg" --config "$artifacts/config.yaml" \
  --workers 2 --timeout-coefficient 20 "${args[@]}" --output "$report" \
  > "$artifacts/gremlins.log" 2>&1; then
  tail -n 30 "$artifacts/gremlins.log" >&2
  fail 'Gremlins failed or exceeded the time budget; inspect gremlins.log.'
fi
if [[ ! -s $report ]]; then
  tail -n 10 "$artifacts/gremlins.log" >&2
  fail 'No mutation report (possibly no supported candidates). No quality conclusion.'
fi
# Do not mistake an empty, malformed, dry-run, or partially understood report
# for a clean run. Count per-mutation statuses; upstream total omits uncovered.
if ! jq -e '
  (.files | type == "array" and length > 0) and
  all(.files[];
    (.file_name | type == "string") and
    (.mutations | type == "array") and
    all(.mutations[];
      (.type | type == "string") and
      (.line | type == "number" and . > 0) and
      (.column | type == "number" and . > 0) and
      (.status | IN("KILLED", "LIVED", "NOT COVERED", "TIMED OUT", "NOT VIABLE", "SKIPPED")))) and
  ([.files[].mutations[]] | length > 0)
' "$report" >/dev/null; then
  fail 'Malformed or unsupported mutation report; inspect report.json.'
fi

# One package means file_name is relative to that package. Include source
# context, but do not pretend the JSON includes an exact before/after patch.
while IFS=$'\t' read -r file line column kind; do
  printf '\nSURVIVED %s/%s:%s:%s [%s]\n' "$pkg" "$file" "$line" "$column" "$kind"
  if [[ $file != */* && -f $pkg/$file ]]; then
    printf '  Source: '
    head -n "$line" "$pkg/$file" | tail -n 1
  fi
  printf '  Tests still passed with this mutation; check inputs and assertions.\n'
done < <(jq -r '.files[] | .file_name as $f | .mutations[] | select(.status == "LIVED") | [$f,.line,.column,.type] | @tsv' "$report" | sort -t $'\t' -k1,1 -k2,2n -k3,3n -k4,4)

jq -r --arg pkg "$pkg" '
  [.files[] | .file_name as $f | .mutations[] | select(.status == "NOT COVERED") | "  \($pkg)/\($f):\(.line)"] | unique |
  if length > 0 then "\nUNCOVERED (package-local tests):", .[] else empty end
' "$report"
jq -r '
  [.files[].mutations[]] | group_by(.status) | map("\(.[0].status): \(length)") | "\n" + join(", ")
' "$report"
printf 'Full report: %s\n' "${report#"$root/"}"

if jq -e 'any(.files[].mutations[]; .status | IN("TIMED OUT", "NOT VIABLE", "SKIPPED"))' "$report" >/dev/null; then
  fail 'Timeouts/nonviable/skipped mutations are inconclusive, not assertion kills. Inspect the report.'
fi
if jq -e 'any(.files[].mutations[]; .status | IN("LIVED", "NOT COVERED"))' "$report" >/dev/null; then
  printf 'NEEDS ATTENTION: inspect findings, strengthen meaningful tests, then rerun.\n'
  exit 1
fi
printf 'CLEAN: all generated candidates were killed; this is not proof of correctness.\n'
