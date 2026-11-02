# Private Prowlarr recording lab

Runs Prowlarr 2.7.0.5723 and mitmproxy 12.2.1 with digest-pinned images. No Mediaz or download clients are required, and nothing is downloaded. Configure your indexers in Prowlarr, then test/search them to generate recordings.

## Start

Requires Docker with the Compose v2 plugin, Node.js >= 18 (built-in `fetch`), curl, and Bash. Nothing else is installed; there are no additional package dependencies.

```bash
./scripts/prowlarr-capture/start.sh
```

The helper:

- checks prerequisites and validates `compose.yaml` before starting anything;
- creates the private `.state/` directories;
- starts mitmproxy to generate its CA, waits (bounded, with timeouts) for it;
- starts Prowlarr and waits for its API;
- configures Prowlarr's global HTTP proxy using its local API key without printing it;
- never runs `sudo` and never touches Tailscale; remote exposure is a separate manual opt-in (below).

Re-running the helper preserves configuration and indexers and re-applies the recording proxy settings. All failures print an actionable message, including which `docker compose logs` command to run.

- Local UI: `http://127.0.0.1:9696` (loopback only). Override the host-side port with `PROWLARR_PORT=<port> ./scripts/prowlarr-capture/start.sh`; it stays bound to `127.0.0.1` in all cases.
- Recorder: `mitmproxy:8080` on the isolated Compose network only; it has no published host port or web UI.

## Optional tailnet exposure (explicit opt-in)

The UI is loopback-only by default. If you want to reach it from another device on your tailnet, publish it yourself with Tailscale Serve; nothing in this directory enables, requires, or removes it automatically.

1. Check your existing Serve routes first, then pick a HTTPS port that is not already used so this adds one route without changing any existing route:

   ```bash
   tailscale serve status
   sudo tailscale serve --bg --https=<choose-unused-port> http://127.0.0.1:<PROWLARR_PORT>
   ```

2. This is Tailscale **Serve** (tailnet-only), not public Funnel. Reachability is governed by your tailnet access policy. Do not put the recorder or raw captures behind Serve.

3. To later remove only this route (other existing routes are untouched):

   ```bash
   sudo tailscale serve --https=<chosen-port> off
   ```

## Configure indexers

1. Open Prowlarr and configure its requested authentication/login settings.
2. Add your providers and enter credentials directly in the UI, not in Git, chat, or shell history.
3. Use the provider's normal HTTPS URL: the global proxy is already configured; no endpoint substitution is necessary.
4. Test the indexer and use Prowlarr Search to generate traffic. Start with a Newznab provider and a direct Torznab provider where available.

Under Settings → General (advanced settings), preserve:

- Proxy enabled, HTTP, hostname `mitmproxy`, port `8080`.
- Certificate validation enabled.
- Local addresses bypass the proxy.

Prowlarr trusts the recorder's **public** CA certificate inside its container through a startup hook; only `mitmproxy-ca-cert.pem` is copied into the Prowlarr container, never the signing key. The host/browser trust store is unchanged. mitmproxy still validates upstream TLS certificates.

The global proxy captures outbound HTTP(S) traffic Prowlarr sends through it, including non-indexer traffic such as update checks. Local-address bypasses and requests made by other services (for example a separate browser solver or downloader) are not captured. This is not a network-enforced egress interception setup.

## Private state and captures

All runtime state is under the Git-ignored, permission-restricted `.state/` directory beside this README:

```text
.state/
  prowlarr/  # Database, API key, application settings and logs
  mitm/      # Recorder CA and private signing key
  captures/  # flows-<UTC timestamp>-<random>-<pid>.mitm (raw, private)
  export/    # Sanitized fixtures awaiting your manual review
```

Each recorder process start creates a new flow file whose name includes a UTC timestamp, a random suffix, and the PID, so recorder restarts cannot collide on or overwrite one file. Do not delete `.state/mitm` while containers are running: Prowlarr's mounted CA must correspond to the recorder's signing key. Run the start helper when initializing a fresh state directory, rather than starting both services simultaneously.

**Raw flow files are sensitive.** They contain API keys, cookies, search terms, response bodies, and potentially tracker passkeys and signed URLs. Application logs may also contain credentials. Keep `.state/` private and Git-ignored, do not commit or share raw state, do not expose the captures with Tailscale Serve, and do not print raw traffic into shared logs. Disk usage is not capped for captures; monitor it and stop recording when finished.

Inspect a copied flow file locally with mitmproxy tooling, e.g. `mitmweb -r <file.mitm>`, without making the inspection UI public.

## Sanitized fixture export

`./scripts/prowlarr-capture/export.sh` runs a containerized exporter against the raw captures so the host never needs mitmproxy tooling installed:

- `--list` prints safe flow indices and metadata only — never raw bodies, credentials, or key-bearing URLs.
- `--select <scenario>=<INDEX> ...` (for example `--select caps=INDEX --select movie-text=INDEX`) writes sanitized fixtures for the chosen flows to `/fixtures` in the container, which maps to `.state/export/` on the host. Review the output there, then copy it out manually only after validation.
- `--test` runs the exporter's Python test suite.

`--host` is required and must be the **upstream provider**, not Prowlarr. For example (replace the example indices with entries from your own listing):

```bash
./scripts/prowlarr-capture/export.sh --host api.nzbgeek.info --list
./scripts/prowlarr-capture/export.sh --host api.nzbgeek.info \
  --provider nzbgeek --protocol newznab --prowlarr-version 2.7.0.5723 \
  --select caps=10 --select movie-text=17 --output /fixtures/nzbgeek-review
./scripts/prowlarr-capture/export.sh --test
```

Output must be a **new subdirectory** of `/fixtures` (the host's `.state/export/`), never `/fixtures` itself. Existing output is not overwritten. Export all related scenarios together: the shared replacement map preserves identities and URL equality across pages and search variants. Indices are global flow indices, stable for append-only recordings ordered by filename; use a complete archived recording if an active write produces a read error. The exporter reads recordings only and makes no provider requests. Its container has no network, no CA key/database mounts, and read-only input captures.

Unknown XML fields/parameters or malformed data **abort the entire batch before output publication**; they are not silently skipped. This conservative allowlist must be reviewed/extended for a provider with different fields. The exporter supports successful UTF-8 XML caps/search responses with an XML media type, not HTML scrapers, binary downloads, DTD/entities, other encodings, or arbitrary error bodies. `--protocol torznab` labels compatible torrent XML exports, but live torrent coverage is not yet validated.

### Sanitization contract

- Request endpoint paths/hosts are canonicalized to `https://indexer.example.invalid/api`; only allowlisted parameters are retained. Headers are not exported.
- API keys become `TEST_API_KEY`; GUIDs/download IDs get shared synthetic identities. All content URLs become `.invalid` targets; torrent infohashes/magnets are synthetic, not usable downloads.
- Namespace identifiers stay unchanged. Repeated attributes, public titles/metadata IDs, sizes, dates, IMDb padding, season/episode encodings, result order and complete pagination metadata are preserved.
- Contact details, channel metadata, descriptions/embedded HTML, plots/cast/director/taglines and image URLs are replaced. HTTP compression is decoded by mitmproxy; response headers are reduced to status and the upstream XML media type (without parameters). These are derived fixtures, not byte-identical wire archives.
- Known request credentials and common encodings are checked for residual leakage before any files are published. This does not prove the absence of every possible secret or personal detail: manually review public titles/query text and scan the staged snapshot too.

Each scenario has `request.json` (ordered query pairs), `response.xml` (upstream XML), and `scenario.json` (status/date/expected structural counts). The provider `manifest.json` lists scenarios and provenance. See `pkg/indexer/testdata/newznab/nzbgeek/README.md` for the existing corpus and comparison invariants.

### Review and TruffleHog scan

After reviewing private exports, copy only the selected sanitized fixture directories into the appropriate `pkg/indexer/testdata/` location. Stage only reviewed files, then run:

```bash
./scripts/prowlarr-capture/scan-staged.sh
```

This scans an isolated copy of **changed staged blobs**, not the working directory, Git history, `.state/`, or untracked files. It runs digest-pinned TruffleHog 3.99.7 with `--no-verification` and container networking disabled. No credentials are sent to providers for verification. Findings include unverified/filtered candidates; the script fails on findings or scanner errors. Only detector counts are printed; potentially sensitive scanner output stays in ignored `.state/trufflehog-*` files. Do not publish those logs. A clean scan is defense in depth, not a replacement for review or corpus validation.

The torrent collector should keep raw flows private, capture caps plus text/structured searches and pagination where supported, and use the same request/response/scenario contract. No actual torrent/NZB files or raw tracker announce/passkey URLs belong in the repository.

## Stop and resume

From the repository root:

```bash
# Stop both services, preserving all configuration and recordings in .state/.
./scripts/prowlarr-capture/stop.sh

# If (and only if) you opted into a Serve route above, remove just that route:
sudo tailscale serve --https=<chosen-port> off
```

Resume with `./scripts/prowlarr-capture/start.sh` from the repository root and, if desired, re-publish the optional Serve route yourself. `docker compose down` also preserves the bind-mounted `.state/` data. Deleting `.state/` destroys the saved configuration and recorder CA.

## Initial verification

- Prowlarr's saved proxy settings were read back through its API; TLS validation remained enabled.
- A curl request from the Prowlarr container through mitmproxy returned HTTPS 200 from `example.com`.
- An unsaved Prowlarr indexer test sent `t=caps` to `https://example.com/api` using a dummy key. The expected 404 was captured with Prowlarr's user agent and a decrypted response body. This proves Prowlarr itself uses the proxy, not just curl.
- The initial capture therefore contains synthetic `example.com` verification flows; no provider was saved and no real provider credentials were used.

Provider interoperability and successful search/grab capture still require your configured providers. This setup does not download media.
