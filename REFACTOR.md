# REFACTOR.md — Verified Code Quality, Bug, and Improvement Findings

Generated: 2026-04-10 · Verified: 2026-04-11

Each finding was reviewed against the source code. Non-issues and false positives have been removed. Remaining items include caveats and context for implementation decisions.

---

## Table of Contents
1. [Bugs](#1-bugs)
2. [Error Handling](#2-error-handling)
3. [Performance](#3-performance)
4. [Code Duplication](#4-code-duplication)
5. [Testing Gaps](#5-testing-gaps)
6. [Architecture & Design](#6-architecture--design)
7. [HTTP Server Issues](#7-http-server-issues)
8. [Naming & Documentation](#8-naming--documentation)

---

## 1. Bugs

### BUG-01: Double `Body.Close()` in metadata fetch/parse
- **File**: `pkg/manager/metadata.go`
- **Lines**: 378 (fetchExternalIDs defer) + 424 (parseExternalIDs defer), 408 (fetchWatchProviders defer) + 437 (parseWatchProviders defer)
- **Severity**: Medium
- **Category**: Resource Management
- **Description**: `fetchExternalIDs` defers `extIDsResp.Body.Close()`, then passes the same `*http.Response` to `parseExternalIDs` which also defers `resp.Body.Close()`. Same pattern for `fetchWatchProviders`/`parseWatchProviders`. The parse defer fires first, the fetch defer fires second — double close.
- **Caveat**: With Go's standard `net/http`, closing an already-closed body is a no-op (the body reader tracks closed state). **No panic or data corruption occurs** with the standard library. The `ReadAll` completes before either close fires. However, this is fragile — alternative HTTP transports may not be as forgiving.
- **Fix**: Remove `defer resp.Body.Close()` from `parseExternalIDs` and `parseWatchProviders`. The caller already defers the close.

### BUG-03: `ReconcileMovies` and `ReconcileSeries` always return nil
- **File**: `pkg/manager/movie_reconcile.go:91-121`, `pkg/manager/series_reconcile.go:28-58`
- **Severity**: Medium
- **Category**: Error Handling
- **Description**: Both functions call sub-reconcilers sequentially, log errors, then overwrite the error variable and continue. Both always return `nil`. The scheduler's `executeJob` (scheduler.go:357) marks the job as `Done` when the executor returns nil, so the scheduler has **zero visibility** into reconciliation failures.
- **Design context**: The "log and continue" pattern is intentional — each sub-reconciler should run independently. If `ReconcileMissingMovies` fails, you still want `ReconcileDownloadingMovies` to check on in-progress downloads. The next scheduled cycle retries any failed step. However, the scheduler should know when a job had partial failures.
- **Fix**: Use `errors.Join` to aggregate errors. This preserves the current execution order (all sub-reconcilers still run) while giving the scheduler visibility into failures:
  ```go
  var allErrors error
  if err := m.ReconcileMissingMovies(ctx, snapshot); err != nil {
      log.Error("failed to reconcile missing movies", zap.Error(err))
      allErrors = errors.Join(allErrors, err)
  }
  // ... continue with other reconcilers
  return allErrors
  ```
  Consider adding a "partial failure" job state so retry logic isn't triggered unnecessarily.

### BUG-05: Copy-pasted log messages in server handlers
- **File**: `server/server.go`
- **Lines**: 339, 373, 494, 528 (log "adding indexer" for quality definition CRUD), 347/381/502/536 ("succesfully" — typo)
- **Severity**: Low
- **Category**: Incorrect Log Messages
- **Description**: Multiple handlers log "adding indexer" when they're actually creating/deleting quality definitions. The typo "succesfully" appears 4 times. Delete handlers log "adding indexer". Download client delete handler logs "failed to get download client" instead of "failed to delete".
- **Fix**: Fix log messages to match the actual operation. Fix "succesfully" → "successfully".

### BUG-06: `findMatchingSeriesResult` panics on empty results when year is nil
- **File**: `pkg/manager/series_reconcile.go`
- **Lines**: 1201–1215
- **Severity**: Medium
- **Category**: Bug — Index Out of Range Panic
- **Description**: When `year == nil`, `findMatchingSeriesResult` directly accesses `results[0]` without checking if the slice is empty. Compare with `findMatchingMovieResult` which correctly checks `len(results) == 0` first. If a TMDB search returns zero results and the year is nil, this function panics.
- **Note**: The original report claimed returning nil on year mismatch was a bug — it's not. Year verification is correct behavior (prevents linking "Battlestar Galactica" 2004 to the 1978 series). The actual bug is the missing empty-results guard.
- **Fix**: Add `if len(results) == 0 { return nil }` before the `year == nil` branch, matching `findMatchingMovieResult`.

### BUG-07: `getEpisodeFileByID` loads ALL episode files to find one
- **File**: `pkg/manager/series_reconcile.go`
- **Lines**: 1280–1293
- **Severity**: Medium
- **Category**: Performance / Correctness
- **Description**: This method calls `m.storage.ListEpisodeFiles(ctx)` which loads every episode file from the database, then iterates to find one by ID. The storage interface already has `GetEpisodeFile(ctx, id int32)` which does a targeted query by primary key.
- **Impact**: O(n) database load per call. Called per discovered episode during reconciliation.
- **Fix**: Replace with `m.storage.GetEpisodeFile(ctx, int32(fileID))`.

---

## 2. Error Handling

### ERR-02: Unchecked errors on metadata refresh in loops
- **File**: `pkg/manager/metadata.go`
- **Lines**: 494, 504, 524, 534
- **Severity**: Low
- **Category**: Error Handling
- **Description**: In `RefreshSeriesMetadata` and `RefreshMovieMetadata`, when iterating over all metadata records, individual refresh errors are logged but the loop continues. The error is never accumulated or returned.
- **Design context**: This is intentional for bulk operations — if you're refreshing 100 series and 3 fail, stopping would prevent the remaining 97 from refreshing. The current behavior is reasonable. Aggregating errors with `errors.Join` would be a nice improvement for callers who want to know how many failed, but it's not a bug.
- **Fix**: Optionally collect errors with `errors.Join` and return the aggregate.

### ERR-03: Inconsistent error handling in HTTP handlers
- **File**: `server/server.go`
- **Lines**: Throughout (e.g., 222 vs 248 vs 290)
- **Severity**: Medium
- **Category**: Code Quality
- **Description**: Some handlers use `http.Error()` for errors (lines 222, 241, 290), others use `s.writeErrorResponse()`. The former sends `text/plain`, the latter sends `application/json`. Clients receive inconsistent error formats.
- **Impact**: Frontend can't reliably parse error responses.
- **Fix**: Standardize on `s.writeErrorResponse()` for all error responses. Never use `http.Error()` directly.

### ERR-04: `RefreshSeriesMetadataFromTMDB` fails on existing metadata instead of updating
- **File**: `pkg/manager/metadata.go`
- **Lines**: 96-97, 141-168
- **Severity**: Medium
- **Category**: Error Handling
- **Description**: `RefreshSeriesMetadataFromTMDB` delegates to `loadSeriesMetadata`, which always calls `CreateSeriesMetadata`. The schema has `tmdb_id INTEGER NOT NULL UNIQUE`, so calling Create on an existing record returns a UNIQUE constraint violation error instead of updating.
- **Note**: `GetSeriesMetadata` handles this correctly — it first tries to GET, and only calls `loadSeriesMetadata` on `ErrNotFound`. The issue only manifests when `RefreshSeriesMetadataFromTMDB` is called for a series that already has metadata (e.g., from `reconcileContinuingSeries`). Note that `UpdateSeriesMetadataFromTMDB` already exists and correctly does a GET → UPDATE pattern.
- **Fix**: Have `RefreshSeriesMetadataFromTMDB` check for existing metadata and call `UpdateSeriesMetadataFromTMDB` if found, or have `loadSeriesMetadata` use an upsert pattern.

---

## 3. Performance

### PERF-01: O(n×m) movie file indexing loop
- **File**: `pkg/manager/manager.go`
- **Lines**: 736–770
- **Severity**: High
- **Category**: Performance
- **Description**: `IndexMovieLibrary` compares each discovered file against every tracked file using a nested loop. For a library with 1000 movies and 1000 tracked files, this is 1,000,000 string comparisons.
- **Fix**: Build a `map[string]struct{}` (set) of tracked paths upfront, then O(1) lookup per discovered file:
  ```go
  tracked := make(map[string]struct{}, len(movieFiles))
  for _, mf := range movieFiles {
      if mf != nil {
          tracked[strings.ToLower(*mf.RelativePath)] = struct{}{}
          tracked[strings.ToLower(*mf.OriginalFilePath)] = struct{}{}
      }
  }
  ```

### PERF-03: Single global mutex for all SQLite write operations
- **File**: `pkg/storage/sqlite/sqlite.go`
- **Lines**: 18, 90–109
- **Severity**: Low
- **Category**: Performance
- **Description**: `SQLite.mu` is a `sync.Mutex` that serializes ALL write operations (insert/update/delete) via `handleStatement`. Each write is wrapped in a transaction under the mutex.
- **Caveat**: SQLite is configured with WAL mode + `busy_timeout = 5000`, which already handles concurrent write contention at the database level. **Reads are NOT serialized** — they go through the `sql.DB` connection pool directly. The mutex is redundant with SQLite's built-in locking but provides simpler guarantees (no `SQLITE_BUSY` errors to handle). For a media manager with low write volume, this is acceptable.
- **Fix**: Can be removed if desired — rely on `busy_timeout` instead. Not urgent.

### PERF-04: N+1 queries in `reconcileMissingSeries`
- **File**: `pkg/manager/series_reconcile.go`
- **Lines**: ~195–290
- **Severity**: Medium
- **Category**: Performance
- **Description**: For each season, the code queries episodes, then for each episode queries episode metadata individually. This creates N+1 query patterns throughout the reconcile loop.
- **Impact**: Slow reconciliation for series with many seasons/episodes.
- **Fix**: Batch-load episode metadata by season IDs upfront, or add storage methods that join episode + metadata in a single query.

### PERF-05: `getSeasonsWithEpisodes` does N+1 metadata lookups
- **File**: `pkg/manager/manager.go`
- **Lines**: 210–290
- **Severity**: Medium
- **Category**: Performance
- **Description**: For each season, fetches season metadata individually. For each episode in that season, fetches episode metadata individually. This is 1 + N queries per season.
- **Impact**: Slow API responses for series detail views.
- **Fix**: Add batch methods like `ListSeasonMetadataBySeriesID` and `ListEpisodeMetadataBySeasonIDs`.

---

## 4. Code Duplication

### DUP-01: Episode result building duplicated between `getEpisodesForSeason` and `ListEpisodesForSeason`
- **File**: `pkg/manager/manager.go`
- **Lines**: 280–330 (getEpisodesForSeason) vs 1385–1455 (ListEpisodesForSeason)
- **Severity**: High
- **Category**: Code Duplication
- **Description**: Both methods iterate episodes, look up metadata, and build `EpisodeResult` structs with nearly identical logic — including the same fallback values (`result.TMDBID = 0`, `result.Number = episode.EpisodeNumber`, `result.Title = fmt.Sprintf("Episode %d"...)`). The entire metadata-to-result transformation is copy-pasted.
- **Impact**: Bug fixes must be applied in two places. Inconsistency risk.
- **Fix**: Extract a common `buildEpisodeResult(episode, episodeMeta, seriesID, seasonNumber) EpisodeResult` helper. Both methods call it.

### DUP-02: `ListMoviesInLibrary` and `ListShowsInLibrary` are nearly identical
- **File**: `pkg/manager/manager.go`
- **Lines**: 878–918 (ListShowsInLibrary) vs 920–958 (ListMoviesInLibrary)
- **Severity**: Medium
- **Category**: Code Duplication
- **Description**: Both methods list entities, skip nil metadata, look up metadata, and build response objects. The structure is identical, differing only in types (Series vs Movie).
- **Impact**: Maintenance burden. Bug fixes must be applied in two places.
- **Fix**: Consider a generic `listLibraryItems[TEntity, TMetadata, TResult]` function, or at minimum extract shared patterns.

### DUP-03: `AddMovieToLibrary` and `AddSeriesToLibrary` share validation/state logic
- **File**: `pkg/manager/manager.go`
- **Lines**: 660–720 (AddMovieToLibrary) vs 730–815 (AddSeriesToLibrary)
- **Severity**: Medium
- **Category**: Code Duplication
- **Description**: Both methods: validate quality profile → fetch metadata → check if entity exists → create entity with state based on release date → return entity. The pattern is identical.
- **Fix**: Extract a shared pattern for "add media to library" operations.

### DUP-04: Server handler boilerplate repeated ~50 times
- **File**: `server/server.go`
- **Lines**: Throughout (1471 lines total)
- **Severity**: Medium
- **Category**: Code Duplication
- **Description**: Nearly every handler follows the same pattern: parse ID from URL → parse body → call manager → write response. The parse-ID-from-URL code alone is duplicated ~20 times. The "read body + unmarshal + error check" block is duplicated ~15 times.
- **Impact**: File is 1471 lines and grows linearly with each new endpoint. Error handling inconsistencies.
- **Fix**: Create helper functions:
  ```go
  func parseID(w, r) (int64, bool) { ... }
  func readJSON(w, r, v) bool { ... }
  func respond(w, status, data) { ... }
  ```
  Or use a lightweight framework with routing + validation middleware.

### DUP-05: Movie reconcile vs series reconcile state evaluation
- **File**: `pkg/manager/series_reconcile.go`
- **Lines**: `evaluateAndUpdateSeasonState` (~200 lines) vs `evaluateAndUpdateSeriesState` (~150 lines)
- **Severity**: Low
- **Category**: Code Duplication
- **Description**: Both methods follow the same pattern: fetch entity → list children → count states → determine new state → update. The state determination logic is similar but differs in specific state names.
- **Impact**: Maintenance burden when adding new states.
- **Fix**: Consider a generic state evaluator that takes a list of child states and transition rules.

---

## 5. Testing Gaps

### TEST-01: No tests for `pkg/manager/indexer_source.go`
- **File**: `pkg/manager/indexer_source.go`
- **Severity**: High
- **Category**: Missing Test Coverage
- **Description**: All indexer source CRUD operations, refresh logic, and cache management have no tests. This is a critical integration path.
- **Impact**: Regressions in indexer source management go undetected.
- **Fix**: Add unit tests using mock storage and mock indexer factory.

### TEST-02: No tests for `pkg/manager/download.go`
- **File**: `pkg/manager/download.go`
- **Severity**: High
- **Category**: Missing Test Coverage
- **Description**: Download client CRUD, `availableProtocols`, `clientForProtocol` have no tests.
- **Fix**: Add unit tests for the helper functions and CRUD operations with mock storage.

### TEST-03: No tests for `pkg/manager/activity.go`, `activity_types.go`
- **File**: `pkg/manager/activity.go`
- **Severity**: Medium
- **Category**: Missing Test Coverage
- **Description**: Activity tracking and timeline features have no test coverage.
- **Fix**: Add integration tests with in-memory SQLite.

### TEST-04: No tests for `pkg/manager/tv_details.go`
- **File**: `pkg/manager/tv_details.go`
- **Severity**: Medium
- **Category**: Missing Test Coverage
- **Description**: TV detail fetching and response building have no tests.
- **Fix**: Add tests with mock TMDB client.

### TEST-05: No tests for `pkg/indexer/` (api, factory, prowlarr)
- **File**: `pkg/indexer/prowlarr.go`, `pkg/indexer/factory.go`
- **Severity**: High
- **Category**: Missing Test Coverage
- **Description**: The indexer integration layer has zero tests. This includes Prowlarr API interaction and indexer factory logic.
- **Impact**: Indexer connection failures and parsing errors go undetected.
- **Fix**: Add tests with mock HTTP client.

### TEST-06: No tests for `pkg/storage/sqlite/activity.go`
- **File**: `pkg/storage/sqlite/activity.go`
- **Severity**: Medium
- **Category**: Missing Test Coverage
- **Description**: Activity queries (active downloads, failures, timeline, transition history) have no tests despite having complex SQL with joins.
- **Fix**: Add integration tests with in-memory SQLite database.

### TEST-07: No tests for `pkg/storage/sqlite/movie_file.go`
- **File**: `pkg/storage/sqlite/movie_file.go`
- **Severity**: Medium
- **Category**: Missing Test Coverage
- **Description**: Movie file CRUD operations have no direct tests (tested indirectly through manager tests).
- **Fix**: Add focused storage-level tests.

### TEST-08: No server tests for most endpoints
- **File**: `server/server.go`
- **Severity**: High
- **Category**: Missing Test Coverage
- **Description**: The existing `server/server_test.go` only has 1 test (health check). The other ~40+ endpoints have no HTTP-level tests.
- **Impact**: Route regressions, handler panics, and serialization errors go undetected.
- **Fix**: Add integration tests for critical endpoints (add/delete movie, add/delete series, job management).

---

## 6. Architecture & Design

### ARCH-01: `MediaManager` is a god object (130+ methods)
- **File**: `pkg/manager/manager.go` + all manager files
- **Lines**: ~17,000 lines total in pkg/manager/
- **Severity**: High
- **Category**: Architecture
- **Description**: `MediaManager` handles: movies, series, indexing, reconciliation, metadata, downloads, quality, search, jobs, indexer sources, activity, config — everything. It has 130+ methods across 15 files. The single struct has 8 dependencies.
- **Impact**: Hard to test individual concerns. Changes to series reconcile risk breaking movie reconcile. High cognitive load.
- **Fix**: Split into domain-focused services: `MovieService`, `SeriesService`, `IndexerService`, `MetadataService`, `DownloadService`, `JobService`. `MediaManager` becomes a thin facade. This can be done incrementally by extracting one service at a time.

### ARCH-02: `server/server.go` is a 1471-line monolith
- **File**: `server/server.go`
- **Severity**: Medium
- **Category**: Architecture
- **Description**: All HTTP handlers, routing setup, and response helpers live in one file. The `Serve()` method alone is ~60 lines of route registration.
- **Impact**: Hard to find specific handlers. Merge conflicts. Slow code review.
- **Fix**: Split into files by domain: `server/movie_handlers.go`, `server/series_handlers.go`, `server/indexer_handlers.go`, `server/download_handlers.go`, `server/quality_handlers.go`, `server/job_handlers.go`, `server/routes.go`.

### ARCH-04: No structured request validation
- **File**: `server/server.go`, `pkg/manager/*.go`
- **Severity**: Medium
- **Category**: Design
- **Description**: Request validation is ad-hoc and inconsistent. Some handlers check for empty strings, some don't. `AddMovieRequest` has no validation tags. `AddSeriesRequest` has no validation. Invalid IDs are only caught by parse failures.
- **Impact**: Invalid data can enter the system. Inconsistent error messages.
- **Fix**: Add a validation layer (e.g., `go-playground/validator` or manual `Validate()` methods on request types).

### ARCH-05: Magic numbers throughout
- **File**: `pkg/manager/scheduler.go:401`, `pkg/download/transmission.go:258`
- **Severity**: Low
- **Category**: Design
- **Description**: Magic numbers like `30 * time.Second` (scheduler cancel timeout), `4` and `5` and `6` (Transmission torrent status codes), `100.0` (percent done comparison), `>> 20` (bytes to MB conversion) are unexplained constants.
- **Impact**: Hard to understand intent. Maintenance burden.
- **Fix**: Extract named constants with descriptive names.

### ARCH-06: `Storage` interface is massive (60+ methods)
- **File**: `pkg/storage/storage.go`
- **Lines**: Throughout
- **Severity**: Medium
- **Category**: Architecture
- **Description**: The `Storage` interface combines `IndexerStorage`, `IndexerSourceStorage`, `QualityStorage`, `MovieStorage`, `MovieMetadataStorage`, `DownloadClientStorage`, `JobStorage`, `SeriesStorage`, `SeriesMetadataStorage`, `StatisticsStorage`, and `ActivityStorage` into one mega-interface. Any consumer only needs a subset.
- **Impact**: Hard to mock for testing. Any storage change requires updating all consumers. Violates Interface Segregation Principle.
- **Fix**: Have consumers depend on the specific sub-interfaces they need (e.g., `MovieStorage`, `JobStorage`).

---

## 7. HTTP Server Issues

### HTTP-01: `Serve()` hangs if server fails to bind port
- **File**: `server/server.go`
- **Lines**: 188–215
- **Severity**: Medium
- **Category**: Reliability
- **Description**: `Serve()` starts the HTTP server in a goroutine, then blocks on `signal.Notify` waiting for SIGINT. If `ListenAndServe` fails immediately (e.g., port in use), the error is only logged — the main goroutine continues to `<-c` and **blocks indefinitely**. The process becomes a zombie: alive but serving nothing.
- **Fix**: Use `srv.ListenAndServe()` directly (it blocks), or use an `errgroup` to propagate startup errors to the main goroutine.

### HTTP-03: DELETE endpoints accept body parameters inconsistently
- **File**: `server/server.go`
- **Lines**: 361–383 (DeleteIndexer reads body), 508–524 (DeleteQualityDefinition reads body)
- **Severity**: Low
- **Category**: API Design
- **Description**: Some DELETE endpoints read the ID from the URL (DeleteMovie, DeleteSeries), while others read it from the request body (DeleteIndexer, DeleteQualityDefinition). This is inconsistent and confusing for API consumers.
- **Impact**: API consumers must learn different patterns for similar operations.
- **Fix**: Standardize on URL parameter for DELETE (RESTful convention).

### HTTP-04: `ListTVShows` sets content-type header before `writeResponse`
- **File**: `server/server.go`
- **Lines**: 285–298
- **Severity**: Low
- **Category**: Bug
- **Description**: `ListTVShows` manually sets `w.Header().Set("content-type", "application/json")` on line 287, then calls `s.writeResponse` which also sets the same header on line 67. While `Header.Set` is idempotent, this is inconsistent with all other handlers.
- **Fix**: Remove the manual header set — `writeResponse` handles it.

---

## 8. Naming & Documentation

### NAME-01: Inconsistent method naming patterns
- **File**: `pkg/manager/manager.go`
- **Severity**: Low
- **Category**: Naming
- **Description**: Mix of naming styles:
  - `SearchMovie` vs `GetMovieDetailByTMDBID` (verb+noun vs verb+noun+preposition)
  - `listIndexersInternal` (unexported with "Internal" suffix) vs `ListIndexers` (exported)
  - `FromMediaDetails` vs `FromSeriesDetails` (should be `fromTMDBMediaDetails` or similar to clarify they're conversion functions)
  - `MOVIE_CATEGORIES` / `TV_CATEGORIES` (should be `movieCategories` / `tvCategories` per Go conventions)
- **Fix**: Standardize naming. Use Go conventions for constants (`movieCategories` not `MOVIE_CATEGORIES`).

### NAME-02: Exported function `ptr` is too generic
- **File**: `pkg/manager/manager.go`
- **Lines**: 1096
- **Severity**: Low
- **Category**: Naming
- **Description**: `ptr[A any](thing A) *A` is exported but has a completely generic name. In Go, helpers like this are usually unexported.
- **Fix**: Rename to a more descriptive name or unexport it.

### NAME-03: Mixed logging styles throughout
- **File**: Throughout `pkg/manager/`
- **Severity**: Low
- **Category**: Consistency
- **Description**: Some code uses structured logging (`log.Error("msg", zap.Error(err))`), some uses `log.Errorw("msg", "key", value)`, and some uses `log.Errorf("msg: %v", err)`. These are all valid zap methods but mixing them makes the code harder to search and parse.
- **Fix**: Standardize on one style. Prefer structured logging with `zap.Field` (`log.Error("msg", zap.Error(err))`) for consistency.

### NAME-04: `TODO` comments scattered without tracking
- **File**: Multiple files
- **Severity**: Low
- **Category**: Documentation
- **Description**: TODOs exist without issue tracking:
  - `pkg/manager/manager.go:652` — "TODO: make sure it's actually relative"
  - `pkg/manager/manager.go:660` — "TODO: check status of movie before doing anything else"
  - `pkg/manager/movie_reconcile.go:36,37` — "TODO: these are specific per indexer"
  - `pkg/manager/movie_reconcile.go:263` — "TODO: should this update state?"
  - `pkg/storage/storage.go:173` — "TODO: do we cascade associated items?"
- **Fix**: Create issues for actionable TODOs. Remove stale TODOs.

---

## Removed Items

The following were investigated and found to be non-issues or false positives:

| ID | Original Claim | Why Removed |
|----|---------------|-------------|
| BUG-02 | `log.Errorf` with `%w` format verb | `%w` behaves identically to `%v` in `fmt.Sprintf` (used internally by zap's `Errorf`). These are log statements, not error returns — `errors.Is` is irrelevant. Zero functional impact. |
| BUG-04 | Value receiver on `MediaManager` copies mutexes | All fields are reference types (interfaces, pointers). The shallow copy shares the same underlying data. `config.Config` is the only value type field and is read-only. No mutex values exist in the struct — mutexes are inside pointed-to structs. Fix for consistency, but not a bug. |
| ERR-01 | `parseMediaResult` doesn't close response body | The caller correctly defers `Body.Close()` before calling the function. This is standard idiomatic Go HTTP handling. |
| PERF-02 | `getEpisodeFileByID` full table scan | Merged into BUG-07 (same issue). |
| CONC-01 | `ReconcileSnapshot` returns shared slices | The snapshot is created once, never modified after construction, and used sequentially within a reconcile cycle. No concurrent access occurs. The mutex is defensive but unnecessary. |
| CONC-02 | `Scheduler.runningJobs` stale cancel functions | Even the original report acknowledges this is "mostly fine." The polling loop with timeout handles the race correctly. |
| ARCH-03 | `ReconcileSnapshot` exposes internal fields | Same as CONC-01 — the snapshot is immutable after creation. No caller modifies the returned slices. |
| HTTP-02 | CORS allows all origins with credentials | The configuration is technically invalid per the CORS spec, but for a self-hosted media manager where the frontend is served from the same origin, this has no practical impact. Browsers reject the `*` + credentials combo, which means cross-origin credentialed requests fail safely. |

---

## Priority Matrix

### Immediate (Do First)
| ID | Summary | Effort |
|----|---------|--------|
| BUG-06 | `findMatchingSeriesResult` panics on empty results | Small |
| BUG-01 | Double Body.Close() | Small |
| BUG-07 | Full table scan for episode file | Small |
| PERF-01 | O(n×m) movie indexing | Small |
| HTTP-01 | Server hangs on bind failure | Small |
| ERR-04 | Refresh fails on existing metadata | Small |
| DUP-01 | Episode result builder duplication | Small |

### Short Term
| ID | Summary | Effort |
|----|---------|--------|
| BUG-03 | Swallowed reconcile errors | Small |
| BUG-05 | Copy-pasted log messages | Small |
| ERR-03 | Inconsistent HTTP error format | Medium |
| PERF-04 | N+1 queries in reconcile | Medium |
| PERF-05 | N+1 metadata lookups | Medium |
| TEST-01 | Indexer source tests | Medium |
| TEST-02 | Download tests | Medium |
| TEST-05 | Indexer integration tests | Medium |
| TEST-08 | Server endpoint tests | Large |

### Long Term (Refactoring)
| ID | Summary | Effort |
|----|---------|--------|
| ARCH-01 | God object MediaManager | Large |
| ARCH-06 | Massive Storage interface | Medium |
| ARCH-02 | Server monolith file | Medium |
| ARCH-04 | No request validation | Medium |
| DUP-02 | ListMovies/ListShows duplication | Medium |
| DUP-03 | AddMovie/AddSeries duplication | Medium |
| DUP-04 | Server handler boilerplate | Large |
| DUP-05 | State evaluator duplication | Medium |
| PERF-03 | Global SQLite mutex | Medium |
| NAME-01 | Inconsistent naming | Medium |
| NAME-03 | Mixed logging styles | Medium |
