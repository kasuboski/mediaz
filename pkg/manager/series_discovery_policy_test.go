package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	libraryMocks "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSeriesDiscoveryMissingProfileIsPublicError(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, ctx)
	profiles, err := store.ListQualityProfilesByMediaType(ctx, "episode")
	require.NoError(t, err)
	for _, profile := range profiles {
		require.NoError(t, store.DeleteQualityProfile(ctx, profile.ID))
	}
	movies, err := store.ListQualityProfilesByMediaType(ctx, "movie")
	require.NoError(t, err)
	require.NotEmpty(t, movies) // A movie profile must not mask a missing TV profile.
	lib := libraryMocks.NewMockLibrary(gomock.NewController(t))
	file := library.EpisodeFile{RelativePath: "Show/episode.mkv", SeriesName: "Show", SeasonNumber: 1, EpisodeNumber: 1}
	lib.EXPECT().FindEpisodes(gomock.Any()).Return([]library.EpisodeFile{file}, nil)
	m := New(nil, nil, lib, store, nil, config.Manager{}, config.Config{})
	// Exercise the same registered public executor as a SeriesIndex job.
	jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: string(SeriesIndex)}, storage.JobStatePending)
	require.NoError(t, err)
	m.jobService.scheduler.executeJob(ctx, sqlcdb.ListJobsRow{Job: sqlcdb.Job{ID: jobID, Type: string(SeriesIndex)}})
	job, err := store.GetJob(ctx, jobID)
	require.NoError(t, err)
	require.Equal(t, string(storage.JobStateError), job.State)
	require.True(t, job.Error.Valid)
	require.Contains(t, job.Error.String, "cannot discover series without a TV quality profile")
	got := snapshotSeriesFailure(t, ctx, store)
	require.Empty(t, got.series)
	require.Empty(t, got.seasons)
	require.Empty(t, got.episodes)
	require.Len(t, got.files, 1) // Successful file writes are intentionally not rolled back.
}

func TestSeriesDiscoveryPropagatesPersistenceErrors(t *testing.T) {
	for _, stage := range []string{"file", "episode lookup", "series lookup", "profile", "series", "season", "episode"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			store := storageMocks.NewMockStorage(ctrl)
			lib := libraryMocks.NewMockLibrary(ctrl)
			file := library.EpisodeFile{RelativePath: "Show/episode.mkv", SeriesName: "Show", SeasonNumber: 1, EpisodeNumber: 1}
			row := episodeFileFromLibrary(file)
			row.ID = 1
			injected := errors.New("injected " + stage + " failure")
			lib.EXPECT().FindEpisodes(ctx).Return([]library.EpisodeFile{file}, nil)
			store.EXPECT().ListEpisodeFiles(ctx).Return([]sqlcdb.EpisodeFile{}, nil)
			if stage == "file" {
				store.EXPECT().CreateEpisodeFile(ctx, episodeFileFromLibrary(file)).Return(int64(0), injected)
				store.EXPECT().ListEpisodeFiles(ctx).Return([]sqlcdb.EpisodeFile{}, nil)
			} else {
				store.EXPECT().CreateEpisodeFile(ctx, episodeFileFromLibrary(file)).Return(int64(1), nil)
				store.EXPECT().ListEpisodeFiles(ctx).Return([]sqlcdb.EpisodeFile{row}, nil)
				lookupErr := error(storage.ErrNotFound)
				if stage == "episode lookup" {
					lookupErr = injected
				}
				store.EXPECT().GetEpisodeByEpisodeFileID(ctx, int64(1)).Return(sqlcdb.GetEpisodeByEpisodeFileIDRow{}, lookupErr)
				if stage != "episode lookup" {
					seriesErr := error(storage.ErrNotFound)
					if stage == "series lookup" {
						seriesErr = injected
					}
					store.EXPECT().GetSeriesByPath(ctx, "Show").Return(sqlcdb.GetSeriesRow{}, seriesErr)
					if stage != "series lookup" {
						if stage == "profile" {
							store.EXPECT().ListQualityProfilesByMediaType(ctx, "episode").Return(nil, injected)
						} else {
							store.EXPECT().ListQualityProfilesByMediaType(ctx, "episode").Return([]sqlcdb.QualityProfile{{ID: 2}}, nil)
							var seriesErr error
							if stage == "series" {
								seriesErr = injected
							}
							store.EXPECT().CreateSeries(ctx, sqlcdb.Series{Path: sql.NullString{String: "Show", Valid: true}, QualityProfileID: 2}, storage.SeriesStateDiscovered).Return(int64(3), seriesErr)
							if stage != "series" {
								store.EXPECT().GetSeasonBySeriesIDAndNumber(ctx, int64(3), int64(1)).Return(sqlcdb.GetSeasonBySeriesIDAndNumberRow{}, storage.ErrNotFound)
								var seasonErr error
								if stage == "season" {
									seasonErr = injected
								}
								store.EXPECT().CreateSeason(ctx, sqlcdb.Season{SeriesID: 3, SeasonNumber: 1}, storage.SeasonStateDiscovered).Return(int64(4), seasonErr)
								if stage == "episode" {
									store.EXPECT().CreateEpisode(ctx, sqlcdb.Episode{SeasonID: 4, EpisodeNumber: 1, EpisodeFileID: sql.NullInt64{Int64: 1, Valid: true}}, storage.EpisodeStateDiscovered).Return(int64(0), injected)
								}
							}
						}
					}
				}
			}
			m := New(nil, nil, lib, store, nil, config.Manager{}, config.Config{})
			require.ErrorIs(t, m.IndexSeriesLibrary(ctx), injected)
		})
	}
}

func TestSeriesDiscoveryContinuesAfterPersistenceFailure(t *testing.T) {
	for _, table := range []string{"episode_file", "series", "season", "episode"} {
		t.Run(table, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "discovery.sqlite")
			store, err := mediaSqlite.New(ctx, path)
			require.NoError(t, err)
			db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			// Only Bad fails; a second independent file must still be indexed.
			conditions := map[string]string{
				"episode_file": "NEW.relative_path = 'Bad/episode.mkv'",
				"series":       "NEW.path = 'Bad'",
				"season":       "NEW.series_id = (SELECT id FROM series WHERE path = 'Bad')",
				"episode":      "NEW.episode_file_id = (SELECT id FROM episode_file WHERE relative_path = 'Bad/episode.mkv')",
			}
			_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE TRIGGER fail_discovery BEFORE INSERT ON %s WHEN %s BEGIN SELECT RAISE(ABORT, 'injected discovery failure'); END", table, conditions[table]))
			require.NoError(t, err)
			lib := libraryMocks.NewMockLibrary(gomock.NewController(t))
			files := []library.EpisodeFile{
				{RelativePath: "Bad/episode.mkv", SeriesName: "Bad", SeasonNumber: 1, EpisodeNumber: 1},
				{RelativePath: "Good/episode.mkv", SeriesName: "Good", SeasonNumber: 2, EpisodeNumber: 3, Size: 1024},
			}
			lib.EXPECT().FindEpisodes(ctx).Return(files, nil).Times(2)
			m := New(nil, nil, lib, store, nil, config.Manager{}, config.Config{})
			require.ErrorContains(t, m.IndexSeriesLibrary(ctx), "injected discovery failure")
			before := snapshotSeriesFailure(t, ctx, store)
			good, err := store.GetSeriesByPath(ctx, "Good")
			require.NoError(t, err)
			profiles, err := store.ListQualityProfilesByMediaType(ctx, "episode")
			require.NoError(t, err)
			require.Equal(t, sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: good.Series.ID, Path: sql.NullString{String: "Good", Valid: true}, QualityProfileID: profiles[0].ID, Added: good.Series.Added}, State: string(storage.SeriesStateDiscovered)}, good)
			season, err := store.GetSeasonBySeriesIDAndNumber(ctx, good.Series.ID, 2)
			require.NoError(t, err)
			require.Equal(t, sqlcdb.GetSeasonBySeriesIDAndNumberRow{Season: sqlcdb.Season{ID: season.Season.ID, SeriesID: good.Series.ID, SeasonNumber: 2}, State: sql.NullString{String: string(storage.SeasonStateDiscovered), Valid: true}}, season)
			require.Len(t, before.episodes, 1)
			episode := before.episodes[0]
			require.Equal(t, sqlcdb.ListEpisodesRow{Episode: sqlcdb.Episode{ID: episode.Episode.ID, SeasonID: season.Season.ID, EpisodeNumber: 3, EpisodeFileID: episode.Episode.EpisodeFileID}, State: string(storage.EpisodeStateDiscovered)}, episode)
			goodFile, err := store.GetEpisodeFile(ctx, episode.Episode.EpisodeFileID.Int64)
			require.NoError(t, err)
			expectedFile := episodeFileFromLibrary(files[1])
			expectedFile.ID, expectedFile.Added = goodFile.ID, goodFile.Added
			require.Equal(t, expectedFile, goodFile)
			// Retry reports the remaining failure without duplicating successful rows.
			require.ErrorContains(t, m.IndexSeriesLibrary(ctx), "injected discovery failure")
			require.Equal(t, before, snapshotSeriesFailure(t, ctx, store))
		})
	}
}

func TestSeriesDiscoveryJoinsFailuresBeforeRefreshError(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	store := storageMocks.NewMockStorage(ctrl)
	lib := libraryMocks.NewMockLibrary(ctrl)
	files := []library.EpisodeFile{{RelativePath: "First/episode.mkv"}, {RelativePath: "Second/episode.mkv"}}
	first, second, refresh := errors.New("first write"), errors.New("second write"), errors.New("refresh")
	gomock.InOrder(
		lib.EXPECT().FindEpisodes(ctx).Return(files, nil),
		store.EXPECT().ListEpisodeFiles(ctx).Return([]sqlcdb.EpisodeFile{}, nil),
		store.EXPECT().CreateEpisodeFile(ctx, episodeFileFromLibrary(files[0])).Return(int64(0), first),
		store.EXPECT().CreateEpisodeFile(ctx, episodeFileFromLibrary(files[1])).Return(int64(0), second),
		store.EXPECT().ListEpisodeFiles(ctx).Return(nil, refresh),
	)
	m := MediaManager{library: lib, seriesStorage: store}
	err := m.IndexSeriesLibrary(ctx)
	for _, failure := range []error{first, second, refresh} {
		require.ErrorIs(t, err, failure)
	}
	require.ErrorContains(t, err, "First/episode.mkv")
	require.ErrorContains(t, err, "Second/episode.mkv")
}
