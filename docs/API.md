# Mediaz HTTP API
This document is created by analyzing the code in the `server` package.

Base URL: `/`

## Health Check

### GET /healthz
- Status: 200 OK
- Response: `{ "response": "ok" }`

---

## API v1
Base path: `/api/v1`

### Movies

#### GET /library/movies
- Query: `page` (positive integer, default 1), `pageSize` (1–100, default 50), repeated `state` (OR filter).
- Valid states: `missing`, `discovered`, `unreleased`, `downloading`, `downloaded`.
- Order: title ascending, then internal entity ID ascending.
- Status: 200 OK; invalid pagination or states return 400.
- Response: `{ "response": { "items": [ LibraryMovie ], "pagination": PaginationMeta, "byState": { "state": count } } }`
- `pagination.totalItems` counts matches for the selected states; `byState` counts all metadata-eligible library items before the state filter, so tab counts remain library-wide.
- No parameters still returns a bounded page. There is no unlimited or legacy-array response mode. Beyond-end pages return empty `items` with accurate totals.

#### POST /library/movies
- Request (JSON): `{ "tmdbID": int, "qualityProfileID": int }`
- Status: 200 OK
- Response: `{ "response": Movie }`

#### GET /movie/{tmdbID}
- Path Parameter: `tmdbID` (integer)
- Status: 200 OK
- Response: `{ "response": MovieDetailResult }`

#### GET /discover/movie
- Query: `query=string`, `page` (positive 32-bit integer, default 1).
- The page is forwarded to TMDB; upstream page size and `SearchMediaResponse` metadata (including page, total pages/results) are preserved. The frontend exposes pagination and media-type selection in its URL; an unselected type shows both movie and TV results rather than changing this endpoint's response shape.
- Status: 200 OK; invalid, repeated, zero, negative, or out-of-range page values return 400.
- Response: `{ "response": SearchMediaResponse }`

---

### TV Shows

#### GET /library/tv
- Query: `page` (positive integer, default 1), `pageSize` (1–100, default 50), repeated `state` (OR filter).
- Valid states: `missing`, `discovered`, `unreleased`, `downloading`, `continuing`, `completed`.
- Order: title ascending, then internal entity ID ascending.
- Status: 200 OK; invalid pagination or states return 400.
- Response: `{ "response": { "items": [ LibraryShow ], "pagination": PaginationMeta, "byState": { "state": count } } }`
- Counts and boundary behavior follow the movie list contract. Rows without metadata are excluded before paging and counting.
- Example Available filter: `/api/v1/library/tv?page=1&pageSize=50&state=discovered&state=completed`.

#### POST /library/tv
- Request (JSON): `{ "tmdbID": int, "qualityProfileID": int }`
- Status: 200 OK
- Response: `{ "response": Series }`

#### GET /tv/{tmdbID}
- Path Parameter: `tmdbID` (integer)
- Status: 200 OK
- Response: `{ "response": TVDetailResult }` (includes seasons with episodes)

#### GET /discover/tv
- Query: `query=string`, `page` (positive 32-bit integer, default 1), with the same upstream paging/metadata behavior as movie discovery.
- Status: 200 OK; invalid page values return 400.
- Response: `{ "response": SearchMediaResponse }`

---

### Indexers

#### GET /indexers
- Query: `page` (positive integer, default 1), `pageSize` (1–100, default 50).
- Returns a bounded page in `{ "response": { "items": [...], "pagination": PaginationMeta } }`. Sorted by name, then ID. The small in-memory catalog is sorted and paged after loading; this is not SQL pagination.
- Status: 200 OK; invalid pagination returns 400.

#### POST /indexers
- Request (JSON): `Indexer { id?: int, name: string, priority: int, uri: string, apiKey?: string }`
- Status: 201 Created
- Response: `{ "response": Indexer }`

#### DELETE /indexers
- Request (JSON): `{ "id": int }`
- Status: 200 OK
- Response: `{ "response": { "id": int } }`

---

### Download Clients

#### GET /download/clients
- Query: `page` (positive integer, default 1), `pageSize` (1–100, default 50).
- Returns a bounded page in `{ "response": { "items": [...], "pagination": PaginationMeta } }`. Stable order is download-client host, then ID (not display name); paging is in memory after loading this small catalog, not in SQL.
- Status: 200 OK; invalid pagination returns 400.

#### GET /download/clients/{id}
- Path Parameter: `id` (integer)
- Status: 200 OK
- Response: `{ "response": DownloadClient }`

#### POST /download/clients
- Request (JSON): `DownloadClient { id?: int, type: string, implementation: string, scheme: string, host: string, port: int, apiKey?: string }`
- Status: 201 Created
- Response: `{ "response": DownloadClient }`

#### DELETE /download/clients/{id}
- Path Parameter: `id` (integer)
- Status: 200 OK
- Response: `{ "response": id }`

---

### Quality Definitions

#### GET /quality/definitions
- Query: `page` (positive integer, default 1), `pageSize` (1–100, default 50), optional `type=movie|series` (omitted means movie; `series` selects episode definitions).
- Returns `{ "response": { "items": [...], "pagination": PaginationMeta } }`, ordered by name then ID. The catalog is loaded, filtered, sorted, and paged in memory; this is not SQL pagination.
- Status: 200 OK; invalid pagination or type returns 400.

#### GET /quality/definitions/{id}
- Path Parameter: `id` (integer)
- Status: 200 OK
- Response: `{ "response": QualityDefinition }`

#### POST /quality/definitions
- Request (JSON): `QualityDefinition { id?: int, name: string, media_type: string, preferredSize: float, minSize: float, maxSize: float }`
- Status: 201 Created
- Response: `{ "response": QualityDefinition }`

#### DELETE /quality/definitions
- Request (JSON): `{ "id": int }`
- Status: 200 OK
- Response: `{ "response": { "id": int } }`

---

### Quality Profiles

#### GET /quality/profiles
- Query: `page` (positive integer, default 1), `pageSize` (1–100, default 50), optional `type=movie|series` to select profiles for that media type (omitted or other values currently return the unfiltered profile list).
- Returns `{ "response": { "items": [...], "pagination": PaginationMeta } }`, ordered by name then ID. The catalog is loaded, sorted, and paged in memory; this is not SQL pagination.
- Status: 200 OK; invalid pagination returns 400.

#### GET /quality/profiles/{id}
- Path Parameter: `id` (integer)
- Status: 200 OK
- Response: `{ "response": QualityProfile }`

---

### Library Configuration & Statistics

#### GET /config
- Status: 200 OK
- Response: `{ "response": ConfigSummary }`

#### GET /library/stats
- Status: 200 OK
- Response: `{ "response": LibraryStats }`

---

### Pagination on jobs and activity

`GET /jobs`, `GET /activity/timeline`, both library lists, the activity active-downloads/failures/entity-history endpoints, and the paginated configuration catalogs use bounded pagination: page defaults to 1, page size to 50, maximum size is 100. Explicit zero, negative, malformed, oversized, duplicate scalar pagination values, and offset overflow return 400. Supplying only `page` still uses size 50. Beyond-end pages return empty arrays with accurate totals.

The five configuration catalogs are `GET /indexers`, `/indexer-sources`, `/download/clients`, `/quality/definitions`, and `/quality/profiles`. They return `{ items, pagination }` inside the standard `response` wrapper. They are sorted/paged in memory after loading small catalogs, not paged in SQL. Definition `type=movie|series` filters (series maps to episode definitions); profiles support the corresponding type filter. Frontend helpers that need complete catalogs traverse the bounded endpoints in pages of up to 100; this does not restore an unbounded legacy HTTP response.

Discovery endpoints accept a positive 32-bit `page` (default 1), forward it to TMDB, and retain TMDB's fixed page size of 20 and upstream result metadata. This is upstream paging, not local pagination. Frontend discovery preserves its query, page, and selected media-type state in the URL; the combined view presents movie and TV results separately and page navigation follows the selected type.

Jobs keeps its jobs/count/pagination envelope. Activity timeline keeps its existing timeline/count response shape and is paginated as described above. Its existing SQL and DATE grouping are unchanged.

`GET /activity/active` returns `movies`, `series`, `jobs`, `counts` (`movies`, `series`, `jobs`, and global `total`), and a `pagination` object containing separate metadata for each category. It accepts independent `moviesPage`, `seriesPage`, and `jobsPage` values. The shared `page` is the fallback for any category without its named page; `pageSize` is shared. Each category is bounded separately (up to 100 each, not 100 combined), so the defaults return at most 150 rows and maximum sizes at most 300. Changing one category's page does not change the others.

`GET /activity/failures` returns `failures` and `pagination`; `hours` defaults to 24 and must be a positive integer. `page`/`pageSize` apply to the failures list. `GET /activity/history/{entityType}/{entityId}` supports `movie`, `series`, `season`, `episode`, and `job`; it returns `entity`, `history`, and `pagination`, including an empty history array when there are no transitions. A page beyond the end retains the entity and accurate total. History durations are computed against the full ordered history (including the next transition across page boundaries), and movie/season metadata is preserved.

The frontend activity polling uses `counts.total` and failure pagination totals for badges; its cache keys include each active category's page, while retry invalidation covers the relevant key prefixes. The full remaining Activity HTTP surface is paginated: active groups, failures, and history for all five supported types. History uses a strict fractional timestamp cutoff, calculates durations across page boundaries, preserves entity metadata on beyond-end pages, and retains movie/season metadata. Internal storage/service complete-list helpers are not themselves public paginated HTTP contracts. No legacy unbounded HTTP response or database migration is part of this pagination change. Timeline SQL `GROUP BY` aggregates were already present and were not changed.

---

## Schemas

### PaginationMeta
- `page`: requested one-based page number
- `pageSize`: positive page size (at most 100)
- `totalItems`: all matching list-eligible items, independent of page size
- `totalPages`: ceiling of totalItems/pageSize, or 0 for no matches

### LibraryMovie
- `path`: string
- `tmdbID`: int
- `title`: string
- `poster_path`: string
- `year?`: int
- `state`: string

### Movie
- `ID`: int
- `path?`: string
- `monitored`: int
- `qualityProfileID`: int
- `movieFileID?`: int
- `movieMetadataID?`: int
- `state`: string

### Series
- `ID`: int
- `path?`: string
- `monitored`: int
- `qualityProfileID`: int
- `tmdbID`: int
- `seriesMetadataID?`: int
- `state`: string

### LibraryShow
- `path`: string
- `tmdbID`: int
- `title`: string
- `poster_path`: string
- `year?`: int
- `state`: string

### MovieDetailResult
- `tmdbID`: int
- `imdbID?`: string
- `title`: string
- `originalTitle?`: string
- `overview?`: string
- `posterPath`: string
- `backdropPath?`: string
- `releaseDate?`: string
- `year?`: int
- `runtime?`: int
- `adult?`: bool
- `voteAverage?`: float
- `voteCount?`: int
- `popularity?`: float
- `genres?`: string
- `studio?`: string
- `website?`: string
- `collectionTmdbID?`: int
- `collectionTitle?`: string
- `libraryStatus`: string
- `path?`: string
- `qualityProfileID?`: int
- `monitored?`: bool

### TVDetailResult
- `tmdbID`: int
- `title`: string
- `originalTitle?`: string
- `overview?`: string
- `posterPath`: string
- `backdropPath?`: string
- `firstAirDate?`: string
- `lastAirDate?`: string
- `status?`: string
- `nextAirDate?`: string
- `originalLanguage?`: string
- `productionCountries?`: [string]
- `networks?`: [ `NetworkInfo` ]
- `seasonCount`: int
- `episodeCount`: int
- `adult?`: bool
- `voteAverage?`: float
- `voteCount?`: int
- `popularity?`: float
- `genres?`: [string]
- `libraryStatus`: string
- `path?`: string
- `qualityProfileID?`: int
- `monitored?`: bool
- `seasons?`: [ `SeasonResult` ]
- `externalIds?`: `ExternalIDs`
- `watchProviders?`: [ `WatchProvider` ]

### NetworkInfo
- `name`: string
- `logoPath?`: string

### ExternalIDs
- `imdbId?`: string
- `tvdbId?`: int

### WatchProvider
- `providerId`: int
- `name`: string
- `logoPath?`: string

### SearchMediaResponse
- `page?`: int
- `total_pages?`: int
- `total_results?`: int
- `results`: [ `SearchMediaResult` ]

### SearchMediaResult
- `adult?`: bool
- `backdrop_path?`: string
- `genre_ids?`: [int]
- `id?`: int
- `original_language?`: string
- `original_title?`: string
- `overview?`: string
- `popularity?`: float
- `poster_path?`: string
- `release_date?`: string
- `title?`: string
- `video?`: bool
- `vote_average?`: float
- `vote_count?`: int

### Indexer
- `id`: int
- `name`: string
- `priority`: int
- `uri?`: string
- `apiKey?`: string
- `status?`: object
- `categories?`: array

### DownloadClient
- `id`: int
- `type`: string
- `implementation`: string
- `scheme`: string
- `host`: string
- `port`: int
- `apiKey?`: string

### QualityDefinition
- `id`: int
- `name`: string
- `media_type`: string
- `preferredSize`: float
- `minSize`: float
- `maxSize`: float

### SeasonResult
- `tmdbID`: int
- `seriesID`: int
- `seasonNumber`: int
- `title`: string
- `overview?`: string
- `airDate?`: string
- `posterPath?`: string
- `episodeCount`: int
- `monitored`: bool
- `episodes?`: [ `EpisodeResult` ]

### EpisodeResult
- `tmdbID`: int
- `seriesID`: int
- `seasonNumber`: int
- `episodeNumber`: int
- `title`: string
- `overview?`: string
- `airDate?`: string
- `stillPath?`: string
- `runtime?`: int
- `voteAverage?`: float
- `monitored`: bool
- `downloaded`: bool

### QualityProfile
- `id`: int
- `name`: string
- `qualities`: [ `QualityDefinition` ]
- `cutoff_quality_id`: int
- `upgradeAllowed`: bool

### ConfigSummary
- `library`: `LibraryConfig`
- `server`: `ServerConfig`
- `jobs`: `JobsConfig`

### LibraryConfig
- `movieDir`: string
- `tvDir`: string
- `downloadMountDir`: string

### ServerConfig
- `port`: int

### JobsConfig
- `movieReconcile`: string
- `movieIndex`: string
- `seriesReconcile`: string
- `seriesIndex`: string

### LibraryStats
- `movies`: `MovieStats`
- `tv`: `TVStats`

### MovieStats
- `total`: int
- `byState`: object (map of state names to counts)

### TVStats
- `total`: int
- `byState`: object (map of state names to counts)