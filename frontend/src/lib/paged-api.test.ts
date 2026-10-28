import { afterEach, describe, expect, it, vi } from 'vitest';
import { downloadClientsApi, indexerSourcesApi, indexersApi, qualityDefinitionsApi, qualityProfilesApi, searchApi } from './api';

afterEach(() => vi.unstubAllGlobals());
const response = (body: unknown) => new Response(JSON.stringify({ response: body }), { status: 200, headers: { 'Content-Type': 'application/json' } });

describe('paged API plumbing', () => {
  it('maps config envelopes and preserves quality type filters', async () => {
    const fetch = vi.fn().mockResolvedValue(response({ items: [{ ID: 1 }], pagination: { page: 2, pageSize: 50, totalItems: 1, totalPages: 1 } }));
    vi.stubGlobal('fetch', fetch);
    const page = await qualityProfilesApi.getQualityProfilesPage({ page: 2 }, 'series');
    expect(new URL(fetch.mock.calls[0][0] as string, 'http://localhost').searchParams.toString()).toBe('page=2&pageSize=50&type=series');
    expect(page.items).toEqual([{ ID: 1 }]);
    expect(page.pagination.page).toBe(2);
  });

  it('traverses all pages for catalog helpers', async () => {
    const fetch = vi.fn().mockImplementation((url: string) => {
      const page = Number(new URL(url, 'http://localhost').searchParams.get('page'));
      return Promise.resolve(response({ items: page === 1 ? Array.from({ length: 100 }, (_, i) => ({ ID: i })) : [{ ID: 100 }], pagination: { page, pageSize: 100, totalItems: 101, totalPages: 2 } }));
    });
    vi.stubGlobal('fetch', fetch);
    const items = await downloadClientsApi.listClients();
    expect(items).toHaveLength(101);
    expect(items[100]).toEqual({ ID: 100 });
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it('supports extracted full-catalog methods across every config catalog', async () => {
    const fetch = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      const page = Number(url.searchParams.get('page'));
      const items = Array.from({ length: page === 1 ? 100 : 1 }, (_, i) => ({ id: (page - 1) * 100 + i + 1 }));
      return Promise.resolve(response({ items, pagination: { page, pageSize: 100, totalItems: 101, totalPages: 2 } }));
    });
    vi.stubGlobal('fetch', fetch);
    const catalogs = [
      [indexerSourcesApi.list, indexerSourcesApi],
      [indexersApi.list, indexersApi],
      [downloadClientsApi.listClients, downloadClientsApi],
      [qualityDefinitionsApi.listDefinitions, qualityDefinitionsApi],
      [qualityProfilesApi.listProfiles, qualityProfilesApi],
    ] as const;
    for (const [method] of catalogs) {
      fetch.mockClear();
      const items = await method();
      expect(items).toHaveLength(101);
      expect(items[100]).toEqual({ id: 101 });
      expect(fetch).toHaveBeenCalledTimes(2);
    }
  });

  it('rejects a failed catalog page instead of returning a partial catalog', async () => {
    const fetch = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const page = Number(new URL(String(input), 'http://localhost').searchParams.get('page'));
      return Promise.resolve(page === 1
        ? response({ items: [{ id: 1 }], pagination: { page: 1, pageSize: 100, totalItems: 101, totalPages: 2 } })
        : new Response('upstream failed', { status: 500 }));
    });
    vi.stubGlobal('fetch', fetch);
    const fetchSources = indexerSourcesApi.list;
    await expect(fetchSources()).rejects.toThrow();
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it('requests discovery page two and safely defaults missing TMDB metadata', async () => {
    const fetch = vi.fn().mockResolvedValue(response({ results: [{ id: 4, title: 'Page two' }] }));
    vi.stubGlobal('fetch', fetch);
    const result = await searchApi.searchMoviesPage('a & b', 2);
    const url = new URL(fetch.mock.calls[0][0] as string, 'http://localhost');
    expect(url.searchParams.get('query')).toBe('a & b');
    expect(url.searchParams.get('page')).toBe('2');
    expect(result).toEqual({ items: [{ id: 4, title: 'Page two', poster_path: '', release_date: undefined, first_air_date: undefined, media_type: 'movie' }], pagination: { page: 1, pageSize: 20, totalItems: 0, totalPages: 0 } });
  });
});
