# SABnzbd

Mediaz targets SABnzbd **5.1.3** and uses its modern `/api` endpoint. Older SAB versions and custom API prefixes are not supported by this adapter.

Configure a download client with implementation `sabnzbd`, scheme (`http` or `https`), host, port (normally `8080`), and API key. Use HTTPS outside a trusted local network. The host is a hostname or IP address, not a URL; IPv6 is supported. An inline host port takes precedence over the separate port if both are set.

No database migration or additional API-path setting is needed. The client honors the configured port, decodes textual queue priorities, rejects explicit API failures, blocks redirects, limits responses to 8 MiB, and uses a 30-second HTTP timeout. Client logs and errors do not include API keys or download URLs.

Tests exercise populated Add/Get/List responses and actual HTTP endpoint selection. Real verification also passed against a disposable SABnzbd 5.1.3 instance: the factory-created client used `/api`, added and retrieved a paused synthetic job, and SAB parsed one file's metadata with textual `Normal` priority and zero progress. No NNTP servers or provider accounts were configured. The owned job was deleted, queue/history verified empty, and the disposable container/network removed.

This does not prove a complete download/import workflow: existing history-only completion tracking and completed-file discovery remain separate limitations.
