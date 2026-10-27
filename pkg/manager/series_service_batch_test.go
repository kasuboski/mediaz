package manager

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type batchSeriesStore struct {
	storage.SeriesStorage
	batchEpisodeReads int
	lastSeasonIDs     []int64
	individualReads   int
	batchEpisodesErr  error
}

func (s *batchSeriesStore) ListEpisodesBySeasonIDs(ctx context.Context, ids []int64) ([]sqlcdb.ListEpisodesBySeasonIDsRow, error) {
	s.batchEpisodeReads++
	s.lastSeasonIDs = append([]int64(nil), ids...)
	if s.batchEpisodesErr != nil {
		return nil, s.batchEpisodesErr
	}
	return s.SeriesStorage.ListEpisodesBySeasonIDs(ctx, ids)
}
func (s *batchSeriesStore) ListEpisodes(ctx context.Context, filter storage.EpisodeFilter) ([]sqlcdb.ListEpisodesRow, error) {
	s.individualReads++
	return s.SeriesStorage.ListEpisodes(ctx, filter)
}

type batchMetadataStore struct {
	storage.SeriesMetadataStorage
	batchSeasonReads  int
	batchEpisodeReads int
	individualReads   int
	batchSeasonsErr   error
}

func (s *batchMetadataStore) ListSeasonMetadataByIDs(ctx context.Context, ids []int64) ([]sqlcdb.SeasonMetadatum, error) {
	s.batchSeasonReads++
	if s.batchSeasonsErr != nil {
		return nil, s.batchSeasonsErr
	}
	return s.SeriesMetadataStorage.ListSeasonMetadataByIDs(ctx, ids)
}
func (s *batchMetadataStore) GetSeasonMetadataByID(ctx context.Context, id int64) (sqlcdb.SeasonMetadatum, error) {
	s.individualReads++
	return s.SeriesMetadataStorage.GetSeasonMetadataByID(ctx, id)
}
func (s *batchMetadataStore) ListEpisodeMetadataByIDs(ctx context.Context, ids []int64) ([]sqlcdb.EpisodeMetadatum, error) {
	s.batchEpisodeReads++
	return s.SeriesMetadataStorage.ListEpisodeMetadataByIDs(ctx, ids)
}

func setupBatchSeries(t *testing.T) (context.Context, storage.Storage, int64, int64) {
	t.Helper()
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	_, err = store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{ID: 1, Name: "test"})
	require.NoError(t, err)
	metaID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 900, Title: "Batch Series"})
	require.NoError(t, err)
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, SeriesMetadataID: sql.NullInt64{Int64: metaID, Valid: true}}, storage.SeriesStateMissing)
	require.NoError(t, err)
	return ctx, store, metaID, seriesID
}

func TestSeriesService_GetSeasonsWithEpisodesBatch(t *testing.T) {
	ctx, store, seriesMetaID, seriesID := setupBatchSeries(t)
	seasonMeta1, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{TmdbID: 101, SeriesMetadataID: seriesMetaID, Number: 1, Title: "One"})
	require.NoError(t, err)
	seasonMeta2, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{TmdbID: 102, SeriesMetadataID: seriesMetaID, Number: 2, Title: "Two"})
	require.NoError(t, err)
	season1, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMeta1, Valid: true}, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	season2, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 2, SeasonMetadataID: sql.NullInt64{Int64: seasonMeta2, Valid: true}}, storage.SeasonStateMissing)
	require.NoError(t, err)
	epMeta, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 201, Title: "Pilot", Number: 1, SeasonMetadataID: seasonMeta1})
	require.NoError(t, err)
	_, err = store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season1, EpisodeNumber: 1, EpisodeMetadataID: sql.NullInt64{Int64: epMeta, Valid: true}, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	_, err = store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season2, EpisodeNumber: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	seriesStore := &batchSeriesStore{SeriesStorage: store}
	metadataStore := &batchMetadataStore{SeriesMetadataStorage: store}
	svc := NewSeriesService(nil, nil, seriesStore, metadataStore, nil, nil)
	got, err := svc.getSeasonsWithEpisodes(ctx, seriesID)
	require.NoError(t, err)
	assert.Equal(t, []SeasonResult{{TMDBID: 101, SeriesID: seriesID, Number: 1, Title: "One", EpisodeCount: 1, Monitored: true, Episodes: []EpisodeResult{{TMDBID: 201, SeriesID: seriesID, SeasonNumber: 1, Number: 1, Title: "Pilot", Monitored: true}}}, {TMDBID: 102, SeriesID: seriesID, Number: 2, Title: "Two", EpisodeCount: 1, Episodes: []EpisodeResult{{SeriesID: seriesID, SeasonNumber: 2, Number: 1, Title: "Episode 1"}}}}, got)
	assert.Equal(t, 1, seriesStore.batchEpisodeReads)
	assert.Zero(t, seriesStore.individualReads)
	assert.Equal(t, 1, metadataStore.batchSeasonReads)
	assert.Zero(t, metadataStore.individualReads)
}

func TestSeriesService_ListEpisodesForSeasonBatchFallback(t *testing.T) {
	for _, tc := range []struct {
		name         string
		failMetadata bool
	}{{"stored number without metadata", false}, {"metadata batch error", true}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, store, metaID, seriesID := setupBatchSeries(t)
			matchingSeason, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 3}, storage.SeasonStateMissing)
			require.NoError(t, err)
			unrelatedSeason, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 4}, storage.SeasonStateMissing)
			require.NoError(t, err)
			_, err = store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: matchingSeason, EpisodeNumber: 1}, storage.EpisodeStateMissing)
			require.NoError(t, err)
			_, err = store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: unrelatedSeason, EpisodeNumber: 9}, storage.EpisodeStateMissing)
			require.NoError(t, err)
			seriesStore := &batchSeriesStore{SeriesStorage: store}
			metadataStore := &batchMetadataStore{SeriesMetadataStorage: store}
			if tc.failMetadata {
				metadataStore.batchSeasonsErr = errors.New("metadata unavailable")
			}
			provider := &mockMetadataProvider{metadata: &sqlcdb.SeriesMetadatum{ID: metaID, TmdbID: 900}}
			svc := NewSeriesService(nil, nil, seriesStore, metadataStore, nil, provider)
			got, err := svc.ListEpisodesForSeason(ctx, 900, 3)
			require.NoError(t, err)
			assert.Equal(t, []EpisodeResult{{SeriesID: seriesID, SeasonNumber: 3, Number: 1, Title: "Episode 1"}}, got)
			assert.Equal(t, 1, seriesStore.batchEpisodeReads)
			assert.Equal(t, []int64{matchingSeason}, seriesStore.lastSeasonIDs)
		})
	}
}

func TestSeriesService_ListEpisodesForSeasonBatchDuplicateMetadataCandidates(t *testing.T) {
	ctx, store, metaID, seriesID := setupBatchSeries(t)
	seasonMeta, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{TmdbID: 303, SeriesMetadataID: metaID, Number: 3, Title: "Three"})
	require.NoError(t, err)
	first, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 2, SeasonMetadataID: sql.NullInt64{Int64: seasonMeta, Valid: true}}, storage.SeasonStateMissing)
	require.NoError(t, err)
	second, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 5, SeasonMetadataID: sql.NullInt64{Int64: seasonMeta, Valid: true}}, storage.SeasonStateMissing)
	require.NoError(t, err)
	unrelatedMeta, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{TmdbID: 404, SeriesMetadataID: metaID, Number: 4, Title: "Four"})
	require.NoError(t, err)
	unrelated, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 4, SeasonMetadataID: sql.NullInt64{Int64: unrelatedMeta, Valid: true}}, storage.SeasonStateMissing)
	require.NoError(t, err)
	_, err = store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: second, EpisodeNumber: 7}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	seriesStore := &batchSeriesStore{SeriesStorage: store}
	metadataStore := &batchMetadataStore{SeriesMetadataStorage: store}
	provider := &mockMetadataProvider{metadata: &sqlcdb.SeriesMetadatum{ID: metaID, TmdbID: 900}}
	svc := NewSeriesService(nil, nil, seriesStore, metadataStore, nil, provider)
	got, err := svc.ListEpisodesForSeason(ctx, 900, 3)
	require.NoError(t, err)
	assert.Equal(t, []EpisodeResult{{SeriesID: seriesID, SeasonNumber: 3, Number: 7, Title: "Episode 7"}}, got)
	assert.Equal(t, 1, seriesStore.batchEpisodeReads)
	assert.Equal(t, []int64{first, second}, seriesStore.lastSeasonIDs)
	assert.NotContains(t, seriesStore.lastSeasonIDs, unrelated)
}

func TestSeriesService_GetSeasonsWithEpisodesBatchErrors(t *testing.T) {
	for _, failMetadata := range []bool{true, false} {
		t.Run(map[bool]string{true: "season metadata", false: "episode batch"}[failMetadata], func(t *testing.T) {
			ctx, store, metaID, seriesID := setupBatchSeries(t)
			seasonMeta, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{TmdbID: 101, SeriesMetadataID: metaID, Number: 1, Title: "One"})
			require.NoError(t, err)
			_, err = store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMeta, Valid: true}}, storage.SeasonStateMissing)
			require.NoError(t, err)
			seriesStore := &batchSeriesStore{SeriesStorage: store}
			metadataStore := &batchMetadataStore{SeriesMetadataStorage: store}
			if failMetadata {
				metadataStore.batchSeasonsErr = errors.New("season metadata failure")
			} else {
				seriesStore.batchEpisodesErr = errors.New("episode batch failure")
			}
			svc := NewSeriesService(nil, nil, seriesStore, metadataStore, nil, nil)
			got, err := svc.getSeasonsWithEpisodes(ctx, seriesID)
			assert.Nil(t, got)
			require.Error(t, err)
			if failMetadata {
				assert.ErrorContains(t, err, "batch list season metadata")
				assert.Zero(t, seriesStore.batchEpisodeReads)
			} else {
				assert.ErrorContains(t, err, "batch list episodes")
				assert.Equal(t, 1, seriesStore.batchEpisodeReads)
			}
		})
	}
}
