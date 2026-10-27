package manager

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type reconcileDownloadBatchCountingStore struct {
	storage.Storage
	listEpisodes  int
	failEpisodeID int64
}

func (s *reconcileDownloadBatchCountingStore) UpdateEpisodeState(ctx context.Context, id int64, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	if id == s.failEpisodeID {
		return sql.ErrConnDone
	}
	return s.Storage.UpdateEpisodeState(ctx, id, state, metadata)
}

func (s *reconcileDownloadBatchCountingStore) ListEpisodes(ctx context.Context, filter storage.EpisodeFilter) ([]sqlcdb.ListEpisodesRow, error) {
	s.listEpisodes++
	return s.Storage.ListEpisodes(ctx, filter)
}

func TestReconcileMissingSeasonWithMetadataDownloadBatch(t *testing.T) {
	for _, tc := range []struct {
		name             string
		partialFailure   bool
		individual       bool
		missingMetadata  bool
		expectedAddCalls int
	}{
		{name: "success", expectedAddCalls: 1},
		{name: "partial failure", partialFailure: true, expectedAddCalls: 1},
		{name: "individual success", individual: true, expectedAddCalls: 2},
		{name: "individual missing metadata", individual: true, missingMetadata: true, expectedAddCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ctx := context.Background()
			db := newStore(t, ctx)
			store := &reconcileDownloadBatchCountingStore{Storage: db}
			client := downloadMock.NewMockDownloadClient(ctrl)
			factory := downloadMock.NewMockFactory(ctrl)
			clientModel := sqlcdb.DownloadClient{ID: 2, Implementation: "transmission", Type: "torrent", Port: 8080, Host: "transmission", Scheme: "http"}
			_, err := db.CreateDownloadClient(ctx, clientModel)
			require.NoError(t, err)
			_, err = db.CreateDownloadClient(ctx, clientModel)
			require.NoError(t, err)
			factory.EXPECT().NewDownloadClient(clientModel).Times(tc.expectedAddCalls).Return(client, nil)
			client.EXPECT().Add(ctx, gomock.Any()).Times(tc.expectedAddCalls).Return(download.Status{ID: "123", Name: "test download"}, nil)

			seriesMetadataID, err := db.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 1, Title: "Series", Status: "Continuing", EpisodeCount: 10})
			require.NoError(t, err)
			seriesID, err := db.CreateSeries(ctx, sqlcdb.Series{ID: 1, QualityProfileID: 1, SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true}}, storage.SeriesStateMissing)
			require.NoError(t, err)
			seasonMetadataID, err := db.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: seriesMetadataID, Title: "Season 1", Number: 1})
			require.NoError(t, err)
			seasonID, err := db.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true}}, storage.SeasonStateMissing)
			require.NoError(t, err)
			seasonMeta, err := db.GetSeasonMetadataByID(ctx, seasonMetadataID)
			require.NoError(t, err)
			episodeMeta := make(map[int64]sqlcdb.EpisodeMetadatum)
			episodeMetadataIDs := make([]int64, 0, 2)
			episodeIDs := make([]int64, 0, 2)
			for n := int64(1); n <= 2; n++ {
				metaID, createErr := db.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: n, Title: "Episode", Number: n, SeasonMetadataID: seasonMetadataID, AirDate: sql.NullTime{Time: time.Now().Add(-2 * time.Hour), Valid: true}, Runtime: sql.NullInt64{Int64: 42, Valid: true}})
				require.NoError(t, createErr)
				meta, getErr := db.GetEpisodeMetadataByID(ctx, metaID)
				require.NoError(t, getErr)
				episodeMeta[metaID] = meta
				episodeMetadataIDs = append(episodeMetadataIDs, metaID)
				episodeID, episodeErr := db.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: n, EpisodeMetadataID: sql.NullInt64{Int64: metaID, Valid: true}}, storage.EpisodeStateMissing)
				require.NoError(t, episodeErr)
				episodeIDs = append(episodeIDs, episodeID)
			}
			if tc.partialFailure {
				store.failEpisodeID = episodeIDs[1]
			}
			if tc.missingMetadata {
				delete(episodeMeta, episodeMetadataIDs[1])
			}
			season, err := db.GetSeasonByID(ctx, seasonID)
			require.NoError(t, err)
			episodes, err := db.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
			require.NoError(t, err)
			store.listEpisodes = 0
			releases := []*prowlarr.ReleaseResource{{ID: ptr.To(int32(1)), Title: nullable.NewNullableWithValue("series.S01.1080p.WEB-DL.HEVC.x265"), Size: sizeGBToBytes(2), Protocol: ptr.To(prowlarr.DownloadProtocolTorrent)}}
			if tc.individual {
				releases = []*prowlarr.ReleaseResource{
					{ID: ptr.To(int32(1)), Title: nullable.NewNullableWithValue("series.S01E01.1080p.WEB-DL.HEVC.x265"), Size: sizeGBToBytes(1), Protocol: ptr.To(prowlarr.DownloadProtocolTorrent)},
					{ID: ptr.To(int32(2)), Title: nullable.NewNullableWithValue("series.S01E02.1080p.WEB-DL.HEVC.x265"), Size: sizeGBToBytes(1), Protocol: ptr.To(prowlarr.DownloadProtocolTorrent)},
				}
			}
			profile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{Name: "HD", MinSize: 15, MaxSize: 1000, PreferredSize: 995, MediaType: "tv"}}}
			snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{clientModel})
			m := New(nil, nil, nil, store, factory, config.Manager{}, config.Config{})
			err = m.reconcileMissingSeasonWithMetadata(ctx, "Series", seasonListRow(season), map[int64][]sqlcdb.ListEpisodesRow{seasonID: episodes}, map[int64]sqlcdb.SeasonMetadatum{seasonMetadataID: seasonMeta}, episodeMeta, snapshot, profile, releases)
			require.NoError(t, err)
			require.LessOrEqual(t, store.listEpisodes, 2)
			expectedEpisodes := append([]sqlcdb.ListEpisodesRow(nil), episodes...)
			for i := range expectedEpisodes {
				if (tc.partialFailure || tc.missingMetadata) && i == 1 {
					continue
				}
				expectedEpisodes[i].State = string(storage.EpisodeStateDownloading)
				expectedEpisodes[i].DownloadID = sql.NullString{String: "123", Valid: true}
				expectedEpisodes[i].DownloadClientID = sql.NullInt64{Int64: 2, Valid: true}
				if !tc.individual {
					expectedEpisodes[i].IsEntireSeasonDownload = sql.NullBool{Bool: true, Valid: true}
				} else {
					expectedEpisodes[i].IsEntireSeasonDownload = sql.NullBool{}
				}
			}
			gotEpisodes, err := db.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
			require.NoError(t, err)
			require.Equal(t, expectedEpisodes, gotEpisodes)
			gotSeason, err := db.GetSeasonByID(ctx, seasonID)
			require.NoError(t, err)
			require.Equal(t, string(storage.SeasonStateDownloading), gotSeason.State.String)
			gotSeries, err := db.GetSeriesByID(ctx, seriesID)
			require.NoError(t, err)
			require.Equal(t, string(storage.SeriesStateDownloading), gotSeries.State)
		})
	}
}
