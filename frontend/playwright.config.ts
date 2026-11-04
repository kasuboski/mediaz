import { defineConfig } from '@playwright/test';

const proxyPort = process.env.MEDIAZ_E2E_PROXY_PORT ?? String(20_000 + Math.floor(Math.random() * 40_000));
process.env.MEDIAZ_E2E_PROXY_PORT = proxyPort;
const baseURL = `http://127.0.0.1:${proxyPort}`;

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  reporter: 'list',
  use: { baseURL, browserName: 'chromium', trace: 'retain-on-failure', launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH } },
  webServer: {
    command: 'node e2e/harness.mjs',
    url: `${baseURL}/api/v1/healthz`,
    reuseExistingServer: false,
    gracefulShutdown: { signal: 'SIGTERM', timeout: 10_000 },
    timeout: 180_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
