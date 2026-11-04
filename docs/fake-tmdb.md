# Local fake TMDB metadata server

`internal/testsupport/faketmdb` is a deterministic metadata-only fake for tests and local UI work. All catalog entries are synthetic; no live response captures or TMDB credentials are included. Relative image paths remain in responses and images are still fetched from TMDB's CDN. The fake never forwards requests to TMDB.

**Known artwork limitation:** fixtures currently use paths such as `/synthetic/movies/1001.jpg` and `/synthetic/tv/2001.jpg`; these are not representative TMDB CDN references, so screenshots can show missing artwork. A direct probe confirmed a dummy `/something.jpg` returns 404 `text/html`, while known schema-example paths `/aOyE420zuFq9zWtEWjIccAiTrzU.jpg` (`w500`) and `/zuW6fOiusv4X9nnW3paHGfXcSll.jpg` (`original`) returned 200 `image/jpeg`. No exact browser `img.src`/`naturalWidth` probe was completed, so this does not rule out browser-level timing or other delivery issues. Follow up by using real representative CDN paths with explicit synthetic provenance; do not add image mocking or metadata API fallbacks.

Start it with `go run ./tools/fake-tmdb` (default bind `127.0.0.1:8765`; override with `FAKE_TMDB_ADDR`). Configure Mediaz with `MEDIAZ_TMDB_URI=http://127.0.0.1:8765` and a dummy `MEDIAZ_TMDB_APIKEY`. The origin intentionally has no `/3` suffix.

Go tests can use `httptest.NewServer(faketmdb.New())`. Each instance records requests via `Requests()` and supports path-local response status overrides with `SetStatus`; no mutable scenario state is shared between instances. Supported metadata route shapes are movie/TV search, movie/TV details, TV season, external IDs and watch providers. Unsupported paths and missing IDs return 404. This does not replace the production TMDB client or defaults.
