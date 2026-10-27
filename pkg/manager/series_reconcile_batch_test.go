package manager

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMocks "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type reconcileBatchCountingStore struct {
	storage.Storage
	episodeMetadataByID      int
	seasonMetadataByID       int
	listEpisodes             int
	listEpisodesBySeasonIDs  int
	listSeasonMetadataByIDs  int
	listEpisodeMetadataByIDs int
}

func (s *reconcileBatchCountingStore) GetEpisodeMetadataByID(ctx context.Context, id int64) (sqlcdb.EpisodeMetadatum, error) {
	s.episodeMetadataByID++
	return s.Storage.GetEpisodeMetadataByID(ctx, id)
}

func (s *reconcileBatchCountingStore) GetSeasonMetadataByID(ctx context.Context, id int64) (sqlcdb.SeasonMetadatum, error) {
	s.seasonMetadataByID++
	return s.Storage.GetSeasonMetadataByID(ctx, id)
}

func (s *reconcileBatchCountingStore) ListEpisodesBySeasonIDs(ctx context.Context, ids []int64) ([]sqlcdb.ListEpisodesBySeasonIDsRow, error) {
	s.listEpisodesBySeasonIDs++
	return s.Storage.ListEpisodesBySeasonIDs(ctx, ids)
}

func (s *reconcileBatchCountingStore) ListSeasonMetadataByIDs(ctx context.Context, ids []int64) ([]sqlcdb.SeasonMetadatum, error) {
	s.listSeasonMetadataByIDs++
	return s.Storage.ListSeasonMetadataByIDs(ctx, ids)
}

func (s *reconcileBatchCountingStore) ListEpisodeMetadataByIDs(ctx context.Context, ids []int64) ([]sqlcdb.EpisodeMetadatum, error) {
	s.listEpisodeMetadataByIDs++
	return s.Storage.ListEpisodeMetadataByIDs(ctx, ids)
}

func (s *reconcileBatchCountingStore) ListEpisodes(ctx context.Context, filter storage.EpisodeFilter) ([]sqlcdb.ListEpisodesRow, error) {
	s.listEpisodes++
	return s.Storage.ListEpisodes(ctx, filter)
}

func TestReconcileMissingSeriesUsesPreloadedMetadataBatches(t *testing.T) {
	ctx := context.Background()
	db, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	store := &reconcileBatchCountingStore{Storage: db}

	seriesMetadataID, err := db.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 101, Title: "Batch Show"})
	require.NoError(t, err)
	seriesID, err := db.CreateSeries(ctx, sqlcdb.Series{
		QualityProfileID: 1, Monitored: 1,
		SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	var partialSeasonID int64
	for seasonNumber := int64(1); seasonNumber <= 3; seasonNumber++ {
		seasonMetadataID, err := db.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{
			SeriesMetadataID: seriesMetadataID, Number: seasonNumber, Title: "Season",
		})
		require.NoError(t, err)
		seasonID, err := db.CreateSeason(ctx, sqlcdb.Season{
			SeriesID: seriesID, SeasonNumber: seasonNumber, Monitored: 1,
			SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)
		if seasonNumber == 3 {
			partialSeasonID = seasonID
		}
		for episodeNumber := int64(1); episodeNumber <= 2; episodeNumber++ {
			metaID, err := db.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
				TmdbID: 1000 + seasonNumber*10 + episodeNumber, Number: seasonNumber*10 + episodeNumber,
				SeasonMetadataID: seasonMetadataID, Title: "Episode",
				AirDate: sql.NullTime{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			})
			require.NoError(t, err)
			state := storage.EpisodeStateMissing
			episodeID, err := db.CreateEpisode(ctx, sqlcdb.Episode{
				SeasonID: seasonID, EpisodeNumber: episodeNumber, Monitored: 1,
				EpisodeMetadataID: sql.NullInt64{Int64: metaID, Valid: true},
			}, state)
			require.NoError(t, err)
			if seasonNumber == 3 && episodeNumber == 1 {
				require.NoError(t, db.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, nil))
				require.NoError(t, db.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloaded, nil))
			}
		}
	}
	seriesRow, err := db.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	series := sqlcdb.ListSeriesRow{Series: seriesRow.Series, State: seriesRow.State}
	ctrl := gomock.NewController(t)
	indexerSource := indexerMocks.NewMockIndexerSource(ctrl)
	indexerSource.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	indexerFactory := indexerMocks.NewMockFactory(ctrl)
	indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(indexerSource, nil).AnyTimes()
	sourceID, err := db.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: "test-source", Implementation: "prowlarr", Host: "localhost", Scheme: "http", Enabled: true})
	require.NoError(t, err)
	_, err = db.CreateIndexer(ctx, sqlcdb.Indexer{IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true}, Name: "test", Priority: 1, Uri: "http://test/indexer", ProwlarrID: sql.NullInt64{Int64: 1, Valid: true}})
	require.NoError(t, err)
	indexerSource.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "test", Priority: 1}}, nil)
	manager := New(nil, indexerFactory, nil, store, nil, config.Manager{}, config.Config{})
	require.NoError(t, manager.RefreshIndexerSource(ctx, sourceID))
	snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, nil)
	snapshot.time = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, manager.reconcileMissingSeries(ctx, series, snapshot))
	require.Equal(t, 1, store.listEpisodesBySeasonIDs)
	require.Equal(t, 1, store.listSeasonMetadataByIDs)
	require.Equal(t, 1, store.listEpisodeMetadataByIDs)

	require.Zero(t, store.episodeMetadataByID, "episode metadata must be batch-loaded, not fetched per episode")
	require.Zero(t, store.seasonMetadataByID, "season metadata must be batch-loaded, not fetched per season")
	require.Zero(t, store.listEpisodes, "episodes must be batch-loaded, including the partially completed season")
	seasons, err := db.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	require.NoError(t, err)
	for _, season := range seasons {
		require.Equal(t, string(storage.SeasonStateMissing), season.State, "no releases should change season state")
	}
	episodes, err := db.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &partialSeasonID})
	require.NoError(t, err)
	require.Len(t, episodes, 2)
	require.Equal(t, string(storage.EpisodeStateDownloaded), episodes[0].State)
}
