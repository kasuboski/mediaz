import { afterEach, describe, expect, it, vi } from 'vitest';
import { activityApi } from './api';

afterEach(() => vi.unstubAllGlobals());
const meta = { page: 2, pageSize: 50, totalItems: 60, totalPages: 2 };
function response(value: unknown) { return { ok: true, json: async () => ({ response: value }) }; }

describe('activity API pagination', () => {
  it('serializes active pages and preserves per-collection envelope', async () => {
    const payload = { movies: [], series: [], jobs: [], counts: { movies: 0, series: 0, jobs: 1, total: 1 }, pagination: { movies: meta, series: meta, jobs: meta } };
    const fetch = vi.fn(async (_input: RequestInfo | URL) => response(payload)); vi.stubGlobal('fetch', fetch);
    expect(await activityApi.getActiveActivity({ moviesPage: 2, seriesPage: 3, jobsPage: 4, pageSize: 20 })).toEqual(payload);
    expect(fetch.mock.calls[0][0].toString()).toContain('moviesPage=2&seriesPage=3&jobsPage=4&pageSize=20');
  });
  it('preserves failure envelope and serializes bounded pagination', async () => {
    const payload = { failures: [], pagination: meta }; const fetch = vi.fn(async (_input: RequestInfo | URL) => response(payload)); vi.stubGlobal('fetch', fetch);
    expect(await activityApi.getRecentFailures(12, { page: 2, pageSize: 50 })).toEqual(payload);
    expect(fetch.mock.calls[0][0].toString()).toContain('hours=12&page=2&pageSize=50');
  });
  it('returns history metadata for requested page', async () => {
    const payload = { entity: { type: 'movie', id: 4, title: 'x', poster_path: '' }, history: [], pagination: meta };
    const fetch = vi.fn(async (_input: RequestInfo | URL) => response(payload)); vi.stubGlobal('fetch', fetch);
    expect(await activityApi.getEntityHistory('movie', 4, { page: 2 })).toEqual(payload);
    expect(fetch.mock.calls[0][0].toString()).toContain('/activity/history/movie/4?page=2&pageSize=50');
  });
});
