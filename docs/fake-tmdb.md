# Local fake TMDB metadata server

`internal/testsupport/faketmdb` is a deterministic metadata-only fake for tests and local UI work. All catalog entries are synthetic; no live response captures or TMDB credentials are included. Relative image paths remain in responses and images are still fetched from TMDB's CDN. The fake never forwards requests to TMDB.

Artwork fields use representative real TMDB schema-example image paths and load from TMDB's live CDN; they are borrowed public artwork and do not depict the fictional catalog entries. Titles, descriptions, and other catalog metadata remain synthetic. CDN availability can vary; the fake does not bundle or proxy artwork.

Start it with `go run ./tools/fake-tmdb` (default bind `127.0.0.1:8765`; override with `FAKE_TMDB_ADDR`). Configure Mediaz with `MEDIAZ_TMDB_URI=http://127.0.0.1:8765` and a dummy `MEDIAZ_TMDB_APIKEY`. The origin intentionally has no `/3` suffix.

Go tests can use `httptest.NewServer(faketmdb.New())`. Each instance records requests via `Requests()` and supports path-local response status overrides with `SetStatus`; no mutable scenario state is shared between instances. Supported metadata route shapes are movie/TV search, movie/TV details, TV season, external IDs and watch providers. Unsupported paths and missing IDs return 404. This does not replace the production TMDB client or defaults.
