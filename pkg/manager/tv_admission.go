package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

var errTVAlreadyActive = errors.New("TV target already active or present")

type tvRequestScope struct{}
type tvScopeValue struct {
	scope string
	base  context.Context
}

func withTVScope(ctx context.Context, scope string) context.Context {
	base := ctx
	if value, ok := ctx.Value(tvRequestScope{}).(tvScopeValue); ok {
		base = value.base
	}
	return context.WithValue(ctx, tvRequestScope{}, tvScopeValue{scope, base})
}

// admitTVRelease holds the per-series admission lock until submission linkage
// is persisted. Searches remain outside the lock so policy saves can win while
// a slow provider is responding. SQLite reservations protect crash ambiguity.
func (m MediaManager) admitTVRelease(ctx context.Context, snapshot *ReconcileSnapshot, release *indexer.Release, profileID int64, seasonOverride sql.NullInt64, episodes []sqlcdb.ListEpisodesRow, fullSeason ...bool) (int64, download.Status, func(bool), error) {
	noop := func(bool) {}
	if len(episodes) == 0 {
		return 0, download.Status{}, noop, fmt.Errorf("empty TV release coverage")
	}
	season, err := m.seriesStorage.GetSeasonByID(ctx, episodes[0].Episode.SeasonID)
	if err != nil {
		return 0, download.Status{}, noop, err
	}
	unlock := storage.LockTVAdmission(season.Season.SeriesID)
	handedOff := false
	defer func() {
		if !handedOff {
			unlock()
		}
	}()
	fail := func(err error) (int64, download.Status, func(bool), error) { return 0, download.Status{}, noop, err }
	season, err = m.seriesStorage.GetSeasonByID(ctx, season.Season.ID)
	if err != nil {
		return fail(err)
	}
	series, err := m.seriesStorage.GetSeriesByID(ctx, season.Season.SeriesID)
	if err != nil {
		return fail(err)
	}
	value, _ := ctx.Value(tvRequestScope{}).(tvScopeValue)
	scope := value.scope
	if scope == "" {
		scope = "episode"
	}
	if series.Series.QualityProfileID != profileID || season.Season.QualityProfileID != seasonOverride ||
		(scope == "scheduled" && series.Series.Monitored == 0) || ((scope == "scheduled" || scope == "series") && season.Season.Monitored == 0) {
		return fail(fmt.Errorf("TV policy changed during search; request again if still wanted"))
	}
	if season.State.String == string(storage.SeasonStateDownloading) && (season.DownloadID.Valid || season.DownloadClientID.Valid) {
		return fail(errTVAlreadyActive)
	}
	if len(fullSeason) > 0 && fullSeason[0] {
		id := season.Season.ID
		current, err := m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &id})
		if err != nil {
			return fail(err)
		}
		if len(current) != len(episodes) {
			return fail(fmt.Errorf("season coverage changed during search"))
		}
		covered := make(map[int64]bool, len(episodes))
		for _, episode := range episodes {
			covered[episode.Episode.ID] = true
		}
		for _, episode := range current {
			if !covered[episode.Episode.ID] {
				return fail(fmt.Errorf("season coverage changed during search"))
			}
		}
	}
	ids := make([]int64, 0, len(episodes))
	for _, old := range episodes {
		current, err := m.seriesStorage.GetEpisodeByID(ctx, old.Episode.ID)
		if err != nil {
			return fail(err)
		}
		if current.State.String != string(storage.EpisodeStateMissing) || hasActiveDownload(episodeListRow(current)) {
			return fail(errTVAlreadyActive)
		}
		if current.Episode.SeasonID != season.Season.ID || current.Episode.QualityProfileID != old.Episode.QualityProfileID || current.Episode.Monitored != old.Episode.Monitored || (scope != "episode" && current.Episode.Monitored == 0) {
			return fail(fmt.Errorf("episode %d eligibility changed during search", old.Episode.ID))
		}
		ids = append(ids, old.Episode.ID)
	}
	// Resolve all local preflight failures before claiming: these cannot have
	// submitted anything and are safe to retry.
	c := clientForProtocol(snapshot.GetDownloadClients(), indexer.DownloadProtocol(*release.Protocol))
	if c == nil {
		return fail(fmt.Errorf("no download client found for protocol: %s", *release.Protocol))
	}
	client, err := m.downloadClientService.buildRuntimeDownloadClient(ctx, *c)
	if err != nil {
		return fail(fmt.Errorf("failed to create download client: %w", err))
	}
	reservations := m.seriesStorage
	if err := reservations.ReserveTVEpisodes(ctx, ids); err != nil {
		return fail(err)
	}
	finish := func(linked bool) {
		defer unlock()
		if linked {
			// A canceled HTTP request must not prevent cleanup of a known linkage.
			_ = reservations.ReleaseTVEpisodes(context.WithoutCancel(ctx), ids)
		}
	}
	addCtx := ctx
	if value.base != nil {
		addCtx = value.base
	}
	status, err := client.Add(addCtx, download.AddRequest{Release: release})
	if err != nil {
		var rejected *download.SubmissionRejectedError
		if errors.As(err, &rejected) {
			_ = reservations.ReleaseTVEpisodes(context.WithoutCancel(ctx), ids)
			return c.ID, status, noop, err
		}
		return c.ID, status, noop, fmt.Errorf("TV submission outcome uncertain; admission retained, verify downloader before manual recovery: %w", err)
	}
	if status.ID == "" {
		return c.ID, status, noop, fmt.Errorf("TV submission returned no job ID; admission retained pending manual recovery")
	}
	handedOff = true
	return c.ID, status, finish, nil
}
