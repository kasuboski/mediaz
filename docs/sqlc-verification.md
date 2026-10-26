# Fresh sqlc migration — verification evidence

## Final TV discovery error-contract closure — 2026-10-26

**Verified implementation: `6bccb66652cce45c91a7c2fb485d07b687612550`**, based on `d1fdbcc29e283f37610890f1bf6092240df8b5f2`. This section supersedes earlier current-source counts/fingerprints and corrects the earlier blanket claim that TV discovery errors already propagated. The accompanying evidence commit changes documentation only; exact pushed-head CI is recorded separately on PR #183.

### Classification and bounded fix

The late discovery audit identified a **real false-success bug**, not an intentional successful-job policy. `New` registers `SeriesIndex` to return `IndexSeriesLibrary` directly; `Scheduler.executeJob` persists `done` for nil and `error` for a returned error. The scanner logged file/profile/series/season/episode failures but returned nil. TV reconciliation already aggregates independent failures with `errors.Join`.

The minimal fix preserves best-effort scan continuation and earlier successful writes, but returns contextual joined errors. It also checks the actual episode-file association lookup error rather than the unrelated outer error. A failed refreshed-file-list lookup retains any preceding write failures. No schema, generated rows, FK/profile selection, HTTP shapes, job state machine, frontend or unrelated orchestration changes.

- **Executed red run before the fix:** `nix develop -c go test -count=1 ./pkg/manager -run 'TestSeriesDiscovery'` exited 1. Missing-TV-profile job was **done**, expected **error**; injected persistence failures returned nil; episode lookup failure incorrectly proceeded into series lookup. The same cases pass after the fix.
- `TestSeriesDiscoveryMissingProfileIsPublicError`: fresh FK-enabled SQLite, remove only TV profiles while retaining movie profiles; run the actual registered executor through the scheduler, verify persisted error state/diagnostic, no series/season/episode rows, and retained successful file write.
- `TestSeriesDiscoveryPropagatesPersistenceErrors`: public method, seven strict gomock cases (file write, episode lookup, series lookup, profile lookup, series/season/episode create); `errors.Is` preserves injected identities and strict expectations prohibit downstream writes after failed lookups.
- `TestSeriesDiscoveryContinuesAfterPersistenceFailure`: four real SQLite trigger faults at file/series/season/episode insert. Independent healthy file still produces complete expected native series/season/episode/file rows with a valid TV profile; a repeated scan reports the remaining failure and preserves the entire snapshot without duplicating successful rows.
- `TestSeriesDiscoveryJoinsFailuresBeforeRefreshError`: both write errors and later refresh error remain discoverable via `errors.Is`, with file path context.
- Independent GPT6.1-sol focused read-only review: **zero blockers**; independently reran all four new test functions uncached. It confirmed the actual executor/job regression, stale-error fix, continuation and retained-success policy; it did not claim to independently repeat the broader gates.

**Explicit limitations:** scan-wide rollback/transactionality, filesystem compensation, arbitrary repair and universal retry/idempotence are not promised. Earlier committed file/parent records may remain after a later failure; independent files continue. Existing empty/unparseable series-name warn/skip behavior is unchanged. Joined diagnostics reach job storage/API through the existing error path; no job/API redesign. Earlier movie discovery and indexer int64 fixes are unchanged.

### Frozen-source verification

Executed **20:28:31–20:29:11 UTC**, locked Nix **Go 1.26.10 / sqlc 1.31.1**, after source/test edits froze:

```sh
nix develop -c go fmt ./...
nix develop -c go vet ./...
nix develop -c go build ./...
nix develop -c go mod tidy
nix develop -c go test -count=1 -json ./...
nix develop -c go test -race -count=1 ./...
nix develop -c go test -count=1 ./pkg/manager -run 'SeriesDiscovery|IndexSeriesLibrary|IndexListEpisodeFiles'
nix develop -c go test -count=1 ./server -run 'Wire|HTTPDatabase'
# Two rounds; compare both to each other and to the prior cutoff-release manifest:
nix develop -c sqlc generate
nix develop -c go generate ./pkg/storage ./pkg/download ./pkg/indexer ./pkg/library
nix develop -c go build -o /tmp/mediaz-series-discovery/mediaz .
LIVE_BINARY=/tmp/mediaz-series-discovery/mediaz \
LIVE_DIST=/tmp/mediaz-followup/frontend/dist \
LIVE_EVIDENCE=/tmp/mediaz-series-discovery/live-smoke.md \
nix develop -c node /tmp/mediaz-validation/live-smoke.mjs
```

All PASS. **407 top-level + 855 subtests = 1,262 passing events**, 15 tested packages, zero failing/skipped tests; full race has no reports. All **61 unchanged wire fixtures** rerun. Formatting settled only the new test file before fingerprinting; tidy leaves `go.mod`/`go.sum` unchanged; `git diff --check` passes. Both generation rounds preserve all **25 generated files**, byte-identical to the prior release; sorted manifest SHA-256 remains **`d46315584c8366603c6ba5331fb8474d50c9a999cebd9ad62acdce81cdab14c3`**.

Portable implementation fingerprint (algorithm below): **`96e25c84286d9f16f1621b21320026a041e8a656c32f621cd6537eaac68a61db`**, identical before gates, after generation/race and after rebuilt smoke. Binary SHA-256: **`568b50f7810243f95b21573d1a18c8bc4714fdee86774264ae65f43b38a91a10`**. Smoke started **20:28:59 UTC** and completed before **20:29:11 UTC**: **51 HTTP checks, zero failures**, all five scheduler types done with proper latest/history, temporary sentinel unchanged, server stopped; fresh isolated DB/config `/tmp/mediaz-live-MMUDnj/`. The empty-library smoke checks general compatibility; the new regressions prove failure reporting.

Frontend source unchanged: reuse the previously verified identical-source dist; no needless new install/build/typecheck/test claim. **14 existing TypeScript errors, 26 vulnerabilities, no test script** remain explicit exceptions. Fresh-start reset requirements, optional-link SET NULL, bootstrap concurrency/repair limits, and preexisting non-atomic multi-step profile/item behavior remain unchanged. No actual user DB/media deletion, force-push or merge; human approval required.

Supplemental artifacts: `/tmp/mediaz-series-discovery/{before.log,gates.log,test.json,counts.json,race.log,targeted.log,wire.log,generated-A.sha256,generated-B.sha256,fingerprint-before,fingerprint-after,fingerprint-after-smoke,binary.sha256,live-smoke.md}`. Commands/scenarios/counts/hashes above are durable evidence, not dependent on access to those temporary paths.

---

## Final cutoff FK hardening — 2026-10-26

**Verified implementation: `52de428e6f21eeba514eeff5f3411dc3171aa487`**, parent `cf7c88af548c0762afb34697ce837c30f383f1b3`. This section supersedes earlier counts/fingerprints for the current implementation; the accompanying evidence commit changes documentation only. Exact final pushed-head test/Docker CI is recorded on PR #183, not inferred from old green runs.

### Minimal policy and regression evidence

The sole production change is nullable `quality_profile.cutoff_quality_id REFERENCES quality_definition(id) ON DELETE RESTRICT` in the canonical fresh-start baseline. Cutoffs select **local definition primary keys**, not the non-unique quality-domain enum (`quality_definition.quality_id`). Existing defaults seed definitions before profiles with valid cutoff IDs; all existing fixtures remain valid without edits. Generated Go, queries, manager behavior, profile items, state features and HTTP shapes are unchanged.

- `TestQualityProfileCutoffReferencesDefinitionID`: real SQLite rejects nonexistent cutoff create/update; rejected create leaves the complete profile list unchanged and rejected update preserves the complete profile. A custom definition with a deliberately different enum proves the target is its primary key. Deleting a definition referenced **only by a cutoff** is refused and preserves the full definition. Clearing the cutoff permits deletion; NULL create/update remain legal and preserve profiles.
- `TestQualityServiceMissingCutoffDefinitionPreservesProfilesAndItems/{add,update}`: missing cutoff included among selections passes membership validation but fails at the profile write, preserving full profile/item snapshots. No manager redesign is needed for this early-rejection case.
- **Executed negative control:** copied these tests into a separate detached worktree at `cf7c88a` and ran `nix develop -c go test -count=1 ./pkg/storage/sqlite ./pkg/manager -run 'TestQualityProfileCutoffReferencesDefinitionID|TestQualityServiceMissingCutoffDefinitionPreservesProfilesAndItems'`. Exit **1**: direct invalid create returned nil error, manager failed add retained a dangling profile, and failed update changed profile/items. The same tests pass with the FK.
- Independent GPT reviewer found **zero blockers**, confirmed definition-ID/null/RESTRICT semantics and seed/HTTP/state compatibility, and independently ran uncached full SQLite/manager/server tests successfully. No competing fixture writes.

Existing policies remain unchanged: mandatory movie/series profiles RESTRICT; optional metadata/file pointers **SET NULL**; owned library subtrees/history CASCADE; metadata ancestry restrictive. SET NULL is the explicit chosen optional-link policy, superseding older audit wording suggesting only refusal/cascade. Retained metadata/file rows are intentional shared-cache/file lifecycle, not dangling parent pointers. Job latest/sort-key indexes and SQLite column-name UNIQUE error mapping were already correct and were not reimplemented.

### Frozen-source release gates

Executed through the locked Nix environment (**Go 1.26.10, sqlc 1.31.1**) at **20:15:15–20:15:54 UTC**, after all source/test writes completed:

```sh
nix develop -c go fmt ./...
nix develop -c go vet ./...
nix develop -c go build ./...
nix develop -c go mod tidy
nix develop -c go test -count=1 -json ./...
nix develop -c go test -race -count=1 ./...
nix develop -c go test -count=1 ./pkg/storage/sqlite -run 'Quality|Bootstrap|FreshAcceptance|Relational|JobTransition|OptionalMedia|MediaRejects'
nix develop -c go test -count=1 ./server -run 'Wire|HTTPDatabase'
# Two rounds, comparing sorted sha256sum manifests of the 25 generated files:
nix develop -c sqlc generate
nix develop -c go generate ./pkg/storage ./pkg/download ./pkg/indexer ./pkg/library
```

All PASS. Full uncached suite: **403 top-level + 844 subtests = 1,247 passing test events**, **15 tested packages**, zero failing/skipped tests. Full race suite has no reports. All **61 wire fixtures** rerun unchanged. Formatting produced no further changes; tidy left `go.mod`/`go.sum` unchanged; `git diff --check` passed. Both generation manifests retain SHA-256 **`d46315584c8366603c6ba5331fb8474d50c9a999cebd9ad62acdce81cdab14c3`**; generated Go remains byte-identical to the prior head.

The portable implementation fingerprint algorithm documented below gives **`38ae82f4bbadb00302ad344346a112dceb25fe95c6a10e6628b38b694661b5ac`** before gates, after generation/race, and after final rebuilt smoke. No implementation inputs changed across gates or publication. Final binary SHA-256: **`f5f30b48acbba446c48c1596fd3b7bb2a7a356e60c5cabada207e8bcb478ca74`**.

After the Go gates, rebuilt with `nix develop -c go build -o /tmp/mediaz-cutoff/mediaz .`, then ran the existing portable scenario described below:

```sh
LIVE_BINARY=/tmp/mediaz-cutoff/mediaz \
LIVE_DIST=/tmp/mediaz-followup/frontend/dist \
LIVE_EVIDENCE=/tmp/mediaz-cutoff/live-smoke.md \
nix develop -c node /tmp/mediaz-validation/live-smoke.mjs
```

Post-freeze smoke at **20:15:43–20:15:54 UTC**: **51 HTTP checks, zero failures**, five scheduler types done with correct history/latest rows, temporary sentinel unchanged, server stopped; fresh isolated DB/config `/tmp/mediaz-live-NoGnj3/`. The unchanged frontend dist is the prior isolated verified build; no new frontend build/typecheck/test claim. Frontend source is identical to the previously verified install/build/configured-lint baseline: **14 preexisting TypeScript errors, 26 vulnerabilities, no test script** remain explicit exceptions. Smoke checks compatibility; the new direct tests, not the smoke scenario, prove cutoff enforcement.

Supplemental artifacts: `/tmp/mediaz-cutoff/{gates.log,test.json,counts.json,race.log,fk-profile.log,wire.log,generated-A.sha256,generated-B.sha256,fingerprint-before,fingerprint-after,fingerprint-after-smoke,binary.sha256,live-smoke.md,before.log,before.exit}`. Scenario and portable fingerprint commands remain below; local artifact paths are not required to interpret the recorded results.

**Boundaries:** this is fresh-start baseline enforcement, not an incremental upgrade for existing version-1 databases. Fresh atomic bootstrap is tested; arbitrary partial-seed repair and simultaneous first-ever migrations are not claimed. With a valid cutoff but another invalid selection, preexisting multi-step manager profile/item operations can still partially persist; the missing-cutoff regressions do not claim general transactionality. No actual user DB/media deletion, merge or force-push. Human approval remains required.

## Final review followup — 2026-10-26

**Verified followup implementation: `ed8e5e481d9058f24d7827308c41d7303e8248e7`**, based on `a04454eb1aec47a9166199da662b4d98b899584e`. The evidence update is a separate documentation-only commit. This section supersedes the initial-delivery counts and fingerprints below for the current source. Exact pushed-head CI is recorded on PR #183; old green CI is not substituted for new-head checks.

### Changes and already-satisfied requirements

- Activity storage already returns generated rows directly; manager owns genuine aggregation/output and the HTTP seam. SQL already selects transition `created_at`, and the mapper already uses it rather than Unix(sequence). The new **`TestActivityTimestampFollowupSequenceOneUTC`** proves real SQLite → generated scan → real manager → raw HTTP for nonempty movie/season-backed series results: sequence **1**, fixed **`2020-02-03T04:05:06Z`**, distinct entity/parent timestamps, exact native rows/full manager output and literal UTC `Z`, with non-UTC process-local time. No activity golden was regenerated to conceal a timestamp error.
- Removed remaining internal int32 narrowing in release filters, search options, TV result helpers and library season arguments. `HistoryEntry.SortKey` is now explicitly **int64**, with direct assignments in all five history mappings. Regression values exceed MaxInt32; history JSON remains a numeric token. Genuine external SDK conversions remain at SDK boundaries; unchanged TMDB casts are not claimed to reject out-of-range external IDs.
- Independent review caught a new resolution/episode-regex false positive (`1920x1080` in a season pack). Fixed shared detection/parsing: explicit S/E markers take precedence, recognized video dimensions are skipped for bare NxM notation, and ordinal width is not capped. Permanent tests retain large S/E and NxM ordinals and accepted resolution-bearing packs. Explicit S1920E1080 disambiguates dimension-shaped ordinals.
- Fixed a real AddSeries native-identity defect exposed by failure-coverage work: persist **`SeasonNumber: sm.Number`**. Real SQLite regression reproduces the prior uniqueness failure, then verifies seasons 0/2/2147483648 and episodes 0/3/2147483649, complete generated rows, parent-scoped identity and repeated-add stability. Episode-number persistence was already correct.
- Job latest-per-parent and `(job_id, sort_key)` unique indexes and direct generated-query rejection/history tests were already present; no redundant baseline/index change. Pending-type uniqueness and actual SQLite column-name constraint mapping are preserved.
- The wire harness now registers **all eight** stored/incoming secrets at construction, before **any** response capture. Independent negative controls detect an incoming secret leaked in an earlier list response. Real routed HTTP tests use database primary key **4294967297**, assert complete responses/persisted mutations and unchanged low-ID sentinels for movies, series, clients, definitions and profiles; truncation negative controls fail as intended.
- Seed transaction comments now explicitly limit concurrency to seed attempts **after migration**. Raw SELECT EXISTS is bootstrap mechanics, not partial-default repair. No claim of simultaneous first-ever migration safety is made.
- Independent non-UTC validation exposed an existing released-series test fixture that formatted a local date two hours ago as a UTC TMDB date. Made the fixture unambiguously released using the **previous UTC calendar day**, without changing production date logic or weakening assertions. Independent complete manager normal/race runs now pass under `Pacific/Auckland`.

### Behavioral coverage mapping, not count-based equivalence

Compared removed/rewritten tests at `29f62f9` with generated-native tests at the initial delivery. Large movie/series deletions mostly replaced legacy setup and models, but one season rejection-state assertion was genuinely lost and has been restored. The table names observable protections; it does not claim exhaustive branch equivalence.

| Earlier behavior | Retained or strengthened regression evidence |
| --- | --- |
| Missing movie release selection/download identity | `Test_Manager_reconcileMissingMovie`: exact selected release/Add and complete persisted native row. New `TestMovieFailureFollowup_MissingPreservesRowAndHistory`: empty/rejected releases, source/search/Add failures, no inappropriate download, complete unchanged row/history. Individual indexer Search errors are currently logged/skipped; source-construction errors propagate. |
| Discovered metadata link and unreleased date decisions | Existing five-case `Test_Manager_reconcileDiscoveredMovie` retains lookup/link seam checks. New `TestMovieFailureFollowup_DiscoveredMetadataPersistence` verifies actual stored link and injected link-write failure; `TestMovieFailureFollowup_PastReleaseTransitionsToMissing` verifies real transition/history and repeated reconciliation. |
| Downloading guards, external failures, completion and top-level error aggregation | `Test_Manager_reconcileDownloadingMovie` retains skip/client/factory/status/library/metadata/incomplete/success cases, with persisted row/file assertions. `Test_Manager_ReconcileMovies` retains client-list failure and aggregated errors. New `TestMovieFailureFollowup_CompletedMultiFileFailures` covers later library/file-write/final-state failures and exact partial persistence. |
| Series update/delete/list/detail/add and enrichment | `TestSeriesService_*`, `TestMediaManager_AddSeriesToLibrary`, manager/server detail tests retain strict error/mapping and existing/released/unreleased-add behavior. New `TestSeriesFailureFollowup_AddSeriesChildCleanup` injects second-child failures after prior children exist, verifies complete preservation of unrelated rows and removal of target entities/history; directory failure verifies no database deletion. New ordinal test verifies genuine multi-season success. |
| TV reconcile/index state/count/download matrices | `series_reconcile_helpers_test.go`, `series_reconcile_test.go`, `series_index_test.go`, `series_native_test.go` retain runtime/state/count, season/episode/missing/continuing/discovered, filename, download and persisted-index scenarios. New `TestSeriesFailureFollowup_IndexListEpisodeFilesErrors` injects errors at both actual lookup sites; the former misleading positive subtest is renamed, not credited as error coverage. Obsolete nil-pointer input cases do not apply to value rows. |
| Rejected transitions and transaction atomicity | Restored `TestSQLite_UpdateSeasonState` compares the entire native row and full history/latest markers after rejection. Four `*SeasonTransitionFailureRollbackRecovery` / `*EpisodeTransitionFailureRollbackRecovery` tests inject real transition-insert aborts during create/update, compare complete state/history, exclude partial base rows and verify recovery. Existing movie/job/series rollback and relational tests remain. |
| Other retained behavior | Native CRUD/metadata/file/default/filter tests; profile associations; metadata cache/fallback/upstream/storage/cancellation/refresh; scheduler execution/cancellation/dedup/pruning; download/indexer service errors and keys; HTTP invalid-input/conflict/notfound/upstream-error tests remain. Wire goldens add successful serialization checks, not substitutes for negative-path tests. Legacy upgrade/schema-parity tests intentionally yield to fresh-baseline/refusal policy. |

**Limits:** tests do not promise complete branch coverage, cleanup-delete failure recovery or successful manual-search orchestration. Multi-file library actions, file-row commits and final state writes are separate boundaries: later failure leaves earlier successful effects. New tests document this behavior using strict external-library mocks and real DB rows; they do not assert filesystem compensation or retry/idempotent repair. No new such guarantee is claimed.

### Frozen-source integrated verification

Application/test edits froze before the integrated run. **2026-10-26 20:01:06–20:01:34 UTC:** all commands exited 0 using Nix **Go1.26.10 / sqlc1.31.1**, in this order:

```sh
# Two rounds; compare sorted SHA-256 manifests over all generated paths below.
sqlc generate
go generate ./pkg/storage ./pkg/download ./pkg/indexer ./pkg/library
# Repeat both commands and compare manifests.
go fmt ./...
go mod tidy
go vet ./...
go build ./...
go test -count=1 -json ./...
go test -race -count=1 ./...
TZ=Pacific/Auckland go test -count=1 ./pkg/storage/sqlite ./pkg/manager ./server \
  -run 'Activity|Transition|Wire|Large|Int64|Secret'
go build -o /tmp/mediaz-followup/mediaz .
```

- **401 top-level + 842 subtests = 1,243 passing events**, zero failing/skipped tests, **15 tested packages**; 15 additional packages have no tests. Full race suite passes with no reports. `go.mod`/`go.sum` unchanged; `git diff --check` passes.
- **61 wire fixtures**, unchanged in this followup, pass in normal/race suites. The accepted timestamp correction remains documented below and in the wire README; fixed raw timestamp assertions prevent normalization from hiding it.
- **25 generated Go files** repeat identically: the prior 24 storage/download/indexer generated hashes below remain unchanged, plus library mock `34be13053a2fdba8681131472c741ce8b891d06c981dba4b7687a0d2748dcc94`. Sorted manifest SHA-256 in both rounds: `d46315584c8366603c6ba5331fb8474d50c9a999cebd9ad62acdce81cdab14c3`. Manifest command: `fd -e go . pkg/storage/sqlcdb pkg/storage/mocks pkg/download/mocks pkg/indexer/mocks pkg/library/mocks | sort | xargs sha256sum`.
- Source fingerprint (same portable `.go`/`.sql`/module/sqlc-config algorithm below) before tests, after race and after final rebuilt smoke: **`14c1ed3b1596a74a18ff5a4d3b0875747a9c58c9993cc1b4d2bf2ccfc77461f1`**. This includes new tests/generated inputs. No implementation inputs changed between those gates and publication.
- Rebuilt binary SHA-256: **`d10e590c98423a474588602d805aa9df6f733a5c2d44e4cc5fc9212fe2ee77f6`**. Post-gate live smoke completed **20:01:58 UTC**: **51 HTTP checks, zero failures**, all five scheduler types done, correct job history/latest rows, unchanged temporary media sentinel, server stopped. Uses the portable fresh-DB/local-stub scenario below, with new binary and freshly installed/built isolated frontend; no real DB/media/services touched.
- Frontend repeated from an isolated fresh copy: `npm ci`, production build and configured lint pass. `npm test` still exits 1 (no script); typecheck exits 2 with **14 diagnostics byte-identical to the isolated `29f62f9` baseline**; install still reports **26 vulnerabilities (2 low, 7 moderate, 17 high)**. No frontend source changes, typecheck/test/audit success or full-source lint claim.
- Independent final reviews: **zero remaining material blockers** from independent int64/HTTP and post-freeze integration/coverage reviews. The latter independently reran full manager normal/race under Auckland after the UTC fixture correction and inspected explicit int64 history mappings. Negative controls independently establish early-secret and large-path-ID tests catch their intended regressions.

Supplemental followup artifacts: `/tmp/mediaz-followup/{gates.log,test.json,counts.json,race.log,targeted.log,generated-A.sha256,generated-B.sha256,binary.sha256,live-smoke.md,frontend-*.log,frontend-baseline.diff}`; temporary live DB/config `/tmp/mediaz-live-kfWbwM/`. Artifacts are supplemental; commands, counts, hashes, scenario mappings and caveats are recorded here durably. The baseline reset requirement and out-of-scope simultaneous first migration/arbitrary seed repair remain unchanged. GitHub human approval remains required; no merge or force-push performed.

---

## Historical initial delivery evidence (8ad1e1a / a04454e)

The following records the earlier immutable delivery, not test counts or source identity of the followup above.

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

These are bounded intentional corrections, **not** blanket golden regeneration. Initial independent review found no unexplained remaining golden mismatch. The later followup coverage audit above identified and restored a lost season state/history assertion, added meaningful failure-boundary coverage, and qualified the limits of any scenario-equivalence claim.

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
