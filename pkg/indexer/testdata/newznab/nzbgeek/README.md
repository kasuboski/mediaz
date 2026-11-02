# NZBgeek upstream Newznab fixtures

Captured 2026-11-02 through Prowlarr 2.7.0.5723 and mitmproxy 12.2.1, from one configured NZBgeek indexer. These are **derived, sanitized upstream provider responses**, not Prowlarr's rewritten XML/REST output. No NZB content or actual downloads are included. They are inputs for a future native adapter, not evidence that such an adapter exists yet.

## Corpus

| Scenario | Request | Items / reported total | Purpose |
|---|---|---:|---|
| `caps` | `t=caps` | — | Optional parameters, categories, max/default page size 100; no advertised TMDB support |
| `movie-text` | `t=search`, movie categories, title query | 67 / 67 | Text baseline; all items have IMDb metadata |
| `movie-imdb` | `t=movie`, IMDb ID | 69 / 69 | Contains all 67 text releases plus 2 others |
| `tv-text-mixed-metadata` | `t=search`, TV categories, broad title query | 100 / 3,284 | First page only; TVDB/season/episode present on 59 of 100 items |
| `tv-text` | `t=search`, TV categories, specific title query | 100 / 120 | Specific-series baseline, metadata present on all 100 items |
| `tv-series` | `t=tvsearch`, TVDB ID | 100 / 120 | Same first-page GUID set as `tv-text` |
| `tv-season` | `t=tvsearch`, TVDB ID + season | 30 / 30 | Same 30 releases as metadata-filtering the original TV first page |
| `tv-episode` | `t=tvsearch`, TVDB ID + season + episode | 2 / 2 | Same 2 releases as metadata-filtering the season response |
| `tv-page-two` | Original specific TV text query, offset 100 | 20 / 120 | Disjoint from `tv-text`; combined set has 120 releases |

The text and ID requests were not simultaneous. The two additional movie releases are an observed difference, not proof that ID search always improves recall. The broad mixed-metadata query is a different workload from the specific-series query. These examples do not establish completeness for all titles or providers.

## Contract

- `manifest.json`: format version 1, provider/protocol, capture source, Prowlarr version, scenario directory names.
- `request.json`: canonical endpoint and **ordered query key/value pairs**, preserving duplicate parameters if present. Fake key: `TEST_API_KEY`.
- `response.xml`: complete sanitized response page, retaining namespaces, item ordering, repeated attributes and upstream `offset`/`total`.
- `scenario.json`: HTTP status/content type, capture date, and expected root/item/pagination/per-item attribute counts. Attribute counts count each name once per item, not every occurrence.

The structural expectations were generated from the sanitized XML and reviewed against the original capture analysis. They are integrity baselines, not independently specified expected domain-model structs. `fixture_contract_test.go` validates the corpus offline and checks the cross-scenario comparisons. A future adapter must add independent normalized-release assertions rather than treating generated counts as a complete correctness oracle.

## Preserved quirks

- `imdb` attributes can contain extra leading zeros.
- `season` / `episode` use `S02` / `E01`, not always plain numbers.
- `runtime` can be formatted as `127 min`.
- Category, language and subtitle attributes can repeat; do not flatten them indiscriminately into a single-value map.
- Metadata IDs are optional. Missing IDs in the broad TV sample are intentional.
- Release identity differs from the actionable download location. Sanitized GUID/download-ID replacements are shared across all nine scenarios; identical source URLs retain equality where relevant.

## Sanitization and provenance

Generated together using `scripts/prowlarr-capture/export.sh`. Raw captures and the private replacement mapping are not committed. No original capture paths, API keys, cookies, account settings, or user contact information are included.

Provider GUIDs and download identifiers are synthetic. Contact/channel descriptions, embedded HTML, plot/cast/director/tagline text, and image links are replaced. Hosts, endpoint paths and download URLs are canonical `.invalid` destinations. The Newznab/Atom namespace URLs remain unchanged because they identify schemas. Public release titles, metadata IDs, sizes, dates, category values and technical attributes remain for realistic parsing/identity tests.

All pages are complete; the exporter does not trim result sets while leaving misleading totals. The broad-query fixture deliberately represents only page zero of a much larger result set. HTTP headers are reduced to status/content type, and HTTP compression has been decoded. These are not byte-for-byte recordings.

To extend or refresh the corpus, export related flows as one batch to a **new private output directory**, review the generated files, update the structural/comparison expectations intentionally, and scan changed staged files with `scripts/prowlarr-capture/scan-staged.sh`. Synthetic error fixtures, when added, must be labeled separately from captured responses. See the capture lab README for the collection and review workflow.
