# Fresh sqlc migration — verification evidence

## Verified code identity and disposition

**Verified implementation commit: `8ad1e1a20d4c2eab4f0eca10d0333e1b5318deeb`**

The implementation was committed without changing the frozen, tested source bytes. This evidence is recorded in a separate documentation-only commit. The earlier revision `29f62f9a47e7860fd19462591a7d263aded9e5ad` is the pre-refactor baseline used for comparison, **not** the verified implementation revision.

Verification ran on **2026-10-26**, against the actual shared implementation after workers completed and the coordinator froze application sources. The six integrated Go gates passed at **19:21:28 UTC**. The final binary was rebuilt and smoke-tested at **19:23:04–19:23:29 UTC**, after those gates. Earlier failures and the earlier 51-check smoke were provisional and are superseded by these final results.

### Stable input fingerprints

The integrated runner computed the same SHA-256 before normal tests and after race tests:

```text
fd1bf345ab0914c8d0c01a47ffe9e7516654a16bc2d9b710e59a0ebe5345f6c8
```

The validation operator independently recomputed this fingerprint after the final rebuild/smoke and obtained the same value. Scope: all `.go` and `.sql` files and files named `go.mod`, `go.sum`, or `sqlc.yaml`, excluding `.git` and `node_modules`; sorted relative paths, each encoded as `path + NUL + file contents`, concatenated without an extra separator, then SHA-256. This includes tests/generated code but is not a hash of every repository file. The following portable Node command reproduces it from the repository root:

```sh
nix develop -c node --input-type=module <<'JS'
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
const files = [];
function walk(dir = '') {
  for (const e of fs.readdirSync(dir || '.', {withFileTypes: true})) {
    if (e.name === '.git' || e.name === 'node_modules') continue;
    const p = path.posix.join(dir, e.name);
    if (e.isDirectory()) walk(p);
    else if (e.isFile() && (/\.(go|sql)$/.test(e.name) ||
      ['go.mod', 'go.sum', 'sqlc.yaml'].includes(e.name))) files.push(p);
  }
}
walk();
const h = crypto.createHash('sha256');
for (const p of files.sort()) h.update(p).update('\0').update(fs.readFileSync(p));
console.log(h.digest('hex'));
JS
```

A second fingerprint covering Git-visible tracked/untracked implementation and configuration inputs, excluding Markdown evidence, remained identical before and after the final rebuild/frontend checks/live smoke:

```text
7b4b456b6dad949d349091261b2f99cbe9b3dc6c63c49841a9a48167284d1f3b
```

That second algorithm sorts unique `git ls-files -z --cached --others --exclude-standard` paths, excludes `.md`, and hashes for each path: `path + NUL + decimal lstat mode + NUL + contents + NUL` (symlinks use their target; deleted paths use `DELETED` in place of mode/contents). It also covers frontend/configuration inputs outside the Go gate fingerprint. These fingerprints identify the frozen inputs tested before the implementation commit. The release operator reconfirmed the first fingerprint before committing the implementation and after committing the evidence. The second fingerprint includes pre-commit deleted-path entries, so its Git-status-dependent representation changes once deletions are committed; it is retained as historical input evidence, not a post-commit tree hash.

## Toolchain

All development/build/test commands ran through `nix develop` using the repository flake/lock, on Linux amd64 with CGO enabled (`CGO_ENABLED=1`, `CC=gcc`).

| Tool | Observed version |
| --- | --- |
| Nix | 2.34.6 |
| Go | go1.26.10 linux/amd64 (`go.mod`: 1.26.0) |
| sqlc | v1.31.1 |
| Node.js | v24.21.0 |
| npm | 11.19.0 |
| SQLite CLI | 3.53.3, 64-bit, build date 2026-06-26 |
| TypeScript (frontend lockfile install) | 5.6.3 |
| Vite | 5.4.10 |
| Biome | 1.9.4 |
| Nix-installed mockgen | v0.6.0 |
| Module-pinned mockgen used by `go generate` | `go.uber.org/mock` v0.4.0 |

The installed mockgen binary and module-pinned generator are deliberately distinguished: generation below uses the project's `go:generate go run go.uber.org/mock/mockgen ...` directives, not an inferred invocation of the Nix binary.

## Integrated final Go gates

The coordinator authorized formatting/tidy after worker completion at 19:21 UTC. All commands below exited **0**, in this order, on the actual shared tree:

| Exact command | UTC start–end | Result |
| --- | --- | --- |
| `nix develop -c go fmt ./...` | 19:21:06–19:21:06 | PASS; whitespace-only changes in `config/config.go` and `pkg/storage/sqlite/movie_metadata.go` |
| `nix develop -c go vet ./...` | 19:21:06–19:21:07 | PASS |
| `nix develop -c go build ./...` | 19:21:07–19:21:10 | PASS |
| `nix develop -c go mod tidy` | 19:21:10–19:21:10 | PASS; **no go.mod/go.sum changes** |
| `nix develop -c go test -count=1 -json ./...` | 19:21:10–19:21:19 | PASS, uncached |
| `nix develop -c go test -race -count=1 ./...` | 19:21:19–19:21:28 | PASS, uncached; no race reports |

Normal-suite counts: **378 top-level tests + 803 subtests = 1,181 passing test/subtest events**; zero failing or skipped test cases. **15 tested packages passed**, and **15 packages had no test files** (their package-level JSON skip events are not skipped test cases). Race suite: 15 packages passed, 15 had no test files, zero failures. No source/module/SQL inputs changed during the test gates.

Passing normal test/subtest events by package, with `github.com/kasuboski/mediaz/` omitted:

| Package | Passing events | Package | Passing events |
| --- | ---: | --- | ---: |
| config | 4 | pkg/cache | 9 |
| pkg/download | 41 | pkg/http | 11 |
| pkg/indexer | 24 | pkg/io | 5 |
| pkg/library | 98 | pkg/logger | 4 |
| pkg/machine | 3 | pkg/manager | 660 |
| pkg/prowlarr | 2 | pkg/ptr | 2 |
| pkg/storage/sqlite | 162 | pkg/tmdb | 13 |
| server | 143 | | |

## Generation reproducibility

CORE performed an initial settling generation and **two repeat generations** after the final schema/down-migration changes. Each round executed exactly:

```sh
nix develop -c sqlc generate
nix develop -c go generate ./pkg/storage ./pkg/download ./pkg/indexer
fd -e go . pkg/storage/sqlcdb pkg/storage/mocks pkg/download/mocks pkg/indexer/mocks \
  | sort | xargs sha256sum > /tmp/mediaz-generation-A.sha256
# Repeat rounds write B.sha256 and C.sha256 instead.
diff -u /tmp/mediaz-generation-A.sha256 /tmp/mediaz-generation-B.sha256
diff -u /tmp/mediaz-generation-B.sha256 /tmp/mediaz-generation-C.sha256
```

Both diffs exited **0**, with no output. Each manifest contains **24 generated Go files** (20 sqlc files and four mock files). All three manifest files had SHA-256:

```text
2ca695db428e0434163e00028a49e9a1b3ca1fa5f7b32f5e8821a72e52c921e2
```

The validation operator checked the final manifest against the actual files with `sha256sum -c /tmp/mediaz-generation-C.sha256`: **all 24 matched**. This is generation of the relevant persistence/mocks, not a claim that external TMDB/Prowlarr OpenAPI schemas were fetched or unrelated clients regenerated.

Final schema hashes:

```text
af04eac000bb4cab5e871afc8e12bb4f9a0fa55dea376b20361af59f287d864d  pkg/storage/sqlite/migrations/000001_baseline.up.sql
7c773a05a9d6a6a2defa193051cd1c1c51c7fdbc4fb98b47abe8452bf77ee44e  pkg/storage/sqlite/migrations/000001_baseline.down.sql
```

For portable verification, the complete final generated-file manifest is retained here rather than only in temporary logs:

```text
52645c1985a4e4b63a11dfcc877912f941cde87400c82c69f98dbf3f753d4e20  pkg/download/mocks/mock_download_client.go
751d71a5f41f98fe2589df0337abf06b2d86fe639f13ac9d470377db113224dd  pkg/download/mocks/mock_factory.go
d19cbaea67c8c4de2f0dda0286c837005361bc84a06cb3dfd90995496c619f2b  pkg/indexer/mocks/mock_indexer_source.go
8910aa023273920c5436c2e26e580a44dc4ea794dcb202e1c74bb22aca482940  pkg/storage/mocks/mock_storage.go
34c13cf88f184b1b8af69fd03dc8510c09f38a854958879ef82734499f0044f5  pkg/storage/sqlcdb/activity.sql.go
f2feb34e76e0d382d7d66666a076ce229cabf03b8b64a753fd146f1a0e22bb3f  pkg/storage/sqlcdb/db.go
e78c6e6a4cfc404f50b2fe02966d3d1dbeb860f2e4394514823d5c06f29414f9  pkg/storage/sqlcdb/download_client.sql.go
358547188bbbddbe51e564c05851ddd15c2c61a0c58e36913b58521f1d41b89b  pkg/storage/sqlcdb/episode_file.sql.go
d2137e8cc552462217f369e8ceb069965a6599647566f6b342f68d3a787086fe  pkg/storage/sqlcdb/episode.sql.go
d29bfc240d8192c03ec328b198778c5237f21f63cfd6446f512df34c51734be9  pkg/storage/sqlcdb/indexer_source.sql.go
bf91c90a9af3aea30a3dc771677ec0aa1cd4d36d8b533df9a52a886b9aa460d6  pkg/storage/sqlcdb/indexer.sql.go
4a5ad9b0d12231d67c077e27a6ef8a59f70e30c126165cf95d75d99ffaa66e68  pkg/storage/sqlcdb/job.sql.go
4bfb0099ada626a1a5ab7b12c2f9935fccf6bddbfa84a6f639cb6251317989e8  pkg/storage/sqlcdb/models.go
70915fdf01fef2631c9fc3986bf276f3e4bb74c617a748a2ad088c0b159f9e2c  pkg/storage/sqlcdb/movie_file.sql.go
95f596fc33ddb30fa43a7d8a8ccb4e812cfa083cb7f121fe7103c7e637516d3d  pkg/storage/sqlcdb/movie_metadata.sql.go
afa4511889a6a3a1baf164c5de7f18368f292d98c90e0a4027a4f9cfb7a35bcf  pkg/storage/sqlcdb/movie.sql.go
8571f84f98bd4840c1fda66ea0629f3166741990883eda60545eea621a8583f7  pkg/storage/sqlcdb/quality_definition.sql.go
6cac3090f8bc9b3baee8a29a5bcbfb99d75ab7fe6a0303caf6ef8a58cda6e37b  pkg/storage/sqlcdb/quality_profile_item.sql.go
4ae332128c0ba9c0ea2d5a01bc37d98c4bb88a7b0bfb2fa413b5794ef6defc62  pkg/storage/sqlcdb/quality_profile.sql.go
03d44cf4b873e0a2319c5b9db2fbf5761f76a8cb2443ac27e579c36d7acfc319  pkg/storage/sqlcdb/querier.go
d1f14197edc8a2e00d4b44eb4f7c72b959660f0ac3740d0398850610f1e9c2f5  pkg/storage/sqlcdb/season.sql.go
95c2bc5c624f8409a4929a302948c5cd9ce11f2e0c844b7da11a8cd76c907890  pkg/storage/sqlcdb/series_metadata.sql.go
6b0030ffea59f6bcbe21e9f241d8d00742bc5250c743b68415db9e3f73995ce5  pkg/storage/sqlcdb/series.sql.go
3aebd86e3ee4c1ec98d925a6ab426b6704ebaace33ca0709a1583b6851b5ba4b  pkg/storage/sqlcdb/stats.sql.go
```

## Real SQLite and behavioral coverage

These are behavioral tests against SQLite, not persistence mocks alone, and are included in the passing full normal/race suites:

- `pkg/storage/sqlite/bootstrap_test.go`: fresh **file-backed** DB creation, repeated initialization, seed idempotence, `:memory:` initialization/pooling, seeding after an already-applied baseline, FK enforcement, and actionable refusal of old-versioned/pre-migration databases without deleting the database/media. The concurrent seed case starts after migration: it does **not** prove simultaneous first-ever migration safety or arbitrary partial-seed repair.
- `fresh_acceptance_test.go`: FKs on every pooled connection; cascade/SET NULL behavior and unchanged fake-media bytes; atomic transition rollback and create/job rollback; concurrent pending-job deduplication/release; deterministic job pagination/filter/count parity and safe bulk deletion; state filters exceeding four values; UTC timestamps/defaults; atomic indexer-source cascade rollback.
- `relational_invariants_test.go`: direct generated `CreateJobTransition` calls reject a second latest transition and duplicate `(job_id, sort_key)`, independently of pending-type deduplication; historical/different-parent keys remain legal. Mandatory movie/series profiles are `NOT NULL` FKs with `DELETE RESTRICT`; optional metadata/file links use `SET NULL`; metadata ancestry remains restrictive. Dangling references fail/roll back, and profile pre-delete cleanup rolls back on refusal.
- `core_invariants_test.go`, `metadata_api_test.go`, and `activity_test.go`: locked-current-state concurrent transitions, independently nullable download metadata, distinct local/TMDB/sequence IDs, public season/episode metadata create/get/list/delete/notfound/empty-ID behavior, native activity rows, and stable ordering.
- `TestFreshAcceptanceUTCTimestampsAndDefaults` compares actual instants against UTC bounds and verifies zero UTC offsets for movie/series/job/transition/metadata/file timestamps. The independent storage reviewer also ran `TZ=Pacific/Auckland go test -count=1 ./pkg/storage/...` successfully and repeated targeted relational/job/public-metadata tests with `-count=10` successfully.

Storage review independently verified **22 application tables**, **141 matching query-source/generated named queries**, and preparation of all **141 generated SQL constants** with SQLite `EXPLAIN` against the actual baseline. No persistence mirror/legacy converter/model-alias layer, ORM, or raw application SQL outside allowed migration/seed mechanics remained in the reviewed scope. Storage deletion APIs do not mutate media files.

## HTTP contract and deliberate deltas

`server/wire_contract_test.go` passes normally and under race detection using real SQLite, manager, and router captures. **61 golden JSON fixtures** in `server/testdata/wire/` preserve field casing, nesting, null/omitted distinctions, arrays and deterministic ordering. Guards reject exposed `sql.Null*` objects and stored/submitted credentials; newly submitted secrets are registered before create/update captures. Library mutation responses retain flat historical PascalCase fields plus lowercase `state`; download IDs/client IDs remain internal.

The accepted targeted deltas, documented in `server/testdata/wire/README.md`, are:

1. Download-client `APIKey` is omitted entirely: stored credentials are **write-only**. Indexer/source responses already omitted them.
2. Canonical seeded profile cutoffs are restored: profiles 1–6 use 2, 8, 13, 16, 23, 27. Upgrades are enabled for 1, 2, 4, 5 and disabled for 3, 6; this corrects old seed drift, not JSON representation.
3. Duplicate seeded quality definitions 29–56 are removed. There are 28 canonical definitions; the custom-create fixture ID changes 57→29. Custom update `/29` now genuinely updates the newly created row (QualityID 0), rather than the first duplicate seed.
4. Job ties use `(created_at DESC, id DESC)`: same-second fixture IDs are 3, 2, 1 (first page 3, 2).
5. Active `stateSince` uses actual transition `created_at`, not `time.Unix(sort_key, 0)`; duration follows the real instant. A raw recency assertion prevents timestamp normalization from concealing this bug.
6. Subsequent transitions retain the actual `from_state`; initial transitions still omit it. Same-time timeline ties use entity type ascending, then transition ID descending. Entities, counts, field names/nesting, and chronological direction are retained.

These are bounded intentional corrections, **not** blanket golden regeneration. Independent final review found no unexplained remaining golden mismatch or material scenario-coverage loss.

## Independent reviewer findings resolved

The final storage and manager/HTTP reviewers reported **zero remaining blockers in their respective scopes**, with independent normal/race package passes. Their earlier failing snapshots were superseded, not presented as final evidence.

| Finding | Verified resolution |
| --- | --- |
| Missing latest-job/sort-key DB constraints | Partial unique latest-per-job and unique `(job_id, sort_key)` indexes in up migration; explicit down drops; malformed direct generated inserts rejected |
| Missing relational integrity | Mandatory profile RESTRICT, optional cache/file SET NULL, restrictive metadata ancestry; real FK/rollback tests, without weakening FK enforcement |
| Discovery inserted profile 0 / hid persistence failures | Select existing eligible movie/TV profiles; reject absence; propagate underlying errors; FK-enabled discovery/idempotence and failure-policy tests |
| Indexer/TV identity narrowing | Internal IDs remain int64; genuine external SDK conversion is range-checked at the edge; tests cover local `(1<<32)+1` and source `1<<40` without wraparound |
| Activity time derived from sort key | Actual transition timestamp used, independently tested before normalization |
| Secret registration / nullable JSON leaks | Input secrets registered before capture, write-only credential responses, scalar/null HTTP DTO conversion |
| Legacy test conversions / coverage debt | Full manager/server suites pass; retained reconciliation/scheduler/TV scenarios reviewed; public metadata API coverage restored |

## Final rebuilt binary / live API smoke

**Result: 51 HTTP status checks passed; zero smoke failures; harness exit 0.** This is a fresh post-gate run, not reuse of the provisional binary. Binary build:

```sh
nix develop --command go build -o /tmp/mediaz-validation/mediaz-final .
sha256sum /tmp/mediaz-validation/mediaz-final
```

Build exited 0. Tested binary SHA-256:

```text
ad5393fd4b2eb80210a03ffac39c2582c048ea1b915ea0e771197f788a0431c1
```

The harness started the real CLI as:

```sh
/tmp/mediaz-validation/mediaz-final --config /tmp/mediaz-live-go82iD/config.json serve
```

Exact harness invocation (the paths identify this run's artifacts, not repository prerequisites):

```sh
LIVE_BINARY=/tmp/mediaz-validation/mediaz-final \
LIVE_DIST=/tmp/mediaz-validation/frontend-final/dist \
LIVE_EVIDENCE=/tmp/mediaz-final-live-smoke.md \
nix develop --command node /tmp/mediaz-validation/live-smoke.mjs
```

Portable scenario specification: create a fresh temporary SQLite DB, empty movie/TV directories, and a text sentinel under the movie directory; serve the freshly built frontend distribution. Bind the API and a local TMDB stub on available loopback ports. Configure `tmdb.uri` to that stub, fake `apiKey: mock-only`, retries 0; use one-hour movie/TV index/reconcile/indexer-sync intervals and a one-second job scheduler interval. The stub serves movie 900001 as `Smoke Mock Movie`, release date `2099-01-01`, runtime 90, empty genres/companies/languages and zero vote counts; other discovery responses are empty. No real assets, external services, or credentials are involved.

| Live API operation | Observed status/result |
| --- | --- |
| `GET /healthz` | 200, `{"response":"ok"}` |
| GET library movies/TV, stats | 200; initially empty library and totals 0 |
| GET activity active/failures/timeline | 200; valid empty initial responses |
| `GET /api/v1/config` | 200; configured temporary paths and intervals (`/config` supports GET, not global config CRUD) |
| Quality definition create/get/update/delete | 201/200/200/200; custom definition ID 29, updated name/sizes persisted |
| Quality profile create/get/update/delete | 201/200/200/200; custom profile with definition 29 |
| Indexer create/list/update/delete | 201/200/200/200; loopback-only endpoint |
| Disabled indexer source create/get/update/delete | 201/200/200/200; no external refresh |
| Download client create/get/update/delete | 201/200/200/200; loopback-only endpoint, credentials absent from responses |
| Job list/create/get/cancel | 200/201/200/200; manually created MovieIndex cancelled while pending |
| Second MovieIndex create/poll | 201 then 200; completed as `done` |
| Scheduler's five job types | MovieIndex, MovieReconcile, SeriesIndex, SeriesReconcile, IndexerSync all reached `done` |
| Mock movie add/list/unmonitor/history/delete | All 200; `unreleased`, Monitored 1→0, history visible, `deleteFiles=false` |
| Final stats/timeline | 200; library totals return to 0; five completed jobs represented |

The 51 count includes job polling requests; it is not a count of distinct routes or exhaustive response-field assertions. Full golden tests above supply the stronger wire-shape assertions.

Direct database evidence used:

```sql
SELECT j.id,j.type,t.from_state,t.to_state,t.sort_key,t.most_recent
FROM job j JOIN job_transition t ON t.job_id=j.id
ORDER BY j.id,t.sort_key;
```

SQLite returned, for job 1, `pending` (sort 1, not latest) then `pending→cancelled` (sort 2, latest). Jobs 2–6 each had `pending` (sort 1), `pending→running` (sort 2), `running→done` (sort 3, latest). Exactly one latest row was present per job. The sentinel bytes were unchanged. The harness stopped the temporary server on completion; no real download/import/seeding or frontend browser rendering was exercised.

## Frontend — passing build/lint and honest baseline exceptions

No tracked frontend files were edited by the validation operator. Final checks ran against a temporary copy of `frontend`, excluding `node_modules`, `dist`, and `*.tsbuildinfo`; baseline checks used a separate extraction of the **pre-refactor revision `29f62f9a47e7860fd19462591a7d263aded9e5ad`**, without checkout or worktree changes:

```sh
mkdir -p /tmp/mediaz-validation/frontend-head
git archive 29f62f9a47e7860fd19462591a7d263aded9e5ad frontend | tar -C /tmp/mediaz-validation/frontend-head --strip-components=1 -xf -
# In each isolated baseline/final copy, using nix develop:
npm ci
npm run build
npm test
npx tsc -b --pretty false
# Final copy only; never run the write-enabled lint script against the checkout:
npm run lint
```

| Check | Final outcome | Baseline/comparison |
| --- | --- | --- |
| `npm ci` | Exit 0; 401 packages installed, 402 audited | Same baseline package/audit counts |
| `npm run build` | Exit 0; 2,882 modules; JS 989.82 kB / gzip 275.80 kB | Baseline also passes |
| `npm run lint` | Exit 0; Biome says **7 files checked, no fixes applied** | Hash comparison before/after final-copy lint identical; this is configured coverage, not a claim all TS source was linted |
| `npm test` | Exit 1: **no test script exists** | Same at the pre-refactor revision; no frontend-test pass is claimed |
| `npx tsc -b --pretty false` | Exit 2: **14 errors** | Final diagnostics byte-identical to the pre-refactor revision (`diff -u` exit 0), also identical to the provisional copy |
| npm audit summary during install | **26 vulnerabilities: 2 low, 7 moderate, 17 high** | Same counts at the pre-refactor revision; no audit remediation/pass is claimed |

The 14 existing TypeScript diagnostics are: two `Record<string, number>` constraints in `src/config/media-filter-configs.ts` (44, 71); two functional-setter-vs-number errors in `src/hooks/use-pagination.ts` (61, 65); two unsupported Lucide `title` props in `src/pages/LibraryPage.tsx` (68, 80); four missing `MovieDetail.id` errors in `src/pages/MovieDetail.tsx` (59, 63, 75, 79); four missing `TVDetail.id` errors in `src/pages/TVDetail.tsx` (151, 155, 167, 171). Vite transpilation success is **not** a TypeScript or frontend-test pass.

Build warnings remain: stale Browserslist data and a minified chunk above 500 kB. npm reports four install-script approval warnings (Biome, SWC, two esbuild versions). These were not hidden or automatically fixed. Final frontend source hashes were identical before/after lint; `git diff --exit-code -- frontend` exited 0.

## Breaking database reset caveat

This release uses **one squashed fresh baseline** and a single idempotent seed path. Old databases (legacy migration versions 1–7 and pre-migration databases) are **not supported/upgraded automatically**. Startup refuses versions above the new baseline and nonempty databases without migration history with actionable path/reset instructions. An old version-1 DB cannot be distinguished by version alone: **operators must reset every database from an earlier release even if startup does not refuse it**.

Follow `README.md` → “Database Reset (Breaking Change)”: stop mediaz, back up as needed, manually remove the configured DB and its matching `-wal`/`-shm` sidecars, then restart. Do not restore old sidecars. Reconfigure database-only settings/history (indexers, clients, monitoring choices, custom profiles, etc.). Media library files are not reset/deleted; mediaz never deletes the database file automatically. Removed `storage.schemas` / `storage.tableValueSchemas` options are not alternative schema inputs.

## Audit trail and limits

The results, counts, hashes, commands, policies and exceptions above are portable evidence. Supplemental local artifacts from this run (not required to understand the disposition):

- `/tmp/mediaz-final-verification.md`, `/tmp/mediaz-final-test.json`, `/tmp/mediaz-final-race.log`: runner transcript, raw test events and race output.
- `/tmp/mediaz-generation-proof.md`, `/tmp/mediaz-generation-{A,B,C}.sha256`: CORE generation transcript/manifests.
- `/tmp/mediaz-storage-final-review.md`, `/tmp/mediaz-manager-http-final-review.md`: independent reviews.
- `/tmp/mediaz-final-live-smoke.md`: all 51 exact requests/statuses/responses and direct DB rows.
- `/tmp/mediaz-validation/`: final binary, tool logs, frontend copies/logs, source fingerprints, readiness snapshot, harness and checksum comparisons.
- `/tmp/mediaz-live-go82iD/`: final temporary config/database/server/mock logs; temporary server stopped.

**Disposition:** integrated Go gates, repeat generation, final rebuilt live smoke and frontend install/build/configured lint pass. Frontend typecheck, missing frontend test script, dependency warnings, and database-reset requirements remain explicit baseline exceptions/caveats. The implementation identity above binds these pre-commit checks to the unchanged committed source; the following evidence commit changes only this document. No claim of real external asset/download/seeding integration, browser rendering, simultaneous first-ever migration safety, or arbitrary seed repair is made. Remote CI results are recorded separately on PR #183 for its final pushed head, rather than inferred from these local checks.
