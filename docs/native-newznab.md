# Native Newznab sources

Mediaz can query Newznab-compatible **Usenet** indexers directly, without Prowlarr. Configure a source in **Indexers → Add source** or through the indexer-sources REST API; source configuration is stored in Mediaz's database, not a separate environment-variable section. Choose Newznab, enter a hostname, optional port, API path, and optional API key. The path defaults to `/api`; specify another absolute path for reverse-proxy prefixes. Host must be only a hostname/IP (no scheme, path, credentials, or query).

Prowlarr sources use its built-in `/api/v1` endpoint and require an API key; custom Prowlarr paths are rejected. Newznab API keys can be omitted for providers that do not require them. API keys are never returned in source responses.

This implementation is Newznab-only: Torznab, tracker scraping, custom cookie/header authentication, and providers requiring browser/session login are unsupported. Direct downloads require releases to supply a usable download URL reachable by the configured downloader; Mediaz does not proxy or resolve details-page links. Searches are paginated only within a bounded request budget. If a later page fails, results already obtained may be returned alongside a partial-error indication; a partial result is not proof that all matches were retrieved. Requests are throttled per source. HTTP 429/503 responses and Newznab error codes 500 (request limit) / 501 (download limit) start a cooldown; when no provider retry delay is supplied, the defaults are one minute for HTTP 429/503 and five minutes for codes 500/501. Delays are capped at five minutes. While a cooldown is active, calls fail fast with a retryable error rather than sleeping; ordinary request spacing remains context-aware. The XML code mappings follow the [nZEDb Newznab API specification, §5](https://github.com/nZEDb/nZEDb/blob/master/docs/newznab_api_specification.txt). Other XML error codes remain provider errors and do not trigger quota cooldowns; no description-text heuristics are used.

## Build and run

From the repository root:

```sh
go build -o mediaz main.go
./mediaz serve
```

Configure the source in the running UI or REST API. No provider credentials or live network access are needed to build or run the server; they are needed to test a real provider.

## REST examples

Test before saving:

```sh
curl -X POST http://localhost:8080/api/v1/indexer-sources/test \
  -H 'Content-Type: application/json' \
  -d '{"name":"Example","implementation":"newznab","scheme":"https","host":"indexer.example","apiPath":"/usenet/api","apiKey":"YOUR_KEY","enabled":true}'
```

Create a source:

```sh
curl -X POST http://localhost:8080/api/v1/indexer-sources \
  -H 'Content-Type: application/json' \
  -d '{"name":"Example","implementation":"newznab","scheme":"https","host":"indexer.example","apiPath":"/usenet/api","apiKey":"YOUR_KEY","enabled":true}'
```

`GET /api/v1/indexer-sources` lists configured sources (without keys); `POST /api/v1/indexer-sources/{id}/refresh` refreshes one manually. Local development can use offline fixtures and package tests; live provider checks are separate integration tests.
