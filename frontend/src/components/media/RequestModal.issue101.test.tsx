import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RequestModal } from './RequestModal';
import { hasIncompleteSeasonMetadata } from './TVSelectionEditor';
import type { SeasonResult } from '@/lib/api';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

function seasonFixture(episodes?: SeasonResult['episodes'], episodeCount = 2): SeasonResult {
  return { tmdbID: 2200101, seriesID: 0, seasonNumber: 1, title: 'Season 1', episodeCount, monitored: false, id: null, episodes };
}

describe('season metadata completeness guard (issue #101)', () => {
  it('flags missing episode arrays and falsely-complete empty seasons', () => {
    expect(hasIncompleteSeasonMetadata([{ ...seasonFixture(undefined) }])).toBe(true);
    expect(hasIncompleteSeasonMetadata([{ ...seasonFixture([]) }])).toBe(true);
    expect(hasIncompleteSeasonMetadata([{ ...seasonFixture([], 0) }])).toBe(false);
    expect(hasIncompleteSeasonMetadata([{ ...seasonFixture([
      { tmdbID: 2200111, seriesID: 0, seasonNumber: 1, episodeNumber: 1, title: 'E1', monitored: false, downloaded: false, id: null },
    ]) }])).toBe(false);
  });

  it('disables submission while TV episode metadata is incomplete instead of sending a falsely-complete selection', async () => {
    const fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      if (url.pathname.startsWith('/api/v1/quality/profiles')) {
        return new Response(JSON.stringify({ response: { items: [
          { id: 5, name: 'HD Series', cutoff_quality_id: null, upgradeAllowed: true, qualities: [] },
        ], pagination: { page: 1, pageSize: 100, totalItems: 1, totalPages: 1 } } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      if (url.pathname === '/api/v1/tv/22001') {
        return new Response(JSON.stringify({ response: {
          tmdbID: 22001, title: 'Golden Series', posterPath: '/p.jpg', productionCountries: [], networks: [],
          seasonCount: 1, episodeCount: 2, genres: [], libraryStatus: 'Not In Library', monitored: true, monitorNewSeasons: false,
          seasons: [seasonFixture([])],
        } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      return new Response('{}', { status: 404 });
    });
    vi.stubGlobal('fetch', fetch);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <RequestModal isOpen onClose={() => {}} mediaType="tv" mediaTitle="Golden Series" tmdbID={22001} />
      </QueryClientProvider>
    );
    // Even once metadata loads (editor visible), the incomplete tree blocks submission.
    expect(await screen.findByText('Season 1')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Request Series' })).toBeDisabled();
    // No add-series request is ever sent with a falsely-complete selection.
    expect((fetch.mock.calls as [RequestInfo | URL, RequestInit?][]).some(([input, init]) => String(input).endsWith('/library/tv') && init?.method === 'POST')).toBe(false);
  });

  it('keeps the expanded season tree inside the viewport with a scrollable region between fixed header and actions', async () => {
    const fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      if (url.pathname.startsWith('/api/v1/quality/profiles')) {
        return new Response(JSON.stringify({ response: { items: [
          { id: 5, name: 'HD Series', cutoff_quality_id: null, upgradeAllowed: true, qualities: [] },
        ], pagination: { page: 1, pageSize: 100, totalItems: 1, totalPages: 1 } } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      if (url.pathname === '/api/v1/tv/22001') {
        const episode = (n: number) => ({ tmdbID: 2200100 + n, seriesID: 0, seasonNumber: 1, episodeNumber: n, title: `E${n}`, monitored: false, downloaded: false, id: null });
        return new Response(JSON.stringify({ response: {
          tmdbID: 22001, title: 'Golden Series', posterPath: '/p.jpg', productionCountries: [], networks: [],
          seasonCount: 1, episodeCount: 2, genres: [], libraryStatus: 'Not In Library', monitored: true, monitorNewSeasons: false,
          seasons: [{ tmdbID: 2200101, seriesID: 0, seasonNumber: 1, title: 'Season 1', episodeCount: 2, monitored: false, id: null, episodes: [episode(1), episode(2)] }],
        } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      return new Response('{}', { status: 404 });
    });
    vi.stubGlobal('fetch', fetch);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <RequestModal isOpen onClose={() => {}} mediaType="tv" mediaTitle="Golden Series" tmdbID={22001} />
      </QueryClientProvider>
    );
    await screen.findByText('Season 1');
    // The dialog itself is viewport-constrained (no fixed pixel height).
    const dialog = screen.getByRole('dialog');
    expect(dialog.className).toMatch(/max-h-\[100dvh\]/);
    expect(dialog.className).toMatch(/flex-col/);
    // The season tree and profile select live in one scrollable middle region...
    const scrollable = screen.getByText('Quality Profile').closest('div.overflow-y-auto')!;
    expect(scrollable).not.toBeNull();
    // ...while the header and the action footer sit OUTSIDE it, always reachable.
    expect(scrollable).not.toContainElement(screen.getByRole('button', { name: 'Close' }));
    expect(scrollable).not.toContainElement(screen.getByRole('button', { name: 'Request Series' }));
    expect(scrollable).not.toContainElement(screen.getByRole('button', { name: 'Cancel' }));
  });

  it('renders the season tree in the COMPACT editor variant so inheritance controls never ellipsize inside the 448px modal', async () => {
    const fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      if (url.pathname.startsWith('/api/v1/quality/profiles')) {
        return new Response(JSON.stringify({ response: { items: [
          { id: 5, name: 'HD Series', cutoff_quality_id: null, upgradeAllowed: true, qualities: [] },
        ], pagination: { page: 1, pageSize: 100, totalItems: 1, totalPages: 1 } } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      if (url.pathname === '/api/v1/tv/22001') {
        const episode = (n: number) => ({ tmdbID: 2200100 + n, seriesID: 0, seasonNumber: 1, episodeNumber: n, title: `E${n}`, monitored: false, downloaded: false, id: null });
        return new Response(JSON.stringify({ response: {
          tmdbID: 22001, title: 'Golden Series', posterPath: '/p.jpg', productionCountries: [], networks: [],
          seasonCount: 1, episodeCount: 2, genres: [], libraryStatus: 'Not In Library', monitored: true, monitorNewSeasons: false,
          seasons: [{ tmdbID: 2200101, seriesID: 0, seasonNumber: 1, title: 'Season 1', episodeCount: 2, monitored: false, id: null, episodes: [episode(1)] }],
        } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      return new Response('{}', { status: 404 });
    });
    vi.stubGlobal('fetch', fetch);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <RequestModal isOpen onClose={() => {}} mediaType="tv" mediaTitle="Golden Series" tmdbID={22001} />
      </QueryClientProvider>
    );
    await screen.findByText('Season 1');
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    // The in-modal trigger is always full-width and its row never uses the
    // viewport-based sm:flex-row layout (the modal is narrow even at 1440px).
    const trigger = screen.getByRole('combobox', { name: 'Quality profile override for Season 1' });
    expect(trigger.className).toMatch(/w-full/);
    expect(trigger.className).not.toMatch(/sm:w-\[190px\]/);
    const row = trigger.closest('div')!;
    expect(row.className).toMatch(/flex-col/);
    expect(row.className).not.toMatch(/sm:flex-row/);
  });

  it('preserves user monitoring/none drafts across an equivalent preview refetch, and reinitializes only on a genuinely changed scope', async () => {
    const episode = (episodeNumber: number) => ({ tmdbID: 2200100 + episodeNumber, seriesID: 0, seasonNumber: 1, episodeNumber, title: `E${episodeNumber}`, monitored: false, downloaded: false, id: null });
    const detailBody = () => ({
      tmdbID: 22001, title: 'Golden Series', posterPath: '/p.jpg', productionCountries: [], networks: [],
      seasonCount: 1, episodeCount: 2, genres: [], libraryStatus: 'Not In Library', monitored: true, monitorNewSeasons: false,
      seasons: [{ tmdbID: 2200101, seriesID: 0, seasonNumber: 1, title: 'Season 1', episodeCount: 2, monitored: false, id: null, episodes: [episode(1), episode(2)] }],
    });
    const fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      if (url.pathname.startsWith('/api/v1/quality/profiles')) {
        return new Response(JSON.stringify({ response: { items: [
          { id: 5, name: 'HD Series', cutoff_quality_id: null, upgradeAllowed: true, qualities: [] },
        ], pagination: { page: 1, pageSize: 100, totalItems: 1, totalPages: 1 } } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      if (url.pathname === '/api/v1/tv/22001' || url.pathname === '/api/v1/tv/22002') {
        // A FRESH object every call: refetch must not reset the draft via object identity.
        const body = url.pathname === '/api/v1/tv/22001' ? detailBody() : {
          ...detailBody(), tmdbID: 22002,
          seasons: [{ tmdbID: 2200101, seriesID: 0, seasonNumber: 1, title: 'Season 1', episodeCount: 1, monitored: false, id: null, episodes: [episode(1)] }],
        };
        return new Response(JSON.stringify({ response: body }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      return new Response('{}', { status: 404 });
    });
    vi.stubGlobal('fetch', fetch);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const view = render(
      <QueryClientProvider client={client}>
        <RequestModal isOpen onClose={() => {}} mediaType="tv" mediaTitle="Golden Series" tmdbID={22001} />
      </QueryClientProvider>
    );
    await screen.findByText('Season 1');
    // Build a custom draft: None, then re-monitor E01 only.
    fireEvent.click(screen.getByRole('button', { name: 'Unmonitor all seasons and episodes' }));
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Monitor S01E01/ }));
    expect(screen.getByRole('checkbox', { name: /Monitor S01E01/ })).toHaveAttribute('data-state', 'checked');
    expect(screen.getByRole('checkbox', { name: /Monitor S01E02/ })).toHaveAttribute('data-state', 'unchecked');
    // Equivalent refetch returns a NEW preview object with the same structure.
    await act(async () => { await client.refetchQueries({ queryKey: ['tv', 'detail', 22001] }); });
    // The draft survived: none/custom monitoring choices were not reset.
    expect(screen.getByRole('checkbox', { name: /Monitor S01E01/ })).toHaveAttribute('data-state', 'checked');
    expect(screen.getByRole('checkbox', { name: /Monitor S01E02/ })).toHaveAttribute('data-state', 'unchecked');
    // A genuinely changed scope (different show) reinitializes deliberately.
    view.rerender(
      <QueryClientProvider client={client}>
        <RequestModal isOpen onClose={() => {}} mediaType="tv" mediaTitle="Other Series" tmdbID={22002} />
      </QueryClientProvider>
    );
    await screen.findByText('Season 1');
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    expect(screen.queryByRole('checkbox', { name: /Monitor S01E02/ })).not.toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: /Monitor S01E01/ })).toHaveAttribute('data-state', 'checked');
  });
});
