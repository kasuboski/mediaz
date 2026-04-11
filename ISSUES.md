# Issue Breakdown Plan

Parent tracking issue: **#126** (Refactor list)

PR #125 was already merged and addressed:
- ~~ARCH-02: Server monolith split~~ → Done
- ~~DUP-04: Server handler boilerplate~~ → Done (helpers added)
- ~~ERR-03: Inconsistent HTTP error format~~ → Done (standardized on respondError)
- ~~HTTP-01: Serve() hangs on bind failure~~ → Done
- ~~HTTP-03: DELETE endpoints read body inconsistently~~ → Done
- ~~BUG-05: Copy-pasted log messages in server handlers~~ → Done

Items below are **still present in the codebase** and should become their own GitHub issues.

---

## Category 1: Bugs

### Issue: BUG-01 — Double `Body.Close()` in metadata fetch/parse
- **Status**: STILL PRESENT
- **File**: `pkg/manager/metadata.go` lines 360–451
- **Description**: `fetchExternalIDs` defers `extIDsResp.Body.Close()` (line 369), then passes the same `*http.Response` to `parseExternalIDs` which also defers `resp.Body.Close()` (line 422). Same pattern for `fetchWatchProviders`/`parseWatchProviders` (lines 391–447).
- **Fix**: Remove `defer resp.Body.Close()` from `parseExternalIDs` and `parseWatchProviders`. The caller already defers the close.
- **Effort**: Small
- **Labels**: `bug`
- **Priority**: Immediate

### Issue: BUG-03 — `ReconcileMovies` and `ReconcileSeries` always return nil
- **Status**: STILL PRESENT
- **Files**: `pkg/manager/movie_reconcile.go:91–126`, `pkg/manager/series_reconcile.go:23–61`
- **Description**: Both functions call sub-reconcilers sequentially, log errors, overwrite the error variable, and always return `nil`. The scheduler's `executeJob` marks the job as `Done` with zero visibility into reconciliation failures.
- **Fix**: Use `errors.Join` to aggregate errors. All sub-reconcilers still run (preserving "log and continue" intent), but the scheduler gets visibility into partial failures. Consider adding a "partial failure" job state.
- **Effort**: Small
- **Labels**: `bug`
- **Priority**: Short Term

### Issue: BUG-06 — `findMatchingSeriesResult` panics on empty results when year is nil
- **Status**: STILL PRESENT
- **File**: `pkg/manager/series_reconcile.go:1201–1215`
- **Description**: When `year == nil`, the function directly accesses `results[0]` without checking if the slice is empty. Compare with `findMatchingMovieResult` (movie_reconcile.go:523) which correctly checks `len(results) == 0` first.
- **Fix**: Add `if len(results) == 0 { return nil }` before the `year == nil` branch.
- **Effort**: Small
- **Labels**: `bug`
- **Priority**: Immediate

### Issue: BUG-07 — `getEpisodeFileByID` loads ALL episode files to find one
- **Status**: STILL PRESENT
- **File**: `pkg/manager/series_reconcile.go:1280–1293`
- **Description**: Calls `m.storage.ListEpisodeFiles(ctx)` which loads every episode file from the database, then iterates to find one by ID. The storage interface already has `GetEpisodeFile(ctx, id int32)` which does a targeted query by primary key. Called per discovered episode during reconciliation.
- **Fix**: Replace with `m.storage.GetEpisodeFile(ctx, int32(fileID))`.
- **Effort**: Small
- **Labels**: `bug`, `performance`
- **Priority**: Immediate

---

## Category 2: Error Handling

### Issue: ERR-02 — Unchecked errors on metadata refresh in loops
- **Status**: STILL PRESENT
- **File**: `pkg/manager/metadata.go:482–512`
- **Description**: In `RefreshSeriesMetadata` (and `RefreshMovieMetadata`), individual refresh errors are logged but never accumulated or returned. For bulk operations this is intentional (don't stop 97 successful refreshes because 3 fail), but callers have no visibility into partial failures.
- **Fix**: Optionally collect errors with `errors.Join` and return the aggregate.
- **Effort**: Small
- **Labels**: `improvement`
- **Priority**: Short Term

### Issue: ERR-04 — `RefreshSeriesMetadataFromTMDB` fails on existing metadata
- **Status**: STILL PRESENT
- **File**: `pkg/manager/metadata.go:88–89, 141–167`
- **Description**: `RefreshSeriesMetadataFromTMDB` delegates to `loadSeriesMetadata` which always calls `CreateSeriesMetadata` (line 165). The schema has `tmdb_id INTEGER NOT NULL UNIQUE`, so calling Create on an existing record returns a UNIQUE constraint violation. `UpdateSeriesMetadataFromTMDB` already exists and correctly does GET → UPDATE.
- **Fix**: Have `RefreshSeriesMetadataFromTMDB` check for existing metadata and call `UpdateSeriesMetadataFromTMDB` if found, or have `loadSeriesMetadata` use an upsert pattern.
- **Effort**: Small
- **Labels**: `bug`
- **Priority**: Immediate

---

## Category 3: Performance

### Issue: PERF-01 — O(n×m) movie file indexing loop
- **Status**: STILL PRESENT
- **File**: `pkg/manager/manager.go:721–780`
- **Description**: `IndexMovieLibrary` compares each discovered file against every tracked file using a nested loop. For a library with 1000 movies and 1000 tracked files, this is 1,000,000 string comparisons.
- **Fix**: Build a `map[string]struct{}` (set) of tracked paths upfront, then O(1) lookup per discovered file.
- **Effort**: Small
- **Labels**: `performance`
- **Priority**: Immediate

### Issue: PERF-04 — N+1 queries in `reconcileMissingSeries`
- **Status**: STILL PRESENT
- **File**: `pkg/manager/series_reconcile.go:333–500`
- **Description**: For each season, queries episodes individually, then for each episode queries episode metadata individually (`GetEpisodeMetadata` per episode at lines ~388, ~483). Creates N+1 query patterns throughout the reconcile loop.
- **Fix**: Batch-load episode metadata by season IDs upfront, or add storage methods that join episode + metadata in a single query.
- **Effort**: Medium
- **Labels**: `performance`
- **Priority**: Short Term

### Issue: PERF-05 — `getSeasonsWithEpisodes` does N+1 metadata lookups
- **Status**: STILL PRESENT
- **File**: `pkg/manager/manager.go:347–488`
- **Description**: For each season, fetches season metadata individually via `GetSeasonMetadata` (line 371). For each episode, fetches episode metadata individually via `GetEpisodeMetadata` (line 419). This is 1 + N queries per season. `ListEpisodesForSeason` (line 1302) has the same N+1 pattern.
- **Fix**: Add batch methods like `ListSeasonMetadataBySeriesID` and `ListEpisodeMetadataBySeasonIDs`.
- **Effort**: Medium
- **Labels**: `performance`
- **Priority**: Short Term

### Issue: PERF-03 — Single global mutex for all SQLite write operations
- **Status**: STILL PRESENT
- **File**: `pkg/storage/sqlite/sqlite.go:18, 90–109`
- **Description**: `SQLite.mu` is a `sync.Mutex` that serializes ALL write operations via `handleStatement`. SQLite is already configured with WAL mode + `busy_timeout = 5000`. The mutex is redundant but provides simpler guarantees.
- **Fix**: Can be removed — rely on `busy_timeout` instead. Low priority for a media manager with low write volume.
- **Effort**: Medium
- **Labels**: `performance`, `improvement`
- **Priority**: Long Term

---

## Category 4: Code Duplication

### Issue: DUP-01 — Episode result building duplicated between `getEpisodesForSeason` and `ListEpisodesForSeason`
- **Status**: STILL PRESENT
- **File**: `pkg/manager/manager.go:408–488` (getEpisodesForSeason) vs `1302–1450` (ListEpisodesForSeason)
- **Description**: Both methods iterate episodes, look up metadata, and build `EpisodeResult` structs with nearly identical logic — including the same fallback values (`TMDBID = 0`, `Number = episode.EpisodeNumber`, `Title = fmt.Sprintf("Episode %d"...)`). The entire metadata-to-result transformation is copy-pasted.
- **Fix**: Extract a common `buildEpisodeResult(episode, episodeMeta, seriesID, seasonNumber) EpisodeResult` helper. Both methods call it.
- **Effort**: Small
- **Labels**: `refactor`
- **Priority**: Immediate

### Issue: DUP-02 — `ListMoviesInLibrary` and `ListShowsInLibrary` are nearly identical
- **Status**: STILL PRESENT
- **File**: `pkg/manager/manager.go:632–706`
- **Description**: Both methods list entities, skip nil metadata, look up metadata, and build response objects. Structure is identical, differing only in types (Series vs Movie).
- **Fix**: Consider a generic `listLibraryItems[TEntity, TMetadata, TResult]` function, or at minimum extract shared patterns.
- **Effort**: Medium
- **Labels**: `refactor`
- **Priority**: Long Term

### Issue: DUP-03 — `AddMovieToLibrary` and `AddSeriesToLibrary` share validation/state logic
- **Status**: STILL PRESENT
- **File**: `pkg/manager/manager.go:827–944`
- **Description**: Both methods: validate quality profile → fetch metadata → check if entity exists → create entity with state based on release date → return entity. The pattern is identical.
- **Fix**: Extract a shared pattern for "add media to library" operations.
- **Effort**: Medium
- **Labels**: `refactor`
- **Priority**: Long Term

### Issue: DUP-05 — State evaluator duplication between seasons and series
- **Status**: STILL PRESENT
- **File**: `pkg/manager/series_reconcile.go:808–895`
- **Description**: `evaluateAndUpdateSeasonState` (~40 lines) and `evaluateAndUpdateSeriesState` (~45 lines) both follow the same pattern: fetch entity → list children → count states → determine new state → update.
- **Fix**: Consider a generic state evaluator that takes a list of child states and transition rules.
- **Effort**: Medium
- **Labels**: `refactor`
- **Priority**: Long Term

---

## Category 5: Testing Gaps

### Issue: TEST-01 — No tests for `pkg/manager/indexer_source.go`
- **Status**: STILL PRESENT — no test file exists
- **Description**: All indexer source CRUD operations, refresh logic, and cache management (236 lines) have no tests. This is a critical integration path.
- **Fix**: Add unit tests using mock storage and mock indexer factory.
- **Effort**: Medium
- **Labels**: `testing`
- **Priority**: Short Term

### Issue: TEST-02 — No tests for `pkg/manager/activity.go` / `activity_types.go`
- **Status**: PARTIALLY PRESENT — no test file for activity.go
- **Description**: Activity tracking and timeline features (322 + 135 lines) have no test coverage.
- **Fix**: Add integration tests with in-memory SQLite.
- **Effort**: Medium
- **Labels**: `testing`
- **Priority**: Short Term

### Issue: TEST-05 — No tests for `pkg/indexer/`
- **Status**: STILL PRESENT — no test files in pkg/indexer/
- **Description**: The indexer integration layer (prowlarr.go, factory.go) has zero tests.
- **Fix**: Add tests with mock HTTP client.
- **Effort**: Medium
- **Labels**: `testing`
- **Priority**: Short Term

### Issue: TEST-06 — No tests for `pkg/storage/sqlite/activity.go`
- **Status**: STILL PRESENT — no activity_test.go exists
- **Description**: Activity queries (active downloads, failures, timeline, transition history) have no tests despite complex SQL with joins.
- **Fix**: Add integration tests with in-memory SQLite.
- **Effort**: Medium
- **Labels**: `testing`
- **Priority**: Short Term

### Issue: TEST-07 — No tests for `pkg/storage/sqlite/movie_file.go`
- **Status**: STILL PRESENT — no movie_file_test.go exists
- **Description**: Movie file CRUD operations have no direct tests (tested indirectly through manager tests).
- **Fix**: Add focused storage-level tests.
- **Effort**: Medium
- **Labels**: `testing`
- **Priority**: Short Term

### Issue: TEST-08 — Expand server endpoint test coverage
- **Status**: PARTIALLY ADDRESSED — PR #125 added tests for many endpoints (download, search, movie, job, series, static handlers), but indexer and quality handlers still lack tests
- **Description**: Server tests now cover: health check, movie detail, TV detail, search (movie/series/season/episode), jobs (list/get/cancel/create), download client (update/test). Missing: indexer CRUD, quality profile CRUD, quality definition CRUD.
- **Fix**: Add tests for indexer_handlers.go and quality_handlers.go.
- **Effort**: Medium
- **Labels**: `testing`
- **Priority**: Short Term

---

## Category 6: Architecture

### Issue: ARCH-01 — `MediaManager` is a god object (130+ methods)
- **Status**: STILL PRESENT
- **File**: `pkg/manager/` — ~17,000 lines total
- **Description**: `MediaManager` handles movies, series, indexing, reconciliation, metadata, downloads, quality, search, jobs, indexer sources, activity, config — everything. Single struct with 8 dependencies.
- **Fix**: Split into domain-focused services: `MovieService`, `SeriesService`, `IndexerService`, `MetadataService`, `DownloadService`, `JobService`. `MediaManager` becomes a thin facade. Do incrementally — extract one service at a time.
- **Effort**: Large
- **Labels**: `architecture`, `refactor`
- **Priority**: Long Term

### Issue: ARCH-04 — No structured request validation
- **Status**: STILL PRESENT
- **File**: `server/` handlers and `pkg/manager/*.go`
- **Description**: Request validation is ad-hoc and inconsistent. Some handlers check for empty strings, some don't. Request types (`AddMovieRequest`, `AddSeriesRequest`) have no validation tags.
- **Fix**: Add a validation layer (e.g., `go-playground/validator` or manual `Validate()` methods on request types).
- **Effort**: Medium
- **Labels**: `architecture`, `improvement`
- **Priority**: Long Term

### Issue: ARCH-05 — Magic numbers throughout
- **Status**: STILL PRESENT
- **Files**: `pkg/manager/scheduler.go:401` (`30 * time.Second`), `pkg/download/transmission.go:109,111` (`>> 20`), `pkg/download/transmission.go:113` (`t.Status > 4`, `t.PercentDone == 100.0`), `pkg/manager/release.go:242` (`>> 20`)
- **Description**: Unexplained constants for timeout durations, bit shifts, torrent status codes, and percentage comparisons.
- **Fix**: Extract named constants with descriptive names.
- **Effort**: Small
- **Labels**: `improvement`
- **Priority**: Long Term

### Issue: ARCH-06 — `Storage` interface is massive (60+ methods)
- **Status**: STILL PRESENT
- **File**: `pkg/storage/storage.go` (376 lines, ~101 methods across all composed interfaces)
- **Description**: The `Storage` interface combines 11 sub-interfaces into one mega-interface. Any consumer only needs a subset. Violates Interface Segregation Principle.
- **Fix**: Have consumers depend on the specific sub-interfaces they need (e.g., `MovieStorage`, `JobStorage`).
- **Effort**: Medium
- **Labels**: `architecture`, `refactor`
- **Priority**: Long Term

---

## Category 7: Naming & Documentation

### Issue: NAME-01 — Inconsistent naming: `MOVIE_CATEGORIES` / `TV_CATEGORIES` constants
- **Status**: STILL PRESENT
- **File**: `pkg/manager/movie_reconcile.go:23–24`
- **Description**: Constants `MOVIE_CATEGORIES` and `TV_CATEGORIES` violate Go naming conventions (should be `movieCategories` / `tvCategories`). Also mixed method naming patterns across the codebase.
- **Fix**: Rename to Go-conventional camelCase. Standardize method naming patterns.
- **Effort**: Small
- **Labels**: `improvement`, `documentation`
- **Priority**: Long Term

### Issue: NAME-02 — Exported generic `ptr` function
- **Status**: STILL PRESENT
- **File**: `pkg/manager/manager.go:1217`
- **Description**: `ptr[A any](thing A) *A` is exported with a completely generic name. In Go, helpers like this are usually unexported.
- **Fix**: Rename to a more descriptive name or unexport it.
- **Effort**: Small
- **Labels**: `improvement`
- **Priority**: Long Term

### Issue: NAME-03 — Mixed logging styles throughout
- **Status**: STILL PRESENT
- **Files**: Throughout `pkg/manager/` — structured (`zap.Error`), `Errorw` style, and `Errorf` style all used
- **Description**: Mix of `log.Error("msg", zap.Error(err))`, `log.Errorw("msg", "key", value)`, and `log.Errorf("msg: %v", err)`. Makes code harder to search and parse.
- **Fix**: Standardize on structured logging with `zap.Field` (`log.Error("msg", zap.Error(err))`).
- **Effort**: Medium
- **Labels**: `improvement`
- **Priority**: Long Term

### Issue: NAME-04 — Actionable TODO comments without tracking
- **Status**: STILL PRESENT
- **Files**: Multiple
- **Description**: TODOs exist without issue tracking:
  - `pkg/manager/manager.go:770` — "TODO: make sure it's actually relative"
  - `pkg/manager/manager.go:826` — "TODO: check status of movie before doing anything else"
  - `pkg/manager/movie_reconcile.go:22` — "TODO: these are specific per indexer"
  - `pkg/manager/movie_reconcile.go:301` — "TODO: should this update state?"
  - `pkg/storage/storage.go:58` — "TODO: do we cascade associated items?"
- **Fix**: Create issues for actionable TODOs. Remove stale TODOs.
- **Effort**: Small
- **Labels**: `documentation`, `improvement`
- **Priority**: Long Term

---

## Recommended Issue Groupings

Rather than creating 30+ individual issues, here are recommended groupings for creating GitHub issues. Each becomes a single issue linked back to #126.

| # | Issue Title | Findings Included | Labels | Priority |
|---|------------|-------------------|--------|----------|
| 1 | Fix: Double Body.Close() in metadata fetch/parse | BUG-01 | bug | Immediate |
| 2 | Fix: findMatchingSeriesResult panics on empty results | BUG-06 | bug | Immediate |
| 3 | Fix: getEpisodeFileByID full table scan | BUG-07 | bug, performance | Immediate |
| 4 | Fix: RefreshSeriesMetadataFromTMDB fails on existing metadata | ERR-04 | bug | Immediate |
| 5 | Fix: O(n×m) movie library indexing | PERF-01 | performance | Immediate |
| 6 | Fix: Duplicate episode result builder | DUP-01 | refactor | Immediate |
| 7 | Fix: Reconcile errors silently swallowed | BUG-03 | bug | Short Term |
| 8 | Fix: N+1 queries in series reconciliation | PERF-04, PERF-05 | performance | Short Term |
| 9 | Improve: Accumulate errors in bulk metadata refresh | ERR-02 | improvement | Short Term |
| 10 | Tests: Add indexer source and integration tests | TEST-01, TEST-05 | testing | Short Term |
| 11 | Tests: Add activity and storage tests | TEST-02, TEST-06, TEST-07 | testing | Short Term |
| 12 | Tests: Expand server endpoint coverage | TEST-08 | testing | Short Term |
| 13 | Refactor: Split MediaManager god object | ARCH-01 | architecture, refactor | Long Term |
| 14 | Refactor: Slim down Storage interface | ARCH-06 | architecture, refactor | Long Term |
| 15 | Refactor: Add request validation layer | ARCH-04 | architecture, improvement | Long Term |
| 16 | Refactor: Deduplicate library and add-media logic | DUP-02, DUP-03, DUP-05 | refactor | Long Term |
| 17 | Cleanup: Remove global SQLite write mutex | PERF-03 | performance, improvement | Long Term |
| 18 | Cleanup: Fix naming, logging, magic numbers, and TODOs | NAME-01, NAME-02, NAME-03, NAME-04, ARCH-05 | improvement, documentation | Long Term |
