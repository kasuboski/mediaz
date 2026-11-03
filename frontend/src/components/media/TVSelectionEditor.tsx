import { useMemo, useState } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import type { EpisodeResult, QualityProfile, SeasonResult, TVSelectionPatch } from "@/lib/api";

export interface SelectionDraft {
  monitored: boolean;
  qualityProfileID: number | null;
  episodes: Record<number, { monitored: boolean; qualityProfileID: number | null }>;
}
export type TVSelection = Record<number, SelectionDraft>;

export function makeTVSelection(seasons: SeasonResult[], mode: "all" | "none" = "all"): TVSelection {
  return Object.fromEntries(seasons.map(season => [season.seasonNumber, {
    monitored: mode === "all", qualityProfileID: null,
    episodes: Object.fromEntries((season.episodes ?? []).map(episode => [episode.episodeNumber, { monitored: mode === "all", qualityProfileID: null }]))
  }]));
}

/**
 * Detail editing baseline: the draft starts from the SERVER's stored choices
 * (monitored flags and quality overrides) so the editor never shows a
 * contradictory state; omitted override fields mean inherit (null).
 */
export function tvSelectionFromSeasons(seasons: SeasonResult[]): TVSelection {
  return Object.fromEntries(seasons.map(season => [season.seasonNumber, {
    monitored: season.monitored, qualityProfileID: season.qualityProfileID ?? null,
    episodes: Object.fromEntries((season.episodes ?? []).map(episode => [episode.episodeNumber, { monitored: episode.monitored, qualityProfileID: episode.qualityProfileID ?? null }]))
  }]));
}

/**
 * Bulk All/None toggles change MONITORING ONLY. Existing quality overrides
 * (persistent choices) are preserved on every season and episode.
 */
export function setBulkMonitoring(seasons: SeasonResult[], selection: TVSelection, monitored: boolean): TVSelection {
  return Object.fromEntries(seasons.map(season => {
    const current = selection[season.seasonNumber] ?? { monitored: !monitored, qualityProfileID: null, episodes: {} };
    return [season.seasonNumber, {
      monitored,
      qualityProfileID: current.qualityProfileID,
      episodes: Object.fromEntries(Object.entries(current.episodes).map(([number, episode]) => [number, { monitored, qualityProfileID: episode.qualityProfileID }])),
    }];
  }));
}

/**
 * Build the atomic selection PATCH from the draft. Only LOCAL library IDs are
 * used; children outside the library (null id) are omitted so the server never
 * receives TMDB IDs. This is a sparse full snapshot of loaded children: no
 * implicit propagation happens server-side.
 */
export function buildSelectionPatch(seasons: SeasonResult[], selection: TVSelection, monitorNewSeasons?: boolean): TVSelectionPatch {
  const patch: TVSelectionPatch = {
    seasons: seasons.flatMap(s => s.id == null ? [] : [{ id: s.id, monitored: selection[s.seasonNumber]?.monitored, qualityProfileID: selection[s.seasonNumber]?.qualityProfileID }]),
    episodes: seasons.flatMap(s => (s.episodes ?? []).flatMap(e => e.id == null ? [] : [{ id: e.id, monitored: selection[s.seasonNumber]?.episodes[e.episodeNumber]?.monitored, qualityProfileID: selection[s.seasonNumber]?.episodes[e.episodeNumber]?.qualityProfileID }])),
  };
  if (typeof monitorNewSeasons === 'boolean') patch.monitorNewSeasons = monitorNewSeasons;
  return patch;
}

const ACTIVE_STATES = ['monitoring', 'pending', 'queued', 'downloading', 'importing', 'downloaded', 'completed'];

function isAired(airDate?: string): boolean {
  if (!airDate) return false;
  const t = new Date(airDate).getTime();
  return Number.isFinite(t) && t <= Date.now();
}

/**
 * UI guard only — the server remains the authority. Returns a human reason
 * when an explicit "Request once" is not offered for this episode.
 */
export function episodeRequestBlockedReason(episode: EpisodeResult): string | null {
  if (episode.id == null) return 'Not in the local library yet';
  const state = (episode.state ?? '').toLowerCase();
  if (episode.downloaded || ACTIVE_STATES.includes(state)) return 'Already downloaded or actively acquired';
  if (!isAired(episode.airDate)) return 'Not yet released';
  return null;
}

function isRequestableEpisode(episode: EpisodeResult): boolean {
  return episode.monitored && episodeRequestBlockedReason(episode) === null;
}

/**
 * A season request includes only its MONITORED aired/missing/inactive children,
 * so it is offered only when at least one such child exists. A valid known
 * zero-episode season falls back to the season air date. (Explicit per-episode
 * requests may still target an unmonitored episode directly.)
 */
export function seasonRequestBlockedReason(season: SeasonResult): string | null {
  if (season.id == null) return 'Not in the local library yet';
  const episodes = season.episodes ?? [];
  if (episodes.length > 0) {
    if (episodes.some(isRequestableEpisode)) return null;
    return episodes.every(e => episodeRequestBlockedReason(e) !== null)
      ? 'All episodes downloaded, active, or unreleased'
      : 'No monitored aired or missing episodes available to request';
  }
  if (!isAired(season.airDate)) return 'Not yet released';
  return null;
}

/**
 * A series request respects season/episode choices, so it is offered only when
 * at least one monitored, aired, non-active episode exists to search for.
 */
export function seriesRequestBlockedReason(seasons: SeasonResult[]): string | null {
  if (!seasons.some(season => (season.episodes ?? []).some(isRequestableEpisode))) {
    return 'No monitored aired or missing episodes available to request';
  }
  return null;
}

/**
 * Returns a human reason why a selection draft must NOT be submitted as a
 * PATCH, distinguishing genuinely incomplete metadata from metadata-only
 * rows that simply have no local library ID yet. Children without local IDs
 * are silently skipped by buildSelectionPatch; submitting a draft that selects
 * them would therefore send a misleading All/None, so it is blocked here.
 * A valid known zero-episode season never blocks.
 */
export function selectionSubmissionProblem(seasons: SeasonResult[], selection: TVSelection): string | null {
  if (hasIncompleteSeasonMetadata(seasons)) {
    return 'Episode metadata is incomplete for one or more seasons; saving selection is unavailable until the full tree loads.';
  }
  for (const season of seasons) {
    const value = selection[season.seasonNumber];
    if (!value) continue;
    if (value.monitored && season.id == null) {
      return `${season.title} is not in the local library yet; it cannot be included in a saved selection.`;
    }
    for (const episode of season.episodes ?? []) {
      if (value.episodes[episode.episodeNumber]?.monitored && episode.id == null) {
        return `S${String(season.seasonNumber).padStart(2, "0")}E${String(episode.episodeNumber).padStart(2, "0")} is not in the local library yet; it cannot be included in a saved selection.`;
      }
    }
  }
  return null;
}

/**
 * True when a season's episode metadata is missing or falsely complete
 * (no episodes but a positive episodeCount). Such metadata must never be
 * silently converted into an empty complete custom selection.
 */
export function hasIncompleteSeasonMetadata(seasons: SeasonResult[]): boolean {
  return seasons.some(season => !Array.isArray(season.episodes) || (season.episodes.length === 0 && season.episodeCount > 0));
}

interface Props {
  seasons: SeasonResult[];
  /**
   * Profiles offered here must already be episode-compatible (the `series`
   * profile type); callers fetch them with useSeriesQualityProfiles. Movie
   * flows keep using movie profiles untouched.
   */
  profiles: QualityProfile[];
  selection: TVSelection;
  onChange: (selection: TVSelection) => void;
  disabled?: boolean;
  previewProfileID?: number;
  /**
   * Compact editor variant for narrow containers (e.g. the 448px request
   * modal). Profile inheritance controls stack at ALL widths instead of only
   * below the sm breakpoint, because media queries follow the viewport and
   * cannot see the modal's true width.
   */
  compact?: boolean;
  /** Explicit per-child requests; handlers receive LOCAL library IDs only. */
  requestOnce?: {
    /** Series-level Request once (closes over the local series ID); shares the same one-in-flight state. */
    series?: () => Promise<void>;
    season: (id: number) => Promise<void>;
    episode: (id: number) => Promise<void>;
  };
}
export function TVSelectionEditor({ seasons, profiles, selection, onChange, disabled, previewProfileID, requestOnce, compact }: Props) {
  const [mode, setMode] = useState<"all" | "none" | "custom">("all");
  const [openSeasons, setOpenSeasons] = useState<Record<number, boolean>>({});
  const [pendingKey, setPendingKey] = useState<string | null>(null);
  const [rowStatus, setRowStatus] = useState<Record<string, { ok?: string; error?: string }>>({});
  const profileName = useMemo(() => new Map(profiles.map(p => [p.id, p.name])), [profiles]);
  const changeSeason = (number: number, monitored: boolean) => {
    const season = selection[number];
    onChange({ ...selection, [number]: { ...season, monitored, episodes: Object.fromEntries(Object.entries(season.episodes).map(([id, episode]) => [id, { ...episode, monitored }])) } });
    setMode("custom");
  };
  const changeEpisode = (seasonNumber: number, episodeNumber: number, patch: Partial<SelectionDraft["episodes"][number]>) => {
    const season = selection[seasonNumber];
    const episodes = { ...season.episodes, [episodeNumber]: { ...season.episodes[episodeNumber], ...patch } };
    onChange({ ...selection, [seasonNumber]: { ...season, monitored: Object.values(episodes).some(e => e.monitored), episodes } });
    setMode("custom");
  };
  const setAll = (next: "all" | "none") => { setMode(next); onChange(setBulkMonitoring(seasons, selection, next === "all")); };
  const runRequest = (key: string, id: number | null, action: (id: number | null) => Promise<void>) => {
    if (pendingKey !== null) return; // one request in flight; prevents duplicate clicks
    setRowStatus(current => ({ ...current, [key]: {} }));
    setPendingKey(key);
    action(id).then(() => {
      setRowStatus(current => ({ ...current, [key]: { ok: 'Request processed; check activity' } }));
    }).catch((error: unknown) => {
      setRowStatus(current => ({ ...current, [key]: { error: error instanceof Error ? error.message : 'Request failed' } }));
    }).finally(() => setPendingKey(null));
  };
  const seriesRequestButton = () => {
    if (!requestOnce?.series) return null;
    const blocked = seriesRequestBlockedReason(seasons);
    const status = rowStatus['series'];
    return <span className="flex items-center gap-2">
      {blocked
        ? <span className="text-xs text-muted-foreground" title={blocked}>Request once unavailable</span>
        : <Button type="button" size="sm" variant="outline" disabled={disabled || pendingKey !== null} aria-label="Request series once"
          onClick={() => runRequest('series', null, () => requestOnce.series!())}>Request once</Button>}
      {status?.ok && <span role="status" className="text-xs text-muted-foreground">{status.ok}</span>}
      {status?.error && <span role="alert" className="text-xs text-destructive">{status.error}</span>}
    </span>;
  };
  const seasonRequestButton = (season: SeasonResult) => {
    if (!requestOnce) return null;
    const blocked = seasonRequestBlockedReason(season);
    const key = `season:${season.seasonNumber}`;
    const status = rowStatus[key];
    return <span className="flex items-center gap-2">
      {blocked
        ? <span className="text-xs text-muted-foreground" title={blocked}>Request once unavailable</span>
        : <Button type="button" size="sm" variant="outline" disabled={disabled || pendingKey !== null} aria-label={`Request season ${season.seasonNumber} once`}
          onClick={() => season.id != null && runRequest(key, season.id, requestOnce.season)}>Request once</Button>}
      {status?.ok && <span role="status" className="text-xs text-muted-foreground">{status.ok}</span>}
      {status?.error && <span role="alert" className="text-xs text-destructive">{status.error}</span>}
    </span>;
  };
  const profileSelect = (current: number | null, effectiveID: number | null | undefined, source: string, label: string, onValue: (id: number | null) => void) => (
    // Stack on small screens so the inherit choice keeps a full usable width
    // instead of being squeezed into a narrow, heavily wrapped column. In the
    // compact variant the stack applies at every width: the modal container is
    // narrow even on large viewports, which viewport media queries cannot see.
    <div className={compact
      ? "flex flex-col gap-1.5 text-xs text-muted-foreground"
      : "flex flex-col gap-1.5 text-xs text-muted-foreground sm:flex-row sm:items-center sm:gap-2"}>
      <Select value={current == null ? "inherit" : String(current)} onValueChange={v => onValue(v === "inherit" ? null : Number(v))} disabled={disabled}>
        <SelectTrigger className={compact ? "h-8 w-full" : "h-8 w-full sm:w-[190px]"} aria-label={label}><SelectValue /></SelectTrigger>
        <SelectContent><SelectItem value="inherit">Inherit profile</SelectItem>{profiles.map(p => <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>)}</SelectContent>
      </Select>
      {/* A null/unresolved effective profile must not falsely imply the series default. */}
      <span>{effectiveID != null
        ? <>Effective: {profileName.get(effectiveID) ?? "unknown profile"} · {source}</>
        : <>Effective profile unavailable (unresolved) · {source}</>}</span>
    </div>
  );
  return <section className="space-y-3" aria-label="Season and episode selection">
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-sm font-medium">Acquisition selection</span>
      <Button type="button" size="sm" variant={mode === "all" ? "default" : "outline"} disabled={disabled} aria-label="Monitor all seasons and episodes" onClick={() => setAll("all")}>All</Button>
      <Button type="button" size="sm" variant={mode === "none" ? "default" : "outline"} disabled={disabled} aria-label="Unmonitor all seasons and episodes" onClick={() => setAll("none")}>None</Button>
      <span className="text-xs text-muted-foreground">{mode === "custom" ? "Custom selection (monitoring changes only; quality overrides kept)" : "Bulk All/None change monitoring only; quality overrides are kept"}</span>
      {seriesRequestButton()}
    </div>
    {seasons.map(season => {
      const value = selection[season.seasonNumber] ?? { monitored: false, qualityProfileID: null, episodes: {} };
      const detailsID = `season-disclosure-${season.seasonNumber}`;
      const seasonOpen = openSeasons[season.seasonNumber] ?? false;
      const episodeValues = Object.values(value.episodes);
      const someMonitored = episodeValues.some(e => e.monitored);
      const allMonitored = episodeValues.length > 0 && episodeValues.every(e => e.monitored);
      const seasonChecked = value.monitored && allMonitored ? true : value.monitored && !allMonitored && someMonitored ? "indeterminate" : value.monitored;
      const seasonEffective = value.qualityProfileID ?? previewProfileID;
      const seasonSource = value.qualityProfileID != null ? "season override" : "series (inherited)";
      // Season identity is primary; acquisition actions sit in a secondary row
      // below the name so the tree scans by season first.
      return <div key={season.seasonNumber} className="border rounded-md p-3">
        <div className="flex flex-wrap items-center gap-2">
          <Checkbox checked={seasonChecked} disabled={disabled} aria-label={`Monitor ${season.title}`}
            onCheckedChange={checked => changeSeason(season.seasonNumber, checked === true)} />
          <span className="font-medium">{season.title}</span>
          <span className="text-xs text-muted-foreground">{season.episodes?.length ?? season.episodeCount} episodes</span>
        </div>
        {/* Accessible disclosure toggle controls the season's episode rows; full
            block reason is available in the tooltip instead of repeated prose. */}
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Button type="button" size="sm" variant="ghost" aria-expanded={seasonOpen} aria-controls={detailsID}
            aria-label={`${seasonOpen ? 'Hide' : 'Show'} episodes for ${season.title}`}
            onClick={() => setOpenSeasons(current => ({ ...current, [season.seasonNumber]: !seasonOpen }))}>
            {seasonOpen ? 'Hide episodes' : 'Show episodes'}
          </Button>
          {seasonRequestButton(season)}
        </div>
        <div id={detailsID} hidden={!seasonOpen} className="mt-3 ml-6 space-y-3">
            {profileSelect(value.qualityProfileID, seasonEffective, seasonSource, `Quality profile override for ${season.title}`,
              id => onChange({ ...selection, [season.seasonNumber]: { ...value, qualityProfileID: id } }))}
            <div className="space-y-3">
              {(season.episodes ?? []).map((episode: EpisodeResult) => {
                const child = value.episodes[episode.episodeNumber] ?? { monitored: false, qualityProfileID: null };
                const active = ["downloading", "importing", "downloaded", "completed"].includes((episode.state ?? "").toLowerCase());
                const blocked = requestOnce ? episodeRequestBlockedReason(episode) : null;
                const key = `episode:${episode.id}`;
                const status = requestOnce ? rowStatus[key] : undefined;
                const effectiveID = child.qualityProfileID ?? value.qualityProfileID ?? previewProfileID;
                const source = child.qualityProfileID != null ? "episode override" : value.qualityProfileID != null ? "season override" : "series (inherited)";
                return <div key={episode.episodeNumber} className="flex flex-wrap items-center gap-3 border-t pt-2">
                  <Checkbox checked={child.monitored} disabled={disabled} aria-label={`Monitor S${String(season.seasonNumber).padStart(2, "0")}E${String(episode.episodeNumber).padStart(2, "0")} ${episode.title}`}
                    onCheckedChange={checked => changeEpisode(season.seasonNumber, episode.episodeNumber, { monitored: checked === true })} />
                  <div className="min-w-48 flex-1"><div className="text-sm">S{String(season.seasonNumber).padStart(2,"0")}E{String(episode.episodeNumber).padStart(2,"0")} · {episode.title}</div><div className="text-xs text-muted-foreground">{episode.airDate || "Air date unknown"} · {episode.state || (episode.downloaded ? "downloaded" : "missing")}{active ? " · active (requests guarded)" : ""}</div></div>
                  {requestOnce && (blocked
                    ? <span className="text-xs text-muted-foreground" title={blocked}>Request once unavailable</span>
                    : <Button type="button" size="sm" variant="outline" disabled={disabled || pendingKey !== null} aria-label={`Request episode S${String(season.seasonNumber).padStart(2, "0")}E${String(episode.episodeNumber).padStart(2, "0")} once`}
                      onClick={() => episode.id != null && runRequest(key, episode.id, requestOnce.episode)}>Request once</Button>)}
                  {status?.ok && <span role="status" className="text-xs text-muted-foreground">{status.ok}</span>}
                  {status?.error && <span role="alert" className="text-xs text-destructive">{status.error}</span>}
                  {profileSelect(child.qualityProfileID, effectiveID, source, `Quality profile override for S${String(season.seasonNumber).padStart(2, "0")}E${String(episode.episodeNumber).padStart(2, "0")}`,
                    id => changeEpisode(season.seasonNumber, episode.episodeNumber, { qualityProfileID: id }))}
                </div>;
              })}
            </div>
          </div>
      </div>;
    })}
  </section>;
}
