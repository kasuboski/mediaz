import { spawn, execFileSync } from 'node:child_process';
import http from 'node:http';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import net from 'node:net';
import { fileURLToPath } from 'node:url';

const frontend = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const root = path.resolve(frontend, '..');
let temp;
const processes = [];
const reservePort = () => new Promise((resolve, reject) => {
  const server = net.createServer();
  server.once('error', reject);
  server.listen(0, '127.0.0.1', () => { const { port } = server.address(); server.close(() => resolve(port)); });
});
const waitFor = async (url, child) => {
  const deadline = Date.now() + 60_000;
  while (Date.now() < deadline) {
    if (child.spawnError) throw child.spawnError;
    if (child.exitCode !== null || child.signalCode !== null) throw new Error(`Process exited (${child.exitCode ?? child.signalCode}) before readiness: ${url}`);
    try { if ((await fetch(url)).ok) return; } catch {}
    await delay(200);
  }
  throw new Error(`Timed out waiting for ${url}`);
};
const start = (command, args, options) => {
  const child = spawn(command, args, { stdio: 'inherit', detached: process.platform !== 'win32', ...options });
  child.once('error', error => { child.spawnError = error; });
  processes.push(child);
  return child;
};
let proxy;
let stopping = false;
let signalExitCode;
let stopHarness;
const stopped = new Promise(resolve => { stopHarness = resolve; });
async function cleanup() {
  if (stopping) return;
  stopping = true;
  if (proxy?.listening) {
    proxy.close();
    proxy.closeAllConnections?.();
  }
  for (const child of [...processes].reverse()) {
    try { if (process.platform === 'win32') child.kill('SIGTERM'); else process.kill(-child.pid, 'SIGTERM'); } catch {}
  }
  await Promise.all(processes.map(child => new Promise(resolve => {
    if (child.exitCode !== null || child.signalCode !== null) return resolve();
    let settled = false;
    const finish = () => { if (settled) return; settled = true; clearTimeout(timer); resolve(); };
    const timer = setTimeout(() => {
      try { if (process.platform === 'win32') child.kill('SIGKILL'); else process.kill(-child.pid, 'SIGKILL'); } catch {}
      finish();
    }, 5000);
    child.once('exit', finish);
    if (child.exitCode !== null || child.signalCode !== null) finish();
  })));
  await rm(temp, { recursive: true, force: true });
}
for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => {
  signalExitCode = signal === 'SIGINT' ? 130 : 143;
  process.exitCode = signalExitCode;
  for (const child of processes) {
    try { if (process.platform === 'win32') child.kill('SIGTERM'); else process.kill(-child.pid, 'SIGTERM'); } catch {}
  }
  stopHarness();
});
const run = (command, args, options) => new Promise((resolve, reject) => {
  const child = start(command, args, options);
  child.once('error', reject);
  child.once('exit', (code, signal) => code === 0 ? resolve() : reject(new Error(`${command} exited (${code ?? signal})`)));
});
try {
  temp = await mkdtemp(path.join(tmpdir(), 'mediaz-playwright-'));
  await run('npm', ['run', 'build'], { cwd: frontend, stdio: 'inherit' });
  const fakePort = await reservePort();
  const apiPort = await reservePort();
  const fakeBinary = path.join(temp, 'fake-tmdb');
  const mediazBinary = path.join(temp, 'mediaz');
  await run('go', ['build', '-o', fakeBinary, './tools/fake-tmdb'], { cwd: root, stdio: 'inherit' });
  await run('go', ['build', '-o', mediazBinary, '.'], { cwd: root, stdio: 'inherit' });
  const fake = start(fakeBinary, [], { cwd: root, env: { ...process.env, FAKE_TMDB_ADDR: `127.0.0.1:${fakePort}` } });
  await waitFor(`http://127.0.0.1:${fakePort}/3/search/movie?query=glass`, fake);
  const mediaz = start(mediazBinary, ['serve'], { cwd: root, env: {
    ...process.env,
    MEDIAZ_TMDB_URI: `http://127.0.0.1:${fakePort}`,
    MEDIAZ_TMDB_APIKEY: 'playwright-dummy-key',
    MEDIAZ_SERVER_PORT: String(apiPort),
    MEDIAZ_SERVER_DISTDIR: path.join(frontend, 'dist'),
    MEDIAZ_STORAGE_FILEPATH: path.join(temp, 'mediaz.sqlite'),
    MEDIAZ_LIBRARY_MOVIE: path.join(temp, 'movies'),
    MEDIAZ_LIBRARY_TV: path.join(temp, 'tv'),
    MEDIAZ_MANAGER_JOBS_MOVIEINDEX: '24h',
    MEDIAZ_MANAGER_JOBS_MOVIERECONCILE: '24h',
    MEDIAZ_MANAGER_JOBS_SERIESINDEX: '24h',
    MEDIAZ_MANAGER_JOBS_SERIESRECONCILE: '24h',
    MEDIAZ_MANAGER_JOBS_INDEXERSYNC: '24h',
    MEDIAZ_MANAGER_JOBS_JOBSCHEDULEINTERVAL: '24h',
  } });
  await waitFor(`http://127.0.0.1:${apiPort}/api/v1/healthz`, mediaz);
  // Playwright and the harness share this per-run origin; never attach to a user's service.
  proxy = http.createServer((req, res) => {
    const upstream = http.request({ host: '127.0.0.1', port: apiPort, path: req.url, method: req.method, headers: req.headers }, response => {
      res.writeHead(response.statusCode, response.headers);
      response.pipe(res);
    });
    upstream.on('error', () => { if (!res.headersSent) res.writeHead(502); res.end(); });
    req.pipe(upstream);
  });
  await new Promise((resolve, reject) => {
    proxy.once('error', reject);
    proxy.listen(Number(process.env.MEDIAZ_E2E_PROXY_PORT || 0), '127.0.0.1', resolve);
  });
  process.env.MEDIAZ_E2E_BASE_URL = `http://127.0.0.1:${proxy.address().port}`;
  await waitFor(`${process.env.MEDIAZ_E2E_BASE_URL}/api/v1/healthz`, { exitCode: null, signalCode: null });
  await stopped;
} catch (error) {
  if (signalExitCode === undefined) throw error;
} finally { await cleanup(); }
