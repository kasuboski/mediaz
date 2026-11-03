package manager

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	libraryMocks "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTVFileDiscoveryInheritsSeasonChoice(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, ctx)
	series, err := store.CreateSeries(ctx, sqlcdb.Series{Path: sql.NullString{String: "Series", Valid: true}, Monitored: 0, QualityProfileID: 5}, storage.SeriesStateMissing)
	require.NoError(t, err)
	season, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: series, SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	lib := libraryMocks.NewMockLibrary(ctrl)
	lib.EXPECT().FindEpisodes(ctx).Return([]library.EpisodeFile{{SeriesName: "Series", SeasonNumber: 1, EpisodeNumber: 3, RelativePath: "Series/S01E03.mkv"}}, nil)
	m := New(nil, nil, lib, store, nil, config.Manager{}, config.Config{})
	require.NoError(t, m.IndexSeriesLibrary(ctx))
	episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &season})
	require.NoError(t, err)
	require.Len(t, episodes, 1)
	require.Equal(t, []sqlcdb.ListEpisodesRow{{Episode: sqlcdb.Episode{ID: episodes[0].Episode.ID, SeasonID: season, EpisodeNumber: 3, Monitored: 1, EpisodeFileID: sql.NullInt64{Int64: 1, Valid: true}}, State: string(storage.EpisodeStateDiscovered)}}, episodes)
}

func TestTVDiscoveryPreservesChoicesAndDefaults(t *testing.T) {
	for _, optIn := range []bool{false, true} {
		for _, seasonOn := range []bool{false, true} {
			name := "default"
			if optIn {
				name = "opt_in"
			}
			if seasonOn {
				name += "_existing_season_on"
			}
			t.Run(name, func(t *testing.T) {
				ctx, store, series, season, episodes := admissionFixture(t)
				off := false
				override := sql.NullInt64{Int64: 5, Valid: true}
				require.NoError(t, store.ApplySeriesSelectionPatch(ctx, series.ID, storage.SeriesSelectionPatch{SeriesMonitored: &off, MonitorNewSeasons: &optIn, Seasons: []storage.SeasonSelectionPatch{{ID: season.Season.ID, Monitored: &seasonOn, QualityProfileID: &override.Int64}}, Episodes: []storage.EpisodeSelectionPatch{{ID: episodes[0].Episode.ID, Monitored: &off, QualityProfileID: &override.Int64}}}))
				if optIn || seasonOn {
					require.NoError(t, store.UpdateSeriesState(ctx, series.ID, storage.SeriesStateDownloading, nil))
					if optIn {
						state := storage.SeriesStateContinuing
						if seasonOn {
							state = storage.SeriesStateCompleted
						}
						require.NoError(t, store.UpdateSeriesState(ctx, series.ID, state, nil))
					}
				}
				ctrl := gomock.NewController(t)
				client := tmdbMocks.NewMockITmdb(ctrl)
				client.EXPECT().GetSeriesDetails(gomock.Any(), 1).Return(&tmdb.SeriesDetails{ID: 1, Name: "Series", NumberOfSeasons: 2, NumberOfEpisodes: 4, Seasons: []tmdb.Season{
					{ID: 0, SeasonNumber: 1, Name: "Season 1", Episodes: []tmdb.Episode{{ID: 1, EpisodeNumber: 1, Name: "one", AirDate: "2020-01-01"}, {ID: 2, EpisodeNumber: 2, Name: "two", AirDate: "2020-01-01"}, {ID: 3, EpisodeNumber: 3, Name: "three", AirDate: "2020-01-01"}}},
					{ID: 102, SeasonNumber: 2, Name: "Season 2", Episodes: []tmdb.Episode{{ID: 4, EpisodeNumber: 1, Name: "one", AirDate: "2020-01-01"}}},
				}}, nil)
				client.EXPECT().TvSeriesExternalIds(gomock.Any(), int32(1)).Return(nil, errors.New("offline fixture"))
				client.EXPECT().TvSeriesWatchProviders(gomock.Any(), int32(1)).Return(nil, errors.New("offline fixture"))
				m := New(client, nil, nil, store, nil, config.Manager{}, config.Config{})
				snapshot := newReconcileSnapshot(nil, nil)
				snapshot.time = time.Now()
				// Even a missing, automation-paused series must refresh its children.
				require.NoError(t, m.ReconcileContinuingSeries(ctx, snapshot))
				got, err := store.GetEpisodeByID(ctx, episodes[0].Episode.ID)
				require.NoError(t, err)
				require.Equal(t, int64(0), got.Episode.Monitored)
				require.Equal(t, override, got.Episode.QualityProfileID)
				gotSeason, err := store.GetSeasonByID(ctx, season.Season.ID)
				require.NoError(t, err)
				require.Equal(t, override, gotSeason.Season.QualityProfileID)
				seasonID := season.Season.ID
				children, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
				require.NoError(t, err)
				require.Len(t, children, 3)
				expected := int64(0)
				if seasonOn {
					expected = 1
				}
				require.Equal(t, expected, children[2].Episode.Monitored)
				newSeason, err := store.GetSeasonBySeriesIDAndNumber(ctx, series.ID, 2)
				require.NoError(t, err)
				expected = 0
				if optIn {
					expected = 1
				}
				require.Equal(t, expected, newSeason.Season.Monitored)
				require.Equal(t, "missing", newSeason.State.String)
				seasonID = newSeason.Season.ID
				children, err = store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
				require.NoError(t, err)
				require.Len(t, children, 1)
				require.Equal(t, expected, children[0].Episode.Monitored)
			})
		}
	}
}
