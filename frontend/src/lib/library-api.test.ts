import { afterEach, describe, expect, it, vi } from 'vitest';
import { moviesApi } from './api';

afterEach(() => vi.unstubAllGlobals());

describe('library API', () => {
  it('serializes normalized repeated states and maps items while retaining server counts', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ response: {
      items: [{ path: '/x', tmdbID: 8, title: 'Film', poster_path: '/p', year: 2020, state: 'downloaded' }],
      pagination: { page: 2, pageSize: 25, totalItems: 26, totalPages: 2 },
      byState: { missing: 7, downloaded: 4 },
    } }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetch);
    const result = await moviesApi.getLibraryMovies({ page: 2, pageSize: 25, states: ['downloaded', 'discovered', 'downloaded'] });
    expect(fetch.mock.calls[0][0]).toContain('page=2&pageSize=25&state=discovered&state=downloaded');
    expect(result).toMatchObject({ pagination: { totalItems: 26 }, byState: { missing: 7, downloaded: 4 }, items: [{ id: 8, media_type: 'movie', state: 'downloaded', year: 2020 }] });
  });
});
