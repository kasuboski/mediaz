# Fake TMDB investigation — issue #123

## Recommendation

Build a small **HTTP fake of the TMDB metadata endpoints Mediaz uses**, backed by a curated, versioned JSON fixture catalog. The goal is to avoid repeated metadata API traffic during tests and UI development, not to make the application fully offline. Continue loading images from TMDB's existing image URLs. Keep the real TMDB client and production defaults. Do not replace TMDB as a product dependency, mirror its catalog, or implement its full generated API.

Expose one reusable Go `http.Handler` and a thin standalone command. Go integration tests start it with `httptest.NewServer`; Playwright starts the same implementation as a process and points the real Mediaz backend at it. Share fixtures and behavior, not mutable test state.

Issue: https://github.com/kasuboski/mediaz/issues/123. Findings below are from the current worktree; no live authenticated TMDB requests were made.

## Actual API surface

All calls are GETs with Bearer authorization. Current callers do not send language or append-to-response options. **Seven route shapes**, not the issue's five, are needed:

| TMDB route | Current use |
| --- | --- |
| `/3/search/movie` | Discover, CLI search, filesystem identity matching; query and optional page |
| `/3/search/tv` | Discover and filesystem matching; query and page |
| `/3/movie/{id}` | Load/refresh movie metadata |
| `/3/tv/{id}` | Load/refresh hierarchy and live TV detail enrichment |
| `/3/tv/{id}/season/{number}` | One request per season listed by series details, including season zero |
| `/3/tv/{id}/external_ids` | Best-effort IMDb/TVDB enrichment |
| `/3/tv/{id}/watch/providers` | Best-effort US streaming-provider enrichment |

Mediaz's Discover routes call TMDB **search**, not TMDB's `/discover` endpoints. No individual-episode, credits, image-metadata, or configuration endpoint is currently needed.

Sources: [`tmdb.go`](../pkg/tmdb/tmdb.go), [`movie_service.go`](../pkg/manager/movie_service.go), [`series_service.go`](../pkg/manager/series_service.go), [`metadata_service.go`](../pkg/manager/metadata_service.go), [`cmd/movie.go`](../cmd/movie.go), [`routes.go`](../server/routes.go).

## Data flow and effective dependencies

Browser/CLI → Mediaz services → real generated HTTP client → TMDB → handwritten decoding/mapping → SQLite → Mediaz API → UI.

- **Movie metadata:** ID, title, runtime, IMDb ID, original title, overview, poster path, genre names, homepage, popularity, first production company, collection ID/name, release date/year. Movie title and runtime pointers are dereferenced unconditionally: successful fixtures must supply them. Many issue-listed fields (budget, revenue, ratings, tagline, etc.) are decoded but not persisted by this mapper.
- **TV metadata:** series ID/name, season/episode counts, first air date, poster, overview and status. Series detail responses also supply transient backdrop, next air date, original language, countries, networks/logos, genres, adult and ratings/popularity. TV detail can call TMDB even with cached metadata.
- **Seasons:** ID, name, number, overview and air date, linked to the persisted series. Season posters are decoded but not persisted by this mapper.
- **Episodes:** ID, name, number, runtime, overview, air date and still path, linked to the persisted season. Cast/crew is not meaningfully copied into persisted metadata.
- **External IDs:** only IMDb and TVDB are retained as normalized JSON, not all fields listed in the issue.
- **Providers:** only `results.US.link` and `results.US.flatrate[]` provider ID/name/logo are retained. Other countries and rent/buy categories are discarded.
- **Search:** paginated results are not persisted directly. They drive Discover and reconciliation; IDs, titles/names and dates must agree with detail fixtures.

Sources: [`metadata_service.go`](../pkg/manager/metadata_service.go) (`FromMediaDetails`, `FromSeriesDetails`, `FromSeriesSeasons`, `FromSeriesEpisodes`, provider/ID parsers), [`tmdb.go`](../pkg/tmdb/tmdb.go), [`external_data.go`](../pkg/manager/external_data.go), [`series_service.go`](../pkg/manager/series_service.go) (`GetTVSeriesDetails`), [`frontend API mapping`](../frontend/src/lib/api.ts).

## Store fixtures as JSON in Git

Suggested layout (new paths, not implemented):

```text
internal/testsupport/faketmdb/
  handler.go
  catalog.go
  fixtures/
    manifest.json
    movies/<id>.json
    tv/<id>/details.json
    tv/<id>/external_ids.json
    tv/<id>/watch_providers.json
    tv/<id>/seasons/0.json
    tv/<id>/seasons/1.json
    search/movie-page-1.json
    search/movie-page-2.json
    search/tv-page-1.json
tools/fake-tmdb/main.go
```

Use `go:embed` for a self-contained handler/executable; the command must not depend on its working directory. Prefer pretty-printed, ordinary JSON over SQLite, opaque archives, Git LFS, or giant Go literals. Raw HTTP response shapes preserve nulls, optional fields and decoding coverage that prebuilt domain structs would bypass.

Recommended catalog:

- A few richly populated movies, plus deterministic lightweight entries sufficient to cross a 20-result page boundary.
- Two small coherent TV hierarchies: an ended show and a returning show, including specials and empty/unknown dates where valid.
- Provider-present and provider-absent cases, missing images and nullable fields.
- Stable IDs shared by search/detail/seasons/episodes/external IDs; reuse the same identities in download/indexer test scenarios where appropriate.

Start with curated/synthetic fixtures; preserve a few representative full wire responses for contract realism. Prefer realistic nullable/unused fields in those samples, rather than trimming everything to today's domain model. Synthesize edge cases separately and label synthetic vs recorded provenance. Checked-in OpenAPI examples can bootstrap shape tests, but are not a coherent catalog: their IDs and titles differ across endpoints.

`manifest.json` should describe catalog/scenario version, source/provenance and capture date for recorded responses, intended behaviors and expected hierarchy counts. Capturing/refreshing should be an explicit developer command, never something tests or the fake do automatically. Sanitize auth headers, API-key query parameters and account-specific data before writing anything; review redistribution/attribution terms before committing captured metadata or artwork. No credentials or live fallback in normal tests.

## Is it big?

Not for a bounded test catalog. The following are **measured examples already in** [`pkg/tmdb/schema/tmdb.schema.json`](../pkg/tmdb/schema/tmdb.schema.json), not fresh captures, catalog-wide estimates, or coherent records for one title. Sizes are bytes after decoding the embedded example and reserializing UTF-8 JSON:

| Response example | Compact bytes | Pretty bytes | Gzip of compact bytes |
| --- | ---: | ---: | ---: |
| Movie detail | 1,797 | 2,342 | 982 |
| TV series detail | 6,915 | 8,304 | 3,095 |
| Season detail, 10 episodes with cast/crew | 99,040 | 152,340 | 13,512 |
| TV external IDs | 245 | 286 | 171 |
| TV providers, 88 regions | 29,240 | 44,277 | 3,177 |
| Movie search, 20 results | 12,937 | 15,654 | 5,049 |
| TV search, 1 result | 725 | 908 | 496 |

A small curated pack should comfortably fit below a few MB of JSON; a sub-1-MB initial target is plausible with short hierarchies and limited full cast/crew samples, but **not measured yet**. Full long-running shows multiply season payloads. Images remain on TMDB's image CDN and are not stored in the fixture pack. Do not download artwork or mirror TMDB.

Measurement recipe: load the schema with Python's `json`; select `paths[route].get.responses["200"].content["application/json"].examples[*].value`; decode that string as JSON; measure `json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()` and the `indent=2` variant; use `gzip.compress` on compact bytes.

## Fake behavior

- Implement only the seven routes, with coherent pagination and deterministic case-insensitive search over the small catalog. Support absent page as page one, explicit page two and no matches. This is not an attempt to reproduce TMDB ranking.
- Keep a small number of exact search wire fixtures for envelope/nullability regression tests; avoid duplicating all detail data in many recordings.
- Return JSON content types, realistic unknown-ID 404s and explicit errors for unsupported routes. Record unexpected requests so tests can assert on them. Never forward to real TMDB.
- Offer per-instance request recording and overrides for 401/404/429/500, malformed JSON, delays and individual season failures. Keep overrides outside the public TMDB surface and avoid global mutable scenario switches shared by parallel workers.
- Bind the standalone command locally by default; add readiness and graceful shutdown. Test configuration supplies a dummy token.

## Go and browser wiring

**Go:** use the real `tmdb.New(fake.URL, "test-token", ...)`, manager/router and in-memory SQLite. Assert full returned/persisted structs, hierarchy completeness and unexpected requests, not just HTTP status. Existing starting points: [`filesystem_discovery_integration_test.go`](../pkg/manager/filesystem_discovery_integration_test.go) and [`native_newznab_integration_test.go`](../pkg/manager/native_newznab_integration_test.go).

**Browser:** browser → real Mediaz API → real TMDB client → fake process. Configure `MEDIAZ_TMDB_URI` to the fake origin and `MEDIAZ_TMDB_APIKEY` to a dummy value. Supply the origin without `/3`; generated routes already contain that prefix. Production defaults remain untouched. Use per-worker app databases/library directories and immutable fixtures; deliberately control background jobs and wait for readiness.

Prefer testing the production frontend served by Mediaz, initially: [`vite.config.ts`](../frontend/vite.config.ts) explicitly defines the development API host. [`cmd/serve.go`](../cmd/serve.go) starts manager jobs automatically. This checkout has Vitest but no Playwright dependency/configuration, so browser orchestration is additional work, not existing infrastructure to plug into.

**Images:** keep the existing `https://image.tmdb.org/t/p/` URLs in [MediaCard](../frontend/src/components/media/MediaCard.tsx), [MovieDetail](../frontend/src/pages/MovieDetail.tsx), [TVDetail](../frontend/src/pages/TVDetail.tsx), and activity cards. Preserve real relative image paths in representative fixtures and let the browser fetch artwork normally. No image capture, image-origin setting or default Playwright interception is needed. Browser runs may require network access for artwork; ordinary workflow assertions should not depend on successful image delivery. Targeted image-error or visual snapshot tests can handle images separately if needed. Do not intercept Mediaz API routes.

Configuration sources: [`cmd/root.go`](../cmd/root.go), [`config/config.go`](../config/config.go), [`tmdb.go`](../pkg/tmdb/tmdb.go), [`client.gen.go`](../pkg/tmdb/client.gen.go).

## Relationship to the Prowlarr work

The useful precedent here is sanitized, checked-in external-provider wire fixtures and contract tests: [`pkg/indexer/testdata/newznab/nzbgeek/`](../pkg/indexer/testdata/newznab/nzbgeek/), [`fixture_contract_test.go`](../pkg/indexer/fixture_contract_test.go). The [`scripts/prowlarr-capture/`](../scripts/prowlarr-capture/) Compose lab runs real Prowlarr for capture; this checkout does not contain a reusable fake Prowlarr HTTP service. Borrow the fixture discipline, not a production replacement of TMDB.

## Risks uncovered

- TV hierarchy decoding does not consistently reject non-200 statuses and can skip failed season calls; incomplete or error-shaped fixtures may look like success. Explicitly test complete hierarchy/error behavior, with any production fix scoped separately.
- Current movie mapping requires non-null title/runtime; decoded fields are not necessarily persisted fields.
- Refresh does not uniformly refresh existing season/episode rows; do not assume hierarchy edits in the fake propagate through every refresh path.
- A fake validates our integration against pinned examples, not ongoing upstream compatibility. Keep optional live capture/smoke checks separate from offline CI.

## Suggested implementation sequence

1. Add coherent curated fixture catalog, manifest and fixture consistency tests.
2. Add embedded handler with all seven routes, pagination, strict unknown requests and per-instance overrides.
3. Add real-client + SQLite integration coverage for search → add → details, TV hierarchy and failure cases.
4. Add thin standalone command and documented local startup/configuration.
5. Add Playwright orchestration and isolated app state; leave image requests on TMDB's CDN and cover Discover → add → detail plus pagination/empty/error scenarios.
6. Add an opt-in capture/refresh tool only if maintaining hand-curated fixtures becomes inconvenient.

Investigation only: no fake, capture tool or Playwright suite has been implemented; no test suite was run.
