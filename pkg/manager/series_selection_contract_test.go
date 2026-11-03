package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"go.uber.org/mock/gomock"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestSelectionContractJSON(t *testing.T) {
	for _, body := range []string{`{"monitored":null}`, `{"seasons":null}`, `{"episodes":null}`, `{"qualityProfileID":null}`, `{"monitorNewSeasons":null}`, `{"seasons":[{"id":1,"monitored":null}]}`} {
		var patch SeriesSelectionPatch
		require.Error(t, json.Unmarshal([]byte(body), &patch), body)
	}
	var patch SeriesSelectionPatch
	require.NoError(t, json.Unmarshal([]byte(`{"seasons":[{"id":1,"qualityProfileID":null}],"episodes":[{"id":2,"qualityProfileID":5}]}`), &patch))
	value := int64(5)
	set := &value
	var inherit *int64
	require.Equal(t, SeriesSelectionPatch{Seasons: []SeasonSelectionUpdate{{ID: 1, QualityProfileID: &inherit}}, Episodes: []EpisodeSelectionUpdate{{ID: 2, QualityProfileID: &set}}}, patch)
	for _, body := range []string{`{"selection":{"mode":"custom","seasons":[{"seasonNumber":1}]}}`, `{"selection":{"mode":"custom","seasons":[{"seasonNumber":1,"monitored":false,"episodes":[{"episodeNumber":1}]}]}}`} {
		var add AddSeriesRequest
		require.Error(t, json.Unmarshal([]byte(body), &add))
	}
}

func TestSelectionContractAtomicAdd(t *testing.T) {
	for _, mode := range []string{"legacy", "all", "none", "custom"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			store, err := mediaSqlite.New(ctx, filepath.Join(t.TempDir(), "selection.sqlite"))
			require.NoError(t, err)
			mid, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 999, Title: "Selection"})
			require.NoError(t, err)
			meta, err := store.GetSeriesMetadataByID(ctx, mid)
			require.NoError(t, err)
			for _, n := range []int64{0, 1} {
				sid, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: mid, Number: n, TmdbID: 100 + n, Title: "Season"})
				require.NoError(t, err)
				_, err = store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{SeasonMetadataID: sid, Number: 1, TmdbID: 200 + n, Title: "Episode"})
				require.NoError(t, err)
			}
			svc := NewSeriesService(nil, nil, store, store, NewQualityService(store), &mockMetadataProvider{metadata: &meta})
			req := AddSeriesRequest{TMDBID: 999, QualityProfileID: 4}
			if mode != "legacy" {
				req.Selection = &SeriesAddSelection{Mode: mode}
			}
			if mode == "custom" {
				req.Selection.Seasons = []SeasonAddSelection{{SeasonNumber: 1, Monitored: true, QualityProfileID: ptrInt64(5), Episodes: []EpisodeAddSelection{{EpisodeNumber: 1, Monitored: true, QualityProfileID: ptrInt64(6)}}}}
			}
			got, err := svc.AddSeriesToLibrary(ctx, req)
			require.NoError(t, err)
			require.Equal(t, &sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: got.Series.ID, SeriesMetadataID: sql.NullInt64{Int64: mid, Valid: true}, QualityProfileID: 4, Monitored: 1, Path: sql.NullString{String: "Selection", Valid: true}, Added: got.Series.Added}, State: string(storage.SeriesStateUnreleased)}, got)
			seasons, err := store.ListSeasons(ctx, storage.SeasonFilter{})
			require.NoError(t, err)
			require.Len(t, seasons, 2)
			for _, row := range seasons {
				flag := int64(0)
				if mode == "all" || mode == "legacy" || mode == "custom" && row.Season.SeasonNumber == 1 {
					flag = 1
				}
				want := sqlcdb.Season{ID: row.Season.ID, SeriesID: got.Series.ID, SeasonNumber: row.Season.SeasonNumber, SeasonMetadataID: row.Season.SeasonMetadataID, Monitored: flag}
				if mode == "custom" && row.Season.SeasonNumber == 1 {
					want.QualityProfileID = sql.NullInt64{Int64: 5, Valid: true}
				}
				require.Equal(t, sqlcdb.ListSeasonsRow{Season: want, State: string(storage.SeasonStateMissing)}, row)
				children, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &row.Season.ID})
				require.NoError(t, err)
				require.Len(t, children, 1)
				child := sqlcdb.Episode{ID: children[0].Episode.ID, SeasonID: row.Season.ID, EpisodeNumber: 1, EpisodeMetadataID: children[0].Episode.EpisodeMetadataID, Monitored: flag}
				if mode == "custom" && row.Season.SeasonNumber == 1 {
					child.QualityProfileID = sql.NullInt64{Int64: 6, Valid: true}
				}
				require.Equal(t, sqlcdb.ListEpisodesRow{Episode: child, State: string(storage.EpisodeStateMissing)}, children[0])
			}
			preview, err := svc.previewSeasons(ctx, mid)
			require.NoError(t, err)
			require.Equal(t, []SeasonResult{
				{TMDBID: 100, Number: 0, Title: "Season", EpisodeCount: 1, Episodes: []EpisodeResult{{TMDBID: 200, Number: 1, Title: "Episode"}}},
				{TMDBID: 101, Number: 1, Title: "Season", EpisodeCount: 1, Episodes: []EpisodeResult{{TMDBID: 201, SeasonNumber: 1, Number: 1, Title: "Episode"}}},
			}, preview)
			detail, err := svc.detailSeasons(ctx, mid, got.Series.ID)
			require.NoError(t, err)
			require.Len(t, detail, 2)
			for _, season := range detail {
				require.NotNil(t, season.ID)
				require.NotNil(t, season.EffectiveQualityProfileID)
				for _, child := range season.Episodes {
					require.NotNil(t, child.ID)
					state := "missing"
					require.Equal(t, &state, child.State)
					require.NotNil(t, child.EffectiveQualityProfileID)
				}
			}
			// Duplicate adds preserve every existing policy field, even with invalid new choices.
			req.QualityProfileID = 1
			req.Selection = &SeriesAddSelection{Mode: "invalid"}
			again, err := svc.AddSeriesToLibrary(ctx, req)
			require.NoError(t, err)
			require.Equal(t, got, again)
		})
	}
}

func TestSelectionContractSparsePatch(t *testing.T) {
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, filepath.Join(t.TempDir(), "patch.sqlite"))
	require.NoError(t, err)
	mid, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 12345, Title: "Patch"})
	require.NoError(t, err)
	meta, err := store.GetSeriesMetadataByID(ctx, mid)
	require.NoError(t, err)
	sid, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: mid, TmdbID: 200, Number: 1, Title: "One"})
	require.NoError(t, err)
	_, err = store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{SeasonMetadataID: sid, TmdbID: 300, Number: 1, Title: "Pilot"})
	require.NoError(t, err)
	client := tmdbMocks.NewMockITmdb(gomock.NewController(t))
	client.EXPECT().TvSeriesDetails(gomock.Any(), int32(12345), nil).DoAndReturn(func(context.Context, int32, *tmdb.TvSeriesDetailsParams, ...tmdb.RequestEditorFn) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	}).AnyTimes()
	svc := NewSeriesService(client, nil, store, store, NewQualityService(store), &mockMetadataProvider{metadata: &meta})
	added, err := svc.AddSeriesToLibrary(ctx, AddSeriesRequest{TMDBID: 12345, QualityProfileID: 4, MonitorNewSeasons: true, Selection: &SeriesAddSelection{Mode: "custom", Seasons: []SeasonAddSelection{{SeasonNumber: 1, Monitored: true, QualityProfileID: ptrInt64(5), Episodes: []EpisodeAddSelection{{EpisodeNumber: 1, Monitored: true, QualityProfileID: ptrInt64(6)}}}}}})
	require.NoError(t, err)
	before, err := svc.GetTVDetailByTMDBID(ctx, 12345)
	require.NoError(t, err)
	seasonID, episodeID := *before.Seasons[0].ID, *before.Seasons[0].Episodes[0].ID
	var patch SeriesSelectionPatch
	require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"monitored":false,"monitorNewSeasons":false,"qualityProfileID":5,"seasons":[{"id":%d,"monitored":false,"qualityProfileID":null}],"episodes":[{"id":%d,"qualityProfileID":null}]}`, seasonID, episodeID)), &patch))
	got, err := svc.UpdateSelection(ctx, added.Series.ID, patch)
	require.NoError(t, err)
	no := false
	source := "series"
	before.Monitored = &no
	before.MonitorNewSeasons = &no
	before.QualityProfileID = ptrInt64(5)
	before.Seasons[0].Monitored = false
	before.Seasons[0].QualityProfileID = nil
	before.Seasons[0].EffectiveQualityProfileID = ptrInt64(5)
	before.Seasons[0].QualityProfileSource = &source
	before.Seasons[0].Episodes[0].QualityProfileID = nil
	before.Seasons[0].Episodes[0].EffectiveQualityProfileID = ptrInt64(5)
	before.Seasons[0].Episodes[0].QualityProfileSource = &source
	require.Equal(t, before, got)
	got, err = svc.UpdateSelection(ctx, added.Series.ID, SeriesSelectionPatch{})
	require.NoError(t, err)
	require.Equal(t, before, got)
	yes := true
	_, err = svc.UpdateSelection(ctx, added.Series.ID, SeriesSelectionPatch{Monitored: &yes, Episodes: []EpisodeSelectionUpdate{{ID: 9999, Monitored: &no}}})
	require.ErrorIs(t, err, storage.ErrNotFound)
	otherID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 4}, storage.SeriesStateMissing)
	require.NoError(t, err)
	foreignSeason, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: otherID, SeasonNumber: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	foreignEpisode, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: foreignSeason, EpisodeNumber: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	_, err = svc.UpdateSelection(ctx, added.Series.ID, SeriesSelectionPatch{Monitored: &yes, Episodes: []EpisodeSelectionUpdate{{ID: foreignEpisode, Monitored: &no}}})
	require.ErrorIs(t, err, storage.ErrNotOwned)
	_, err = svc.UpdateSelection(ctx, added.Series.ID, SeriesSelectionPatch{Monitored: &yes, Seasons: []SeasonSelectionUpdate{{ID: foreignSeason, Monitored: &no}}})
	require.ErrorIs(t, err, storage.ErrNotOwned)
	row, err := store.GetSeriesByID(ctx, added.Series.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), row.Series.Monitored)
	_, err = svc.UpdateSelection(ctx, added.Series.ID, SeriesSelectionPatch{QualityProfileID: ptrInt64(1)})
	require.ErrorIs(t, err, ErrValidation)
	legacy, err := svc.UpdateSeriesMonitored(ctx, added.Series.ID, true)
	require.NoError(t, err)
	require.Equal(t, int64(5), legacy.Series.QualityProfileID)
	child, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	require.Equal(t, int64(1), child.Episode.Monitored)
	require.False(t, child.Episode.QualityProfileID.Valid)
}

func TestSelectionContractInvalidAddDoesNotCreate(t *testing.T) {
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, filepath.Join(t.TempDir(), "invalid.sqlite"))
	require.NoError(t, err)
	mid, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 999, Title: "Selection"})
	require.NoError(t, err)
	meta, err := store.GetSeriesMetadataByID(ctx, mid)
	require.NoError(t, err)
	svc := NewSeriesService(nil, nil, store, store, NewQualityService(store), &mockMetadataProvider{metadata: &meta})
	_, err = svc.AddSeriesToLibrary(ctx, AddSeriesRequest{TMDBID: 999, QualityProfileID: 4, Selection: &SeriesAddSelection{Mode: "custom", Seasons: []SeasonAddSelection{{SeasonNumber: 99}}}})
	require.ErrorIs(t, err, ErrSelectionStale)
	rows, err := store.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = svc.AddSeriesToLibrary(ctx, AddSeriesRequest{TMDBID: 999, QualityProfileID: 1})
	require.ErrorIs(t, err, ErrValidation)
}
