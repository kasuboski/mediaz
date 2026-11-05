# Browser smoke suite

From the repository root, run `nix develop -c npm --prefix frontend run test:e2e`. Playwright builds the production UI, starts the real Mediaz server and the shared fixture fake on dynamically reserved loopback ports, and uses an isolated temporary SQLite database/library. Processes are stopped and temporary data removed when the suite exits. Chromium can be installed with `nix develop -c npx --prefix frontend playwright install chromium`; on NixOS, provide a compatible browser using `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` only if your local Playwright setup supports that executable override. No TMDB credentials or outbound metadata requests are used; poster/backdrop references intentionally use the real image CDN.

For manual UI work, start the fake in one shell with `FAKE_TMDB_ADDR=127.0.0.1:8765 nix develop -c go run ./tools/fake-tmdb`. In another shell, build the frontend (`nix develop -c npm --prefix frontend run build`) and run Mediaz with isolated paths and explicit settings, for example:

```sh
tmpdir=$(mktemp -d)
MEDIAZ_TMDB_URI=http://127.0.0.1:8765 MEDIAZ_TMDB_APIKEY=local-test \
MEDIAZ_SERVER_PORT=8080 MEDIAZ_SERVER_DISTDIR="$PWD/frontend/dist" \
MEDIAZ_STORAGE_FILEPATH="$tmpdir/mediaz.sqlite" \
MEDIAZ_LIBRARY_MOVIE="$tmpdir/movies" MEDIAZ_LIBRARY_TV="$tmpdir/tv" \
  nix develop -c go run . serve
```

Visit `http://127.0.0.1:8080/discover` and search for **The Glass Harbor**. The fake serves metadata only; CDN image requests remain real. Remove the temporary directory after stopping Mediaz.

The Playwright suite covers the movie request smoke flow, a TV discovery/request/library-detail flow using **Lanterns at Low Tide** (including episode metadata and the Netflix provider), and unmatched/cleared searches that assert stale results disappear. It also captures successful full-page screenshots at a consistent 1440×1000 viewport: Discover search results, TV library and series detail, movie library, and Activity. Run `nix develop -c npm --prefix frontend run test:e2e`; screenshots are retained under `frontend/test-results/` in each Playwright test's output directory (and as test attachments). Activity remains honestly empty unless the real app emits events; the capture does not fabricate activity or start downloads. The harness's default quality profiles include a series profile labeled **High Definition**; the TV request test selects this by its visible label (not a movie profile).
