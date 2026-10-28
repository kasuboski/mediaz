# Pagination delivery — issue #46

## Delivered

### Shared bounded pagination

Paginated endpoints default to page 1 and page size 50, with a maximum page size of 100. Invalid/malformed values, explicit zero or negative values, duplicate scalar pagination parameters, and offset overflow are rejected. Beyond-end requests return an empty page with accurate totals. Internal complete-list operations remain unbounded.

### Library

`GET /api/v1/library/movies` and `GET /api/v1/library/tv` return `{ items, pagination, byState }` inside the standard response wrapper. Repeated `state` query values are ORed and duplicates deduplicated. `totalItems` reflects the selected states; `byState` aggregates metadata-eligible rows across states. Metadata eligibility and state filtering are applied before page selection. Ordering is title ascending, then entity ID ascending. The frontend serializes page, page size, and state filter in the URL, and corrects out-of-range pages when totals are known. Library page-size preference is URL-only (not a persisted preference).

### Configuration catalogs

The following GET endpoints return `{ items, pagination }` in the response envelope: indexers, indexer sources, download clients, quality definitions, and quality profiles. These are small in-memory catalogs: production loads the collection, applies applicable filtering and stable ordering, then slices it; pagination is not SQL-backed. Download clients order by host then ID. Definitions and profiles order by name then ID. Definitions accept `type=movie|series` (series maps to episode definitions; omitted means movie). Profiles accept media type filtering. Frontend helpers needing a complete catalog traverse bounded pages of up to 100; the old unbounded array HTTP contract is not retained.

### Discovery

Movie and TV discovery accept a positive 32-bit `page`, default 1, and forward it to TMDB. TMDB's page size (20) and upstream metadata are retained; this is upstream paging rather than local pagination. The frontend URL carries query, page, and selected media type; page controls operate on the selected result type, while the combined view displays movie and TV results.

### Jobs and activity timeline

Jobs has bounded pagination and keeps its existing jobs/count/pagination envelope. Activity timeline keeps its existing timeline/count response shape and is paginated as described above. Existing timeline item SQL and DATE grouping were already correct and were not changed as part of this delivery.

### Activity active downloads, failures, and entity history

`GET /activity/active` returns movies, series, and jobs arrays, global counts (including `total`), and per-category pagination metadata. `moviesPage`, `seriesPage`, and `jobsPage` independently select each category's page; shared `page` is the fallback, and shared `pageSize` defaults to 50 (maximum 100). Pagination is bounded per group, not combined: at most 150 rows at defaults or 300 at maximum size. Moving one category's page leaves the others unchanged.

`GET /activity/failures` defaults to the previous 24-hour window (`hours=24`) and returns a paginated failures array. Hours must be a positive integer; pagination follows the shared page/pageSize rules.

`GET /activity/history/{entityType}/{entityId}` paginates history for movie, series, season, episode, and job. Its response includes entity, history, and pagination; empty results use an empty array, while a beyond-end page preserves the entity and total. The SQLite queries calculate `LEAD` across full history before limiting rows, so durations remain correct at page boundaries. Movie and season metadata remain present.

Internal complete-list operations remain unbounded; this does not create an unbounded legacy HTTP contract or require a migration. Unused internal helpers are not public API routes. The frontend uses active global total and failure pagination total for badges, keys active requests by all category pages, and prefix-invalidates on retries. A history API hook exists, but no history UI consumes it yet.

## Validation

Frontend validation: 44 tests passed and typecheck passed. Mock-backed browser checks passed for independent active-category Next navigation, failures Next, and mobile layouts. Page-size reset behavior is covered by automated tests, not verified in the browser. This used browser mocks, not a live backend; live end-to-end validation was not performed. Existing Browserslist database age and large-chunk build warnings are non-blocking.

Final frontend gate: 45 tests passed across 17 files, including the approximately 10-second real polling test; typecheck and production build passed, and `npm ls` reported no dependency problems. Build warnings for stale Browserslist data and the large JavaScript chunk are existing and non-blocking. Automated tests cover page-size reset and retry/recovery behavior; those interactions were not separately verified in the browser.

Mock-backed browser checks verified active downloads with 202 total rows rendered as pages of 50/50/12, independent Next navigation (movies to page 2 while other groups stay on page 1), failures Next to page 2, and a 390px mobile viewport without horizontal overflow. Screenshot: `/tmp/mediaz-remaining-activity-mobile.png`. Browser checks used deterministic mocks; no live backend or external service was exercised.

Real SQLite-backed HTTP tests passed for global counts and pagination/history headers across all five history types. Activity timestamps retain strict fractional cutoff behavior, boundary durations are computed across page boundaries, out-of-range pages retain entity metadata, and history metadata is preserved. The complete activity HTTP surface (active groups, failures, and five history types) is paginated; no unbounded legacy HTTP response or migration was introduced. Existing timeline SQL `GROUP BY` aggregates were already in place; the earlier concern about unbounded timeline behavior was reviewed and was not a new defect or fix.

The failures-page retry error and same-page recovery regression are fixed and covered by automated tests. Page-size reset and 500-error handling are covered by automated tests, not claimed as browser-verified.
