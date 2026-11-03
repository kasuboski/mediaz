import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { TVSelectionEditor, makeTVSelection, setBulkMonitoring, buildSelectionPatch, episodeRequestBlockedReason, seasonRequestBlockedReason, seriesRequestBlockedReason, selectionSubmissionProblem, type TVSelection } from './TVSelectionEditor';
import type { EpisodeResult, SeasonResult } from '@/lib/api';

function seasonFixture(overrides: Partial<SeasonResult> = {}, episodes: EpisodeResult[] = []): SeasonResult {
  return {
    tmdbID: 2200101, seriesID: 1, seasonNumber: 1, title: 'Season 1', episodeCount: episodes.length,
    monitored: true, id: 11, ...overrides, episodes,
  };
}

function episodeFixture(overrides: Partial<EpisodeResult> = {}): EpisodeResult {
  return {
    tmdbID: 2200111, seriesID: 1, seasonNumber: 1, episodeNumber: 1, title: 'Episode 1',
    monitored: true, downloaded: false, id: 22, airDate: '2020-01-01', ...overrides,
  };
}

const profiles = [
  { id: 5, name: 'HD Series', cutoff_quality_id: null, upgradeAllowed: true, qualities: [] },
  { id: 7, name: 'Ultra Series', cutoff_quality_id: null, upgradeAllowed: true, qualities: [] },
];

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

describe('bulk All/None preserves quality overrides (issue #101)', () => {
  it('keeps season and episode overrides while flipping monitoring only', () => {
    const seasons = [seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: 23 })])];
    const draft: TVSelection = {
      1: { monitored: true, qualityProfileID: 5, episodes: { 1: { monitored: true, qualityProfileID: 7 }, 2: { monitored: false, qualityProfileID: null } } },
    };
    const next = setBulkMonitoring(seasons, draft, false);
    expect(next[1]).toEqual({ monitored: false, qualityProfileID: 5, episodes: { 1: { monitored: false, qualityProfileID: 7 }, 2: { monitored: false, qualityProfileID: null } } });
    const all = setBulkMonitoring(seasons, next, true);
    expect(all[1].monitored).toBe(true);
    expect(all[1].qualityProfileID).toBe(5);
    expect(all[1].episodes[1]).toEqual({ monitored: true, qualityProfileID: 7 });
  });

  it('clicking None in the editor keeps an explicit override', () => {
    const seasons = [seasonFixture({}, [episodeFixture()])];
    let latest: TVSelection | undefined;
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={s => { latest = s; }} previewProfileID={5} />);
    fireEvent.click(screen.getByRole('button', { name: 'Unmonitor all seasons and episodes' }));
    expect(latest?.[1].monitored).toBe(false);
    expect(latest?.[1].qualityProfileID).toBeNull();
    // Override set BEFORE None is preserved:
    const withOverride: TVSelection = { 1: { monitored: true, qualityProfileID: 7, episodes: { 1: { monitored: true, qualityProfileID: null } } } };
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={withOverride} onChange={s => { latest = s; }} previewProfileID={5} />);
    fireEvent.click(screen.getAllByRole('button', { name: 'Unmonitor all seasons and episodes' })[1]);
    expect(latest?.[1].qualityProfileID).toBe(7);
    expect(latest?.[1].episodes[1].qualityProfileID).toBeNull();
  });
});

describe('selection patch payload', () => {
  it('uses local IDs only and omits children outside the library', () => {
    const seasons = [
      seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: null })]),
      seasonFixture({ seasonNumber: 2, title: 'Future', id: null }, []),
    ];
    const draft = makeTVSelection(seasons, 'none');
    draft[1].monitored = true;
    draft[1].qualityProfileID = 7;
    draft[1].episodes[1].qualityProfileID = null;
    const patch = buildSelectionPatch(seasons, draft, true);
    expect(patch.seasons).toEqual([{ id: 11, monitored: true, qualityProfileID: 7 }]);
    expect(patch.episodes).toEqual([{ id: 22, monitored: false, qualityProfileID: null }]);
    expect(patch.monitorNewSeasons).toBe(true);
  });

  it('omits monitorNewSeasons when undefined so PATCH leaves it unchanged', () => {
    const seasons = [seasonFixture({}, [episodeFixture()])];
    const patch = buildSelectionPatch(seasons, makeTVSelection(seasons, 'none'));
    expect(patch.monitorNewSeasons).toBeUndefined();
  });

  it('distinguishes inherited (null) from explicit overrides in the patch', () => {
    const seasons = [seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: 23 })])];
    const draft = makeTVSelection(seasons, 'none');
    draft[1].episodes[1].qualityProfileID = 7;
    draft[1].episodes[2].qualityProfileID = null;
    const patch = buildSelectionPatch(seasons, draft);
    expect(patch.episodes).toEqual([
      { id: 22, monitored: false, qualityProfileID: 7 },
      { id: 23, monitored: false, qualityProfileID: null },
    ]);
  });
});

describe('per-target Request once guards', () => {
  it('reports block reasons for non-library, downloaded, and unreleased episodes', () => {
    expect(episodeRequestBlockedReason(episodeFixture({ id: null }))).toMatch(/local library/i);
    expect(episodeRequestBlockedReason(episodeFixture({ downloaded: true }))).toMatch(/downloaded/i);
    expect(episodeRequestBlockedReason(episodeFixture({ state: 'downloading' }))).toMatch(/downloaded|actively/i);
    expect(episodeRequestBlockedReason(episodeFixture({ airDate: '2099-01-01' }))).toMatch(/released/i);
    expect(episodeRequestBlockedReason(episodeFixture({}))).toBeNull();
    expect(seasonRequestBlockedReason(seasonFixture({ id: null }, [episodeFixture()]))).toMatch(/local library/i);
    expect(seasonRequestBlockedReason(seasonFixture({}, [episodeFixture({ downloaded: true }), episodeFixture({ episodeNumber: 2, id: 23, airDate: '2099-01-01' })]))).toMatch(/downloaded, active, or unreleased/i);
    expect(seasonRequestBlockedReason(seasonFixture({}, [episodeFixture({ downloaded: true }), episodeFixture({ episodeNumber: 2, id: 23 })]))).toBeNull();
  });

  it('hides request buttons for children without local IDs', () => {
    const seasons = [seasonFixture({ id: null }, [episodeFixture({ id: null })])];
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}} requestOnce={{ season: async () => {}, episode: async () => {} }} />);
    expect(screen.queryByRole('button', { name: /Request .* once/i })).not.toBeInTheDocument();
  });

  it('shows processed-not-download wording on success and disables duplicate clicks while pending', async () => {
    const seasons = [seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: 23, title: 'Episode 2' })])];
    let resolveEpisode!: () => void;
    const episodeRequests = vi.fn((_id: number) => new Promise<void>(resolve => { resolveEpisode = resolve; }));
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}}
      requestOnce={{ season: async () => {}, episode: episodeRequests }} />);
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    const buttons = screen.getAllByRole('button', { name: /Request episode .* once/i });
    expect(buttons).toHaveLength(2);
    fireEvent.click(buttons[0]);
    expect(episodeRequests).toHaveBeenCalledWith(22);
    // While one request is pending, all request buttons are disabled (no duplicate clicks).
    expect(buttons[0]).toBeDisabled();
    expect(buttons[1]).toBeDisabled();
    fireEvent.click(buttons[0]); // ignored
    fireEvent.click(buttons[1]); // ignored
    resolveEpisode();
    expect(await screen.findByRole('status')).toHaveTextContent('Request processed; check activity');
    expect(episodeRequests).toHaveBeenCalledTimes(1);
  });

  it('scopes errors to the row and keeps the draft selection unchanged', async () => {
    const seasons = [seasonFixture({}, [episodeFixture()])];
    const selection = makeTVSelection(seasons, 'all');
    const episodeRequests = vi.fn(async () => { throw new Error('Admission rejected'); });
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={selection} onChange={() => {}} requestOnce={{ season: async () => {}, episode: episodeRequests }} />);
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    fireEvent.click(screen.getByRole('button', { name: /Request episode .* once/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Admission rejected');
    // Selection draft was never mutated by the failed request.
    expect(selection[1].monitored).toBe(true);
    expect(selection[1].episodes[1].monitored).toBe(true);
  });
});

describe('editor semantics', () => {
  it('shows an indeterminate season checkbox for a partially monitored season', () => {
    const seasons = [seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: 23 })])];
    const selection = makeTVSelection(seasons, 'none');
    selection[1].episodes[1].monitored = true;
    selection[1].monitored = true;
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={selection} onChange={() => {}} previewProfileID={5} />);
    const seasonCheckbox = screen.getByRole('checkbox', { name: 'Monitor Season 1' });
    expect(seasonCheckbox).toHaveAttribute('data-state', 'indeterminate');
  });

  it('labels effective profiles with episode > season > series source resolution', () => {
    const seasons = [seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: 23 })])];
    const selection = makeTVSelection(seasons, 'all');
    selection[1].qualityProfileID = 5;
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={selection} onChange={() => {}} previewProfileID={7} />);
    // Season level: explicit override names the season source, even with a series profile present.
    expect(screen.getAllByText('Effective: HD Series · season override').length).toBeGreaterThan(0);
    // Episode without override inherits the season override, not the series profile (no ·series· label).
    expect(screen.queryByText('Effective: Ultra Series · series (inherited)')).not.toBeInTheDocument();
  });

  it('resolves episode override over season override for effective text', () => {
    const seasons = [seasonFixture({}, [episodeFixture()])];
    const selection = makeTVSelection(seasons, 'all');
    selection[1].qualityProfileID = 5;
    selection[1].episodes[1].qualityProfileID = 7;
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={selection} onChange={() => {}} previewProfileID={5} />);
    expect(screen.getByText('Effective: Ultra Series · episode override')).toBeInTheDocument();
  });

  it('labels an unresolved (null) effective profile as unavailable instead of implying the series default', () => {
    const seasons = [seasonFixture({}, [episodeFixture()])];
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}} />);
    expect(screen.getAllByText(/Effective profile unavailable \(unresolved\)/).length).toBeGreaterThan(0);
  });

  it('season disclosure contains and controls the episode rows via an accessible toggle outside a summary', () => {
    const seasons = [seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: 23 })])];
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}} />);
    const toggle = screen.getByRole('button', { name: 'Show episodes for Season 1' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    // The toggle itself is accessible OUTSIDE any summary.
    expect(toggle.closest('summary')).toBeNull();
    // The season name stays visible even while episodes are collapsed.
    expect(screen.getByText('Season 1')).toBeInTheDocument();
    fireEvent.click(toggle);
    const shown = screen.getByRole('button', { name: 'Hide episodes for Season 1' });
    expect(shown).toHaveAttribute('aria-expanded', 'true');
    // The episode rows live INSIDE the region the toggle controls.
    const episodeControl = screen.getByRole('checkbox', { name: /Monitor S01E01/ });
    const regionID = shown.getAttribute('aria-controls');
    expect(episodeControl.closest(`#${regionID}`)).not.toBeNull();
    fireEvent.click(shown);
    expect(screen.getByRole('button', { name: 'Show episodes for Season 1' })).toHaveAttribute('aria-expanded', 'false');
  });

  it('keeps the season name primary with actions in a secondary row, and shortened request blockers with tooltip reasons', () => {
    const seasons = [seasonFixture({}, [episodeFixture({ downloaded: true })])];
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}} requestOnce={{ season: async () => {}, episode: async () => {} }} />);
    // Season title appears BEFORE the disclosure/request actions in DOM order.
    const card = screen.getByText('Season 1').closest('div.border')!;
    const nameIndex = Array.prototype.indexOf.call(card.querySelectorAll('*'), screen.getByText('Season 1'));
    const actionIndex = Array.prototype.indexOf.call(card.querySelectorAll('*'), screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    expect(nameIndex).toBeLessThan(actionIndex);
    // Blocked reasons are short labels with the full reason in the tooltip (no repeated prose).
    const blocked = screen.getAllByText('Request once unavailable');
    expect(blocked.length).toBeGreaterThan(0);
    for (const node of blocked) {
      const reason = node.getAttribute('title');
      expect(reason).toMatch(/downloaded|unreleased/);
    }
    expect(screen.queryByText(/Request once unavailable \(/)).not.toBeInTheDocument();
  });

  it('stacks profile inheritance controls full-width on small screens instead of squeezing them', () => {
    const seasons = [seasonFixture({}, [episodeFixture()])];
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}} previewProfileID={5} />);
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    const trigger = screen.getByRole('combobox', { name: 'Quality profile override for Season 1' });
    expect(trigger.className).toMatch(/w-full/);
    expect(trigger.className).toMatch(/sm:w-\[190px\]/);
    const row = trigger.closest('div')!;
    expect(row.className).toMatch(/flex-col/);
    expect(row.className).toMatch(/sm:flex-row/);
  });

  it('compact variant stacks inheritance controls at EVERY width for narrow containers (request modal)', () => {
    const seasons = [seasonFixture({}, [episodeFixture()])];
    render(<TVSelectionEditor compact seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}} previewProfileID={5} />);
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    // On a wide viewport a 448px modal still matches sm: media queries, so the
    // compact variant must not rely on them at all.
    const trigger = screen.getByRole('combobox', { name: 'Quality profile override for Season 1' });
    expect(trigger.className).toMatch(/w-full/);
    expect(trigger.className).not.toMatch(/sm:w-\[190px\]/);
    const row = trigger.closest('div')!;
    expect(row.className).toMatch(/flex-col/);
    expect(row.className).not.toMatch(/sm:flex-row/);
  });

  it('series request shares the one-in-flight guard and shows inline processed status', async () => {
    const seasons = [seasonFixture({}, [episodeFixture()])];
    let resolveSeries!: () => void;
    const seriesRequests = vi.fn(() => new Promise<void>(resolve => { resolveSeries = resolve; }));
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}}
      requestOnce={{ series: seriesRequests, season: async () => {}, episode: async () => {} }} />);
    const seriesButton = screen.getByRole('button', { name: 'Request series once' });
    fireEvent.click(screen.getByRole('button', { name: 'Show episodes for Season 1' }));
    fireEvent.click(seriesButton);
    expect(seriesRequests).toHaveBeenCalledTimes(1);
    // Same in-flight state disables every other request button.
    expect(screen.getByRole('button', { name: /Request episode .* once/i })).toBeDisabled();
    fireEvent.click(seriesButton); // ignored
    resolveSeries();
    expect(await screen.findByRole('status')).toHaveTextContent('Request processed; check activity');
    expect(seriesRequests).toHaveBeenCalledTimes(1);
  });

  it('hides the series request when no monitored requestable child exists', () => {
    const seasons = [seasonFixture({}, [episodeFixture({ monitored: false })])];
    render(<TVSelectionEditor seasons={seasons} profiles={profiles} selection={makeTVSelection(seasons, 'all')} onChange={() => {}}
      requestOnce={{ series: async () => {}, season: async () => {}, episode: async () => {} }} />);
    expect(screen.queryByRole('button', { name: 'Request series once' })).not.toBeInTheDocument();
    const blocked = screen.getAllByText('Request once unavailable');
    expect(blocked.length).toBeGreaterThan(0);
    for (const node of blocked) expect(node).toHaveAttribute('title', 'No monitored aired or missing episodes available to request');
  });
});

describe('bounded fix pass: submission and request-availability guards', () => {
  it('season request availability requires a MONITORED aired/missing child; explicit unmonitored episodes stay requestable', () => {
    expect(seasonRequestBlockedReason(seasonFixture({}, [episodeFixture({ monitored: false })]))).toMatch(/No monitored aired or missing/);
    expect(seasonRequestBlockedReason(seasonFixture({}, [episodeFixture({ monitored: false }), episodeFixture({ episodeNumber: 2, id: 23 })]))).toBeNull();
    // An explicit episode can still be requested unmonitored.
    expect(episodeRequestBlockedReason(episodeFixture({ monitored: false }))).toBeNull();
    expect(seriesRequestBlockedReason([seasonFixture({}, [episodeFixture({ monitored: false })])])).toMatch(/No monitored aired or missing/);
    // A valid known zero-episode season stays requestable when aired.
    expect(seasonRequestBlockedReason(seasonFixture({ airDate: '2020-01-01' }, []))).toBeNull();
    expect(seasonRequestBlockedReason(seasonFixture({ airDate: '2099-01-01' }, []))).toMatch(/released/i);
  });

  it('selectionSubmissionProblem distinguishes incomplete metadata, metadata-only rows, and null-ID selected episodes', () => {
    // Incomplete metadata blocks regardless of the draft.
    const incomplete = [seasonFixture({ episodes: undefined as unknown as EpisodeResult[], episodeCount: 2 })];
    expect(selectionSubmissionProblem(incomplete, makeTVSelection(incomplete, 'all'))).toMatch(/incomplete/i);
    // Complete tree with local IDs never blocks, including a valid zero-episode season.
    const complete = [
      seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: 23 })]),
      seasonFixture({ seasonNumber: 2, title: 'Specials', id: 12 }, []),
    ];
    expect(selectionSubmissionProblem(complete, makeTVSelection(complete, 'all'))).toBeNull();
    // A selected metadata-only season cannot be submitted.
    const future = [seasonFixture({ seasonNumber: 2, title: 'Future', id: null }, [])];
    const futureDraft = makeTVSelection(future, 'none');
    expect(selectionSubmissionProblem(future, futureDraft)).toBeNull();
    futureDraft[2].monitored = true;
    expect(selectionSubmissionProblem(future, futureDraft)).toMatch(/Future is not in the local library/);
    // A selected episode without a local ID cannot be submitted; unmonitored ones are safely omitted.
    const mixed = [seasonFixture({}, [episodeFixture(), episodeFixture({ episodeNumber: 2, id: null, monitored: false })])];
    const mixedDraft = makeTVSelection(mixed, 'none');
    expect(selectionSubmissionProblem(mixed, mixedDraft)).toBeNull();
    mixedDraft[1].episodes[2].monitored = true;
    expect(selectionSubmissionProblem(mixed, mixedDraft)).toMatch(/S01E02 is not in the local library/);
  });
});
