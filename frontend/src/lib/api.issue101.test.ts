import { afterEach, describe, expect, it, vi } from 'vitest';
import { tvApi } from './api';

afterEach(() => vi.unstubAllGlobals());

describe('issue #101 API transport', () => {
  it('tolerates an empty 202 search response without parsing JSON', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response('', { status: 202 }));
    vi.stubGlobal('fetch', fetch);
    await expect(tvApi.requestEpisode(33)).resolves.toBeUndefined();
    await expect(tvApi.requestSeason(11)).resolves.toBeUndefined();
    await expect(tvApi.requestSeries(7)).resolves.toBeUndefined();
    const urls = fetch.mock.calls.map(([input]) => String(input));
    expect(urls.some(u => u.endsWith('/episode/33/search'))).toBe(true);
    expect(urls.some(u => u.endsWith('/season/11/search'))).toBe(true);
    expect(urls.some(u => u.endsWith('/library/tv/7/search'))).toBe(true);
  });

  it('tolerates an empty 204 response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })));
    await expect(tvApi.requestSeries(7)).resolves.toBeUndefined();
  });

  it('posts the atomic selection PATCH to the local series route and parses the updated detail', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ response: {
      tmdbID: 22001, title: 'Golden Series', posterPath: '/p.jpg', productionCountries: [], networks: [],
      seasonCount: 1, episodeCount: 1, genres: [], libraryStatus: 'missing', monitored: true, monitorNewSeasons: true, seasons: [],
    } }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetch);
    const result = await tvApi.updateSelection(7, {
      monitorNewSeasons: false,
      seasons: [{ id: 11, monitored: true, qualityProfileID: null }],
      episodes: [{ id: 22, monitored: false, qualityProfileID: 7 }],
    });
    const [url, init] = fetch.mock.calls[0];
    expect(String(url)).toContain('/library/tv/7/selection');
    expect(init.method).toBe('PATCH');
    const body = JSON.parse(init.body);
    expect(body).toEqual({ monitorNewSeasons: false, seasons: [{ id: 11, monitored: true, qualityProfileID: null }], episodes: [{ id: 22, monitored: false, qualityProfileID: 7 }] });
    expect(result).toMatchObject({ tmdbID: 22001, monitorNewSeasons: true });
  });
});
