import { readFile } from 'node:fs/promises';

// Node >= 18 is required: built-in fetch plus AbortSignal.timeout.
if (typeof fetch !== 'function' || typeof AbortSignal?.timeout !== 'function') {
  throw new Error('node >= 18 is required (built-in fetch and AbortSignal.timeout)');
}

const port = Number(process.env.PROWLARR_PORT ?? 9696);
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  throw new Error(`Invalid PROWLARR_PORT: ${JSON.stringify(process.env.PROWLARR_PORT ?? '')}`);
}

// Bounded so a hung API call can never stall start.sh.
const timeoutMs = 10_000;
const fetchJson = async (url, init) => {
  const signal = AbortSignal.timeout(timeoutMs);
  const response = await fetch(url, { ...init, signal });
  if (!response.ok) {
    throw new Error(`${init?.method ?? 'GET'} ${new URL(url).pathname} failed: HTTP ${response.status}`);
  }
  return response.json();
};

// Keep the generated API key out of argv, shell tracing, and console output.
const xml = await readFile(new URL('./.state/prowlarr/config.xml', import.meta.url), 'utf8');
const apiKey = xml.match(/<ApiKey>([^<]+)<\/ApiKey>/)?.[1];
if (!apiKey) {
  throw new Error('Prowlarr has not generated its API key yet (diagnose with: docker compose logs prowlarr)');
}
const base = `http://127.0.0.1:${port}`;
const headers = { 'X-Api-Key': apiKey, 'Content-Type': 'application/json' };

let settings;
try {
  settings = await fetchJson(`${base}/api/v1/config/host`, { headers });
} catch (cause) {
  throw new Error(`Read host settings failed: ${cause.message} (diagnose with: docker compose logs prowlarr)`);
}
Object.assign(settings, {
  proxyEnabled: true,
  proxyType: 'http',
  proxyHostname: 'mitmproxy',
  proxyPort: 8080,
  proxyBypassFilter: '',
  proxyBypassLocalAddresses: true,
  certificateValidation: 'enabled',
});
try {
  await fetchJson(`${base}/api/v1/config/host/${settings.id}`, {
    method: 'PUT', headers, body: JSON.stringify(settings),
  });
} catch (cause) {
  throw new Error(`Save host settings failed: ${cause.message} (diagnose with: docker compose logs prowlarr)`);
}
console.log(`Prowlarr uses mitmproxy:8080; TLS certificate validation remains enabled.`);
