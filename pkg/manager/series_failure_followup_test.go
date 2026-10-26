package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Compare complete native rows, including unrelated children and file links, not
// just the presence/absence of the series that the operation targeted.
type seriesFailureSnapshot struct {
	series   []sqlcdb.ListSeriesRow
	seasons  []sqlcdb.ListSeasonsRow
	episodes []sqlcdb.ListEpisodesRow
	files    []sqlcdb.EpisodeFile
}

func snapshotSeriesFailure(t *testing.T, ctx context.Context, store storage.Storage) seriesFailureSnapshot {
	t.Helper()
	var got seriesFailureSnapshot
	var err error
	got.series, err = store.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	got.seasons, err = store.ListSeasons(ctx, storage.SeasonFilter{})
	require.NoError(t, err)
	got.episodes, err = store.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	got.files, err = store.ListEpisodeFiles(ctx)
	require.NoError(t, err)
	return got
}

func seedSeriesFailureUnrelated(t *testing.T, ctx context.Context, store storage.Storage) int64 {
	t.Helper()
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
		Path: sql.NullString{String: "Unrelated Show", Valid: true}, QualityProfileID: 1, Monitored: 1,
	}, storage.SeriesStateMissing)
	require.NoError(t, err)
	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	fileID, err := store.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{
		RelativePath:     sql.NullString{String: "Unrelated Show/episode.mkv", Valid: true},
		OriginalFilePath: sql.NullString{String: "/tv/Unrelated Show/episode.mkv", Valid: true}, Size: 4096,
	})
	require.NoError(t, err)
	_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
		SeasonID: seasonID, EpisodeNumber: 1, Monitored: 1,
		EpisodeFileID: sql.NullInt64{Int64: fileID, Valid: true},
	}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	return seriesID
}

func TestSeriesFailureFollowup_AddSeriesChildCleanup(t *testing.T) {
	for _, child := range []string{"season", "episode"} {
		t.Run(child, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "cleanup.sqlite")
			store, err := mediaSqlite.New(ctx, path)
			require.NoError(t, err)
			// A second connection permits real SQLite fault injection without
			// exposing production internals or replacing compensating deletes.
			db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			q := sqlcdb.New(db)
			seedSeriesFailureUnrelated(t, ctx, store)
			before := snapshotSeriesFailure(t, ctx, store)
			metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 9876, Title: "New Show"})
			require.NoError(t, err)
			var seasonIDs, episodeIDs []int64
			for number := int64(1); number <= 2; number++ {
				seasonID, err := q.UpsertSeasonMetadata(ctx, sqlcdb.UpsertSeasonMetadataParams{
					SeriesMetadataID: metadataID, Number: number, TmdbID: 100 + number, Title: fmt.Sprintf("Season %d", number),
				})
				require.NoError(t, err)
				seasonIDs = append(seasonIDs, seasonID)
				for episode := int64(1); episode <= 2; episode++ {
					id, err := q.UpsertEpisodeMetadata(ctx, sqlcdb.UpsertEpisodeMetadataParams{
						SeasonMetadataID: seasonID, Number: episode, TmdbID: 1000 + number*10 + episode, Title: "Episode",
					})
					require.NoError(t, err)
					episodeIDs = append(episodeIDs, id)
				}
			}
			metadata, err := store.GetSeriesMetadataByID(ctx, metadataID)
			require.NoError(t, err)
			svc := NewSeriesService(nil, nil, store, store, NewQualityService(store), &mockMetadataProvider{metadata: &metadata})

			// Fail only after earlier children really exist. This protects the
			// cascading cleanup of partial work, not merely an empty parent.
			condition := fmt.Sprintf("NEW.season_metadata_id = %d", seasonIDs[1])
			if child == "episode" {
				condition = fmt.Sprintf("NEW.episode_metadata_id = %d", episodeIDs[1])
			}
			trigger := fmt.Sprintf(`CREATE TRIGGER fail_child BEFORE INSERT ON %s
				WHEN %s AND EXISTS (SELECT 1 FROM episode WHERE episode_metadata_id = %d)
				BEGIN SELECT RAISE(ABORT, 'injected child creation failure'); END`, child, condition, episodeIDs[0])
			_, err = db.ExecContext(ctx, trigger)
			require.NoError(t, err)
			got, err := svc.AddSeriesToLibrary(ctx, AddSeriesRequest{TMDBID: 9876, QualityProfileID: 1, MonitorNewSeasons: true})
			require.ErrorContains(t, err, "injected child creation failure")
			require.Nil(t, got)
			require.Equal(t, before, snapshotSeriesFailure(t, ctx, store))
			_, err = store.GetSeriesByMetadataID(ctx, metadataID)
			require.ErrorIs(t, err, storage.ErrNotFound)
			// Cascades must also remove transitions for the abandoned rows.
			for _, table := range []string{"series_transition", "season_transition", "episode_transition"} {
				var count int
				require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count))
				require.Equal(t, 1, count, table+" must retain only the unrelated entity's transition")
			}

			// Cleanup removes library entities, not their cached metadata.
			retained, err := store.GetSeriesMetadataByID(ctx, metadataID)
			require.NoError(t, err)
			require.Equal(t, metadata, retained)
			for _, id := range seasonIDs {
				_, err := q.GetSeasonMetadataByID(ctx, id)
				require.NoError(t, err)
			}
			for _, id := range episodeIDs {
				_, err := q.GetEpisodeMetadataByID(ctx, id)
				require.NoError(t, err)
			}
		})
	}
}

func TestSeriesFailureFollowup_DeleteDirectoryPreservesDatabase(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	store := newStore(t, ctx)
	seriesID := seedSeriesFailureUnrelated(t, ctx, store)
	before := snapshotSeriesFailure(t, ctx, store)
	lib := mockLibrary.NewMockLibrary(ctrl)
	injected := errors.New("directory permission denied")
	lib.EXPECT().DeleteSeriesDirectory(ctx, "Unrelated Show").Return(injected)
	svc := NewSeriesService(nil, lib, store, store, nil, nil)
	err := svc.DeleteSeries(ctx, seriesID, true)
	require.ErrorIs(t, err, injected)
	require.ErrorContains(t, err, "failed to delete series directory Unrelated Show")
	require.Equal(t, before, snapshotSeriesFailure(t, ctx, store))
}

func TestSeriesFailureFollowup_IndexListEpisodeFilesErrors(t *testing.T) {
	for _, lookup := range []string{"initial", "after upsert"} {
		t.Run(lookup, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			store := storageMocks.NewMockStorage(ctrl)
			lib := mockLibrary.NewMockLibrary(ctrl)
			file := library.EpisodeFile{RelativePath: "Show/episode.mkv", AbsolutePath: "/tv/Show/episode.mkv", SeriesName: "Show", Size: 1024}
			injected := errors.New("injected ListEpisodeFiles failure: " + lookup)
			calls := []any{lib.EXPECT().FindEpisodes(ctx).Return([]library.EpisodeFile{file}, nil)}
			if lookup == "after upsert" {
				calls = append(calls,
					store.EXPECT().ListEpisodeFiles(ctx).Return([]sqlcdb.EpisodeFile{}, nil),
					store.EXPECT().CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{
						RelativePath: sql.NullString{String: file.RelativePath, Valid: true}, OriginalFilePath: sql.NullString{String: file.AbsolutePath, Valid: true}, Size: file.Size,
					}).Return(int64(42), nil),
				)
			}
			calls = append(calls, store.EXPECT().ListEpisodeFiles(ctx).Return(nil, injected))
			gomock.InOrder(calls...)
			// Strict mocks prohibit upserts after the initial failure and any
			// series/episode association work after the refreshed-list failure.
			m := MediaManager{library: lib, seriesStorage: store}
			require.ErrorIs(t, m.IndexSeriesLibrary(ctx), injected)
		})
	}
}
