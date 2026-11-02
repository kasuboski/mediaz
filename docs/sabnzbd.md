# SABnzbd download client configuration

Mediaz's SABnzbd API integration is compatible with SABnzbd 5.1.3. Configure it from **Settings → Download Clients** (or the corresponding download-client form):

- **Protocol** is `http` or `https` and applies to the connection scheme.
- **Host** is the hostname or IP address; enter the **Port** separately. The dialog suggests port `8080` for a new SABnzbd client; this is a form default, not automatic port discovery or a server-side default. IPv6 literals in the legacy inline `host:port` form are accepted where unambiguous; conflicts with a separately supplied port are rejected.
- **API path** is the full path to SABnzbd's API endpoint, not a base URL. New clients default to `/api`. Existing clients with a legacy null path continue using `/sabnzbd/api`. Use `/sabnzbd/api` for legacy installations or enter the full custom path exposed by a reverse proxy, such as `/sab/api`.
- **API key** is write-only: it is not returned by the API. On edit, leave it blank to use the saved key; the client ID is supplied for connection tests so the server can reuse that key. Enter a key only to replace it.

Requests use bounded timeouts (30 seconds) and response reads are limited to 8 MiB. SABnzbd request errors are reported without exposing request URLs or query-string credentials; API keys are sent to SABnzbd as query parameters, so use HTTPS when traffic crosses an untrusted network. The API path is validated as a path, not a URL, and requests do not follow redirects.

## Compatibility and scope

The path field is nullable for existing saved clients: a null value preserves the legacy `/sabnzbd/api` behavior, while new clients use `/api`. A custom value is stored and reused on edit. This compatibility does not imply that the integration covers every SABnzbd API feature: the pre-existing history-only completion/import gap is outside this change, and full completion/import support is not claimed.

Automated validation uses derived synthetic fixtures and tests; it does not validate live provider payloads or require a live SABnzbd instance.
