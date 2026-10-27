package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tvTestEpisode creates a series, season and episode and returns the episode ID.
func tvTestEpisode(t *testing.T, ctx context.Context, store storage.Storage, episodeNumber int64) int64 {
	t.Helper()
	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: tvTestSeries(t, ctx, store), SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	id, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: episodeNumber, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	return id
}

func TestEpisodeStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id := tvTestEpisode(t, ctx, store, 1)
	got, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	episode := sqlcdb.Episode{ID: id, SeasonID: got.Episode.SeasonID, EpisodeNumber: 1, Monitored: 1}
	assert.Equal(t, sqlcdb.GetEpisodeByIDRow{Episode: episode, State: sql.NullString{String: string(storage.EpisodeStateMissing), Valid: true}}, got)
	seasonID := episode.SeasonID
	list, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListEpisodesRow{{Episode: episode, State: string(storage.EpisodeStateMissing)}}, list)
	require.NoError(t, store.DeleteEpisode(ctx, id))
	list, err = store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
	require.NoError(t, err)
	assert.Empty(t, list)
	_, err = store.GetEpisodeByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestEpisodeStorage_CreateWithID(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: tvTestSeries(t, ctx, store), SeasonNumber: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	episode := sqlcdb.Episode{ID: 100, SeasonID: seasonID, EpisodeNumber: 7, Monitored: 1}
	id, err := store.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	require.NoError(t, err)
	assert.Equal(t, episode.ID, id)
	before, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, episode, before.Episode)
	update := episode
	update.EpisodeNumber = 9
	upsertID, err := store.CreateEpisode(ctx, update, storage.EpisodeStateDiscovered)
	require.NoError(t, err)
	assert.Equal(t, id, upsertID)
	after, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	before.Episode = update
	assert.Equal(t, before, after, "entity refresh must preserve its existing transition state")
}

func TestEpisodeStorage_ListFilters(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	a := tvTestEpisode(t, ctx, store, 1)
	first, err := store.GetEpisodeByID(ctx, a)
	require.NoError(t, err)
	seasonID := first.Episode.SeasonID
	b, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: 2, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	other := tvTestEpisode(t, ctx, store, 3)
	require.NoError(t, store.UpdateEpisodeState(ctx, b, storage.EpisodeStateDownloading, nil))
	require.NoError(t, store.UpdateEpisodeState(ctx, b, storage.EpisodeStateDownloaded, nil))
	for _, tc := range []struct {
		name   string
		filter storage.EpisodeFilter
		ids    []int64
	}{
		{"all", storage.EpisodeFilter{}, []int64{a, b, other}},
		{"season", storage.EpisodeFilter{SeasonID: &seasonID}, []int64{a, b}},
		{"missing", storage.EpisodeFilter{SeasonID: &seasonID, States: []storage.EpisodeState{storage.EpisodeStateMissing}}, []int64{a}},
		{"downloaded", storage.EpisodeFilter{SeasonID: &seasonID, States: []storage.EpisodeState{storage.EpisodeStateDownloaded}}, []int64{b}},
		{"empty", storage.EpisodeFilter{States: []storage.EpisodeState{storage.EpisodeStateCompleted}}, nil},
		{"many states", storage.EpisodeFilter{SeasonID: &seasonID, States: []storage.EpisodeState{storage.EpisodeStateNew, storage.EpisodeStateCompleted, storage.EpisodeStateDiscovered, storage.EpisodeStateDownloading, storage.EpisodeStateMissing, storage.EpisodeStateDownloaded}}, []int64{a, b}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := store.ListEpisodes(ctx, tc.filter)
			require.NoError(t, err)
			var ids []int64
			for _, row := range rows {
				ids = append(ids, row.Episode.ID)
			}
			assert.ElementsMatch(t, tc.ids, ids)
		})
	}
}

func TestUpdateEpisodeState(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id := tvTestEpisode(t, ctx, store, 1)
	before, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	clientID, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "torrent", Implementation: "transmission", Host: "localhost", Scheme: "http"})
	require.NoError(t, err)
	require.NoError(t, store.UpdateEpisodeState(ctx, id, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{DownloadID: ptr.To("12"), DownloadClientID: &clientID, IsEntireSeasonDownload: ptr.To(true)}))
	got, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetEpisodeByIDRow{Episode: before.Episode, State: sql.NullString{String: string(storage.EpisodeStateDownloading), Valid: true}, DownloadID: sql.NullString{String: "12", Valid: true}, DownloadClientID: sql.NullInt64{Int64: clientID, Valid: true}, IsEntireSeasonDownload: sql.NullBool{Bool: true, Valid: true}}, got)
	assert.ErrorIs(t, store.UpdateEpisodeState(ctx, 999999, storage.EpisodeStateMissing, nil), storage.ErrNotFound)
}

func TestUpdateEpisodeState_InvalidTransition(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id := tvTestEpisode(t, ctx, store, 1)
	require.NoError(t, store.UpdateEpisodeState(ctx, id, storage.EpisodeStateDiscovered, nil))
	before, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	require.Error(t, store.UpdateEpisodeState(ctx, id, storage.EpisodeStateDownloading, nil))
	after, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestEpisodeStorage_GetByEpisodeFileID(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id := tvTestEpisode(t, ctx, store, 1)
	fileID, err := store.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{Quality: "HDTV-720p", RelativePath: sql.NullString{String: "episode.mkv", Valid: true}})
	require.NoError(t, err)
	before, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	require.NoError(t, store.UpdateEpisodeEpisodeFileID(ctx, id, fileID))
	before.Episode.EpisodeFileID = sql.NullInt64{Int64: fileID, Valid: true}
	got, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, got)
	byFile, err := store.GetEpisodeByEpisodeFileID(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetEpisodeByEpisodeFileIDRow{Episode: before.Episode, State: before.State}, byFile)
	clientID, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "torrent", Implementation: "transmission", Host: "localhost", Scheme: "http"})
	require.NoError(t, err)
	require.NoError(t, store.UpdateEpisodeState(ctx, id, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{DownloadID: ptr.To("bundle-1"), DownloadClientID: &clientID, IsEntireSeasonDownload: ptr.To(true)}))
	byFile, err = store.GetEpisodeByEpisodeFileID(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetEpisodeByEpisodeFileIDRow{Episode: before.Episode, State: sql.NullString{String: string(storage.EpisodeStateDownloading), Valid: true}, DownloadID: sql.NullString{String: "bundle-1", Valid: true}, DownloadClientID: sql.NullInt64{Int64: clientID, Valid: true}, IsEntireSeasonDownload: sql.NullBool{Bool: true, Valid: true}}, byFile)
	_, err = store.GetEpisodeByEpisodeFileID(ctx, 99999)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
