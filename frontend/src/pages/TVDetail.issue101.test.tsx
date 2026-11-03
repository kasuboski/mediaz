import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import TVDetail from './TVDetail';
import type { TVDetailResult } from '@/lib/api';

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
import { toast } from 'sonner';

function detailFixture(overrides: Partial<TVDetailResult> = {}): TVDetailResult {
  return {
    id: 7,
    tmdbID: 22001,
    title: 'Golden Series',
    posterPath: '/p.jpg',
    productionCountries: [],
    networks: [],
    seasonCount: 1,
    episodeCount: 2,
    genres: [],
    libraryStatus: 'missing',
    qualityProfileID: 5,
    monitored: true,
    monitorNewSeasons: false,
    seasons: [{
      tmdbID: 2200101, seriesID: 1, seasonNumber: 1, title: 'Season 1', episodeCount: 2,
      monitored: true, id: 11, qualityProfileID: null, episodes: [
        { tmdbID: 2200111, seriesID: 1, seasonNumber: 1, episodeNumber: 1, title: 'Episode 1', monitored: true, downloaded: false, id: 22, airDate: '2020-01-01', qualityProfileID: null },
        { tmdbID: 2200112, seriesID: 1, seasonNumber: 1, episodeNumber: 2, title: 'Episode 2', monitored: false, downloaded: false, id: 23, airDate: '2099-06-01', qualityProfileID: 7 },
      ],
    }],
    ...overrides,
  };
}

function detailResponse(detail: TVDetailResult) {
  return new Response(JSON.stringify({ response: detail }), { status: 200, headers: { 'Content-Type': 'application/json' } });
}

type ExtraHandler = (url: URL, method: string, init?: RequestInit) => Response | undefined;

function setup(initialDetail: TVDetailResult, extra?: ExtraHandler): Mock {
  const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), 'http://localhost');
    const method = (init?.method ?? 'GET').toUpperCase();
    const handled = extra?.(url, method, init);
    if (handled) return handled;
    if (url.pathname === '/api/v1/tv/22001') return detailResponse(initialDetail);
    if (url.pathname.startsWith('/api/v1/quality/profiles')) {
      return new Response(JSON.stringify({ response: { items: [
        { id: 5, name: 'HD Series', cutoff_quality_id: null, upgradeAllowed: true, qualities: [] },
        { id: 7, name: 'Ultra Series', cutoff_quality_id: null, upgradeAllowed: true, qualities: [] },
      ], pagination: { page: 1, pageSize: 100, totalItems: 2, totalPages: 1 } } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }
    throw new Error(`Unexpected request: ${method} ${url.pathname}`);
  });
  vi.stubGlobal('fetch', fetch);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/tv/22001']}>
        <Routes><Route path="/tv/:id" element={<TVDetail />} /></Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
  return fetch;
}

beforeEach(() => { vi.mocked(toast.success).mockClear(); vi.mocked(toast.error).mockClear(); });
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

/** The selection editor lives behind the explicit "Edit monitoring & quality" view toggle. */
async function openEditor() {
  await screen.findByText('Seasons & acquisition');
  fireEvent.click(screen.getByRole('button', { name: 'Edit monitoring & quality' }));
}

describe('TVDetail selection editing (issue #101)', () => {
  it('saves an atomic PATCH with local IDs, explicit booleans, overrides, and the future-seasons setting', async () => {
    const fetch = setup(detailFixture(), (url, method) => {
      if (method === 'PATCH' && url.pathname === '/api/v1/library/tv/7/selection') {
        return detailResponse(detailFixture({ monitorNewSeasons: true }));
      }
      return undefined;
    });
    await openEditor();
    // Expand and enable Save: unmonitor one aired episode creates the draft.
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Monitor S01E01 Episode 1/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'Monitor future seasons' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save selection' }));

    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Selection saved'));
    const patchCall = fetch.mock.calls.find(([input, init]) => String(input).includes('/library/tv/7/selection') && init?.method === 'PATCH');
    expect(patchCall).toBeTruthy();
    const body = JSON.parse(patchCall![1].body);
    // Local IDs only (7/11/22/23), never TMDB IDs; explicit booleans for every loaded child.
    // Draft is based on the loaded 'none' baseline, so unmonitoring S01E01 leaves everything false.
    expect(body).toEqual({
      seasons: [{ id: 11, monitored: false, qualityProfileID: null }],
      episodes: [
        { id: 22, monitored: false, qualityProfileID: null },
        { id: 23, monitored: false, qualityProfileID: 7 },
      ],
      monitorNewSeasons: true,
    });
  });

  it('hides selection controls outside the library', async () => {
    setup(detailFixture({ libraryStatus: 'Not In Library' }));
    await screen.findByText('Seasons & acquisition');
    expect(screen.queryByRole('button', { name: 'Save selection' })).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: 'Monitor future seasons' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit monitoring & quality' })).not.toBeInTheDocument();
  });

  it('shows ONE season tree at a time: metadata accordion by default, selection editor only in edit mode', async () => {
    setup(detailFixture());
    await screen.findByText('Seasons & acquisition');
    // Default view: episode metadata accordion only; no editor, no automation controls.
    expect(screen.getByRole('button', { name: /^Season 1 2 episodes/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Show episodes for Season 1' })).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: 'Monitor future seasons' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Request series once' })).not.toBeInTheDocument();
    // Edit view: selection editor only; the metadata accordion is gone.
    fireEvent.click(screen.getByRole('button', { name: 'Edit monitoring & quality' }));
    expect(screen.getByRole('button', { name: 'Show episodes for Season 1' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Monitor future seasons' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Season 1 2 episodes/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Back to episode details' })).toBeInTheDocument();
  });

  it('preserves the draft, metadata access, and dirty state across view switches', async () => {
    setup(detailFixture());
    await openEditor();
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Monitor S01E01 Episode 1/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'Monitor future seasons' }));
    expect(screen.getByRole('button', { name: 'Save selection' })).toBeEnabled();
    // Switch back to details: metadata accordion is available again and the
    // unsaved draft is surfaced, not silently dropped.
    fireEvent.click(screen.getByRole('button', { name: 'Back to episode details' }));
    expect(screen.getByRole('button', { name: /^Season 1 2 episodes/ })).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('unsaved monitoring and quality changes');
    expect(screen.getByRole('button', { name: 'Save selection' })).toBeEnabled();
    // Return to the editor: the draft (unchecked S01E01, future seasons on) is intact.
    fireEvent.click(screen.getByRole('button', { name: 'Edit monitoring & quality' }));
    // Editor-local disclosure resets on remount (view-local state), so reopen it.
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    expect(screen.getByRole('checkbox', { name: /Monitor S01E01 Episode 1/i })).toHaveAttribute('data-state', 'unchecked');
    expect(screen.getByRole('checkbox', { name: 'Monitor future seasons' })).toHaveAttribute('data-state', 'checked');
    expect(screen.getByRole('button', { name: 'Save selection' })).toBeEnabled();
  });

  it('sends Request once for an episode to its LOCAL route and reports processed-not-download', async () => {
    const fetch = setup(detailFixture(), (url, method) => {
      if (method === 'POST' && url.pathname === '/api/v1/episode/22/search') {
        return new Response('', { status: 202 });
      }
      return undefined;
    });
    await openEditor();
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    fireEvent.click(screen.getByRole('button', { name: /Request episode S01E01 once/i }));
    expect(await screen.findByRole('status')).toHaveTextContent('Request processed; check activity');
    expect((fetch.mock.calls as [RequestInfo | URL][]).some(([input]) => String(input).endsWith('/api/v1/episode/22/search'))).toBe(true);
    // Unreleased episode S01E02 offers no request button.
    expect(screen.queryByRole('button', { name: /Request episode S01E02 once/i })).not.toBeInTheDocument();
  });

  it('blocks Save with a visible reason when the draft selects a child without a local ID, and sends no PATCH', async () => {
    const noID = detailFixture();
    noID.seasons![0].episodes![1].id = null;
    // The server reports this metadata-only child as monitored, so the draft inherits it.
    noID.seasons![0].episodes![1].monitored = true;
    const fetch = setup(noID);
    await openEditor();
    // Toggling any child creates a draft; the draft would select the metadata-only episode too.
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Monitor S01E01 Episode 1/i }));
    const save = screen.getByRole('button', { name: 'Save selection' });
    expect(save).toBeDisabled();
    expect(screen.getByRole('alert')).toHaveTextContent('S01E02 is not in the local library');
    expect((fetch.mock.calls as [RequestInfo | URL, RequestInit?][]).some(([input, init]) => String(input).includes('/library/tv/7/selection') && init?.method === 'PATCH')).toBe(false);
  });

  it('blocks Save while season metadata is incomplete instead of sending a falsely-complete hierarchy', async () => {
    const bad = detailFixture();
    bad.seasons![0].episodes = undefined;
    bad.seasons![0].episodeCount = 2;
    setup(bad);
    await openEditor();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Monitor Season 1' }));
    expect(screen.getByRole('button', { name: 'Save selection' })).toBeDisabled();
    expect(screen.getByRole('alert')).toHaveTextContent('metadata is incomplete');
  });

  it('sends the series-level Request once through the shared in-flight state with an inline processed status', async () => {
    const fetch = setup(detailFixture(), (url, method) => {
      if (method === 'POST' && url.pathname === '/api/v1/library/tv/7/search') {
        return new Response('', { status: 202 });
      }
      return undefined;
    });
    await openEditor();
    // Exactly one series Request once control exists, integrated in the editor.
    expect(screen.getAllByRole('button', { name: 'Request series once' })).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: 'Request series once' }));
    expect(await screen.findByRole('status')).toHaveTextContent('Request processed; check activity');
    expect((fetch.mock.calls as [RequestInfo | URL][]).some(([input]) => String(input).endsWith('/api/v1/library/tv/7/search'))).toBe(true);
  });

  it('associates the future-seasons checkbox with its accessible label', async () => {
    setup(detailFixture());
    await openEditor();
    const checkbox = screen.getByLabelText('Monitor future seasons');
    expect(checkbox).toHaveAttribute('id', 'monitor-new-seasons-22001');
  });

  it('preserves the draft and future-seasons toggle when saving fails', async () => {
    setup(detailFixture(), (url, method) => {
      if (method === 'PATCH' && url.pathname === '/api/v1/library/tv/7/selection') {
        return new Response(JSON.stringify({ error: 'boom' }), { status: 500 });
      }
      return undefined;
    });
    await openEditor();
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Monitor S01E01 Episode 1/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'Monitor future seasons' }));
    const save = screen.getByRole('button', { name: 'Save selection' });
    fireEvent.click(save);
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('HTTP 500')));
    // Draft preserved: Save remains enabled and the editor state is intact (S01E01 now unchecked in the draft).
    expect(save).toBeEnabled();
    expect(screen.getByRole('checkbox', { name: /Monitor S01E01 Episode 1/i })).toHaveAttribute('data-state', 'unchecked');
    expect(screen.getByRole('checkbox', { name: 'Monitor future seasons' })).toHaveAttribute('data-state', 'checked');
  });

  it('saves a sparse future-seasons-only PATCH without any tree edits, unrelated monitoring, or metadata validation', async () => {
    let patched = false;
    const fetch = setup(detailFixture(), (url, method) => {
      if (method === 'PATCH' && url.pathname === '/api/v1/library/tv/7/selection') {
        patched = true;
        return detailResponse(detailFixture({ monitorNewSeasons: true }));
      }
      if (patched && method === 'GET' && url.pathname === '/api/v1/tv/22001') {
        return detailResponse(detailFixture({ monitorNewSeasons: true }));
      }
      return undefined;
    });
    await openEditor();
    const save = screen.getByRole('button', { name: 'Save selection' });
    // Fresh route: Save is disabled until an edit exists.
    expect(save).toBeDisabled();
    // Toggle ONLY the future-seasons option; no tree edit, no expansion needed.
    fireEvent.click(screen.getByRole('checkbox', { name: 'Monitor future seasons' }));
    expect(save).toBeEnabled();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    fireEvent.click(save);

    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Selection saved'));
    const patchCall = fetch.mock.calls.find(([input, init]) => String(input).includes('/library/tv/7/selection') && init?.method === 'PATCH');
    expect(patchCall).toBeTruthy();
    // The PATCH changes ONLY the future-season option: no seasons, episodes, or other flags.
    expect(JSON.parse(patchCall![1].body)).toEqual({ monitorNewSeasons: true });
    // After success the drafts reset and Save is disabled again.
    expect(save).toBeDisabled();
    await waitFor(() => expect(screen.getByRole('checkbox', { name: 'Monitor future seasons' })).toHaveAttribute('data-state', 'checked'));
  });

  it('preserves the future-seasons-only draft when saving fails and allows retry', async () => {
    const fetch = setup(detailFixture(), (url, method) => {
      if (method === 'PATCH' && url.pathname === '/api/v1/library/tv/7/selection') {
        return new Response(JSON.stringify({ error: 'boom' }), { status: 500 });
      }
      return undefined;
    });
    await openEditor();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Monitor future seasons' }));
    const save = screen.getByRole('button', { name: 'Save selection' });
    fireEvent.click(save);
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('HTTP 500')));
    // Draft preserved and retryable; every PATCH carried ONLY the future-season option.
    expect(save).toBeEnabled();
    expect(screen.getByRole('checkbox', { name: 'Monitor future seasons' })).toHaveAttribute('data-state', 'checked');
    const patchBodies = fetch.mock.calls
      .filter(([input, init]) => String(input).includes('/library/tv/7/selection') && init?.method === 'PATCH')
      .map(([, init]) => JSON.parse(init!.body));
    expect(patchBodies).toEqual([{ monitorNewSeasons: true }]);
  });
});
