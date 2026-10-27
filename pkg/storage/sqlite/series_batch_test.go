package sqlite

import (
	"database/sql"
	"slices"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListEpisodesBySeasonIDs(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	seriesID := tvTestSeries(t, ctx, store)
	seasonA, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 2, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	seasonB, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	idA, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonA, EpisodeNumber: 2, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	idB, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonA, EpisodeNumber: 1, Monitored: 1}, storage.EpisodeStateDiscovered)
	require.NoError(t, err)
	idC, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonB, EpisodeNumber: 1, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)

	rows, err := store.ListEpisodesBySeasonIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
	rows, err = store.ListEpisodesBySeasonIDs(ctx, []int64{-1, 999999})
	require.NoError(t, err)
	assert.Empty(t, rows)
	rows, err = store.ListEpisodesBySeasonIDs(ctx, []int64{seasonA, seasonA, 999999, seasonB})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListEpisodesBySeasonIDsRow{
		{Episode: sqlcdb.Episode{ID: idB, SeasonID: seasonA, EpisodeNumber: 1, Monitored: 1}, State: string(storage.EpisodeStateDiscovered)},
		{Episode: sqlcdb.Episode{ID: idA, SeasonID: seasonA, EpisodeNumber: 2, Monitored: 1}, State: string(storage.EpisodeStateMissing)},
		{Episode: sqlcdb.Episode{ID: idC, SeasonID: seasonB, EpisodeNumber: 1, Monitored: 1}, State: string(storage.EpisodeStateMissing)},
	}, rows)

	ids := make([]int64, 901)
	for i := range ids {
		ids[i] = 100000 + int64(i)
	}
	ids[0], ids[900] = seasonB, seasonA
	ids[1] = seasonA
	input := slices.Clone(ids)
	rows, err = store.ListEpisodesBySeasonIDs(ctx, ids)
	require.NoError(t, err)
	assert.Equal(t, input, ids)
	assert.Equal(t, []int64{seasonA, seasonA, seasonB}, []int64{rows[0].Episode.SeasonID, rows[1].Episode.SeasonID, rows[2].Episode.SeasonID})
	assert.Equal(t, []int64{1, 2, 1}, []int64{rows[0].Episode.EpisodeNumber, rows[1].Episode.EpisodeNumber, rows[2].Episode.EpisodeNumber})
}

func TestListSeasonMetadataByIDs(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	seriesID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 100, Title: "Series"})
	require.NoError(t, err)
	first := sqlcdb.SeasonMetadatum{ID: 51, SeriesMetadataID: seriesID, Number: 2, TmdbID: 201, Title: "Second", Overview: sql.NullString{String: "overview", Valid: true}, AirDate: sql.NullTime{Time: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), Valid: true}}
	second := sqlcdb.SeasonMetadatum{ID: 50, SeriesMetadataID: seriesID, Number: 1, TmdbID: 200, Title: "First"}
	_, err = store.CreateSeasonMetadata(ctx, first)
	require.NoError(t, err)
	_, err = store.CreateSeasonMetadata(ctx, second)
	require.NoError(t, err)

	got, err := store.ListSeasonMetadataByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
	got, err = store.ListSeasonMetadataByIDs(ctx, []int64{-1, 999999})
	require.NoError(t, err)
	assert.Empty(t, got)
	got, err = store.ListSeasonMetadataByIDs(ctx, []int64{first.ID, first.ID, second.ID})
	require.NoError(t, err)
	assert.ElementsMatch(t, []sqlcdb.SeasonMetadatum{first, second}, got)

	ids := make([]int64, 901)
	for i := range ids {
		ids[i] = 100000 + int64(i)
	}
	ids[0], ids[900] = first.ID, second.ID
	ids[1] = first.ID
	input := slices.Clone(ids)
	got, err = store.ListSeasonMetadataByIDs(ctx, ids)
	require.NoError(t, err)
	assert.Equal(t, input, ids)
	assert.ElementsMatch(t, []sqlcdb.SeasonMetadatum{first, second}, got)
}

func TestListEpisodeMetadataByIDsChunked(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	seriesID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 300, Title: "Series"})
	require.NoError(t, err)
	seasonID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{ID: 71, SeriesMetadataID: seriesID, Number: 1, TmdbID: 301, Title: "Season"})
	require.NoError(t, err)
	first := sqlcdb.EpisodeMetadatum{ID: 81, SeasonMetadataID: seasonID, Number: 2, TmdbID: 402, Title: "Second", Overview: sql.NullString{String: "overview", Valid: true}, AirDate: sql.NullTime{Time: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), Valid: true}, Runtime: sql.NullInt64{Int64: 45, Valid: true}, StillPath: sql.NullString{String: "/still.jpg", Valid: true}}
	second := sqlcdb.EpisodeMetadatum{ID: 80, SeasonMetadataID: seasonID, Number: 1, TmdbID: 401, Title: "First"}
	_, err = store.CreateEpisodeMetadata(ctx, first)
	require.NoError(t, err)
	_, err = store.CreateEpisodeMetadata(ctx, second)
	require.NoError(t, err)

	got, err := store.ListEpisodeMetadataByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
	got, err = store.ListEpisodeMetadataByIDs(ctx, []int64{-1, 999999})
	require.NoError(t, err)
	assert.Empty(t, got)
	got, err = store.ListEpisodeMetadataByIDs(ctx, []int64{first.ID, first.ID, second.ID})
	require.NoError(t, err)
	assert.ElementsMatch(t, []sqlcdb.EpisodeMetadatum{first, second}, got)

	ids := make([]int64, 905)
	for i := range ids {
		ids[i] = 100000 + int64(i)
	}
	ids[0] = first.ID
	ids[904] = second.ID
	got, err = store.ListEpisodeMetadataByIDs(ctx, ids)
	require.NoError(t, err)
	assert.ElementsMatch(t, []sqlcdb.EpisodeMetadatum{first, second}, got)

	ids = make([]int64, 901)
	for i := range ids {
		ids[i] = 200000 + int64(i)
	}
	ids[0], ids[900] = first.ID, second.ID
	ids[1] = first.ID
	input := slices.Clone(ids)
	got, err = store.ListEpisodeMetadataByIDs(ctx, ids)
	require.NoError(t, err)
	assert.Equal(t, input, ids)
	assert.ElementsMatch(t, []sqlcdb.EpisodeMetadatum{first, second}, got)
}
