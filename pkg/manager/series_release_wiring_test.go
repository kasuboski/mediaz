package manager

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSeriesSearchWiresReleaseIdentity(t *testing.T) {
	for _, mode := range []string{"scheduled episode", "scheduled pack", "manual season", "manual episode"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			store, err := mediaSqlite.New(ctx, ":memory:")
			require.NoError(t, err)
			ctrl := gomock.NewController(t)

			downloadModel := sqlcdb.DownloadClient{ID: 1, Implementation: "transmission", Type: "torrent", Host: "localhost", Port: 9091, Scheme: "http"}
			downloadModel.ID, err = store.CreateDownloadClient(ctx, downloadModel)
			require.NoError(t, err)
			metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 4242, Title: "Original Series", EpisodeCount: 1, ExternalIds: sql.NullString{String: `{"tvdb_id":77}`, Valid: true}})
			require.NoError(t, err)
			seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{SeriesMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}, QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
			require.NoError(t, err)
			seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: metadataID, Title: "Season 1", Number: 1})
			require.NoError(t, err)
			seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true}, Monitored: 1}, storage.SeasonStateMissing)
			require.NoError(t, err)
			episodeMetadataID, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{SeasonMetadataID: seasonMetadataID, TmdbID: 1, Title: "Pilot", Number: 1, AirDate: sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}, Runtime: sql.NullInt64{Int64: 42, Valid: true}})
			require.NoError(t, err)
			episodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: 1, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID, Valid: true}, Monitored: 1}, storage.EpisodeStateMissing)
			require.NoError(t, err)

			releases := []*prowlarr.ReleaseResource{
				{ID: ptr.To(int32(1)), Title: nullable.NewNullableWithValue("Original.Series.S01E01.1080p.WEB-DL-GROUP"), Seeders: nullable.NewNullableWithValue(int32(200)), Size: sizeGBToBytes(2), Protocol: ptr.To(prowlarr.DownloadProtocolTorrent), TvdbID: ptr.To(int32(88))},
				{ID: ptr.To(int32(2)), Title: nullable.NewNullableWithValue("Serie.Traducida.S01E01.1080p.WEB-DL-GROUP"), Seeders: nullable.NewNullableWithValue(int32(1)), Size: sizeGBToBytes(2), Protocol: ptr.To(prowlarr.DownloadProtocolTorrent), TvdbID: ptr.To(int32(77))},
				{ID: ptr.To(int32(3)), Title: nullable.NewNullableWithValue("Serie.Traducida.S02E01.1080p.WEB-DL-GROUP"), Seeders: nullable.NewNullableWithValue(int32(500)), Size: sizeGBToBytes(2), Protocol: ptr.To(prowlarr.DownloadProtocolTorrent), TvdbID: ptr.To(int32(77))},
			}
			if mode == "scheduled pack" {
				for _, r := range releases {
					r.Title = nullable.NewNullableWithValue(strings.ReplaceAll(r.Title.MustGet(), "E01", ""))
				}
			}
			mockIndexer := indexerMock.NewMockIndexerSource(ctrl)
			mockIndexer.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(releases, nil).AnyTimes()
			mockIndexer.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "test", Priority: 1, URI: "http://localhost/indexer"}}, nil)
			indexerFactory := indexerMock.NewMockFactory(ctrl)
			indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexer, nil).AnyTimes()

			sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: "test", Implementation: "prowlarr", Scheme: "http", Host: "localhost", Enabled: true})
			require.NoError(t, err)
			_, err = store.CreateIndexer(ctx, sqlcdb.Indexer{IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true}, Name: "test", Priority: 1, Uri: "http://localhost/indexer", ProwlarrID: sql.NullInt64{Int64: 1, Valid: true}})
			require.NoError(t, err)

			downloadClient := downloadMock.NewMockDownloadClient(ctrl)
			downloadClient.EXPECT().Add(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, req download.AddRequest) (download.Status, error) {
				require.Equal(t, download.AddRequest{Release: releases[1]}, req, "only the translated matching-ID release should be grabbed")
				return download.Status{ID: "accepted", Name: "translated release"}, nil
			})
			downloadFactory := downloadMock.NewMockFactory(ctrl)
			downloadFactory.EXPECT().NewDownloadClient(downloadModel).Return(downloadClient, nil)

			m := New(nil, indexerFactory, nil, store, downloadFactory, config.Manager{}, config.Config{})
			require.NoError(t, m.RefreshIndexerSource(ctx, sourceID))
			wantEpisodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
			require.NoError(t, err)
			require.Len(t, wantEpisodes, 1)
			switch mode {
			case "scheduled episode", "scheduled pack":
				err = m.ReconcileMissingSeries(ctx, newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{downloadModel}))
			case "manual season":
				err = m.SearchForSeason(ctx, seasonID)
			case "manual episode":
				err = m.SearchForEpisode(ctx, episodeID)
			}
			require.NoError(t, err)
			wantEpisodes[0].State = string(storage.EpisodeStateDownloading)
			wantEpisodes[0].DownloadID = sql.NullString{String: "accepted", Valid: true}
			wantEpisodes[0].DownloadClientID = sql.NullInt64{Int64: downloadModel.ID, Valid: true}
			if mode == "scheduled pack" {
				wantEpisodes[0].IsEntireSeasonDownload = sql.NullBool{Bool: true, Valid: true}
			}
			episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
			require.NoError(t, err)
			require.Equal(t, wantEpisodes, episodes)
		})
	}
}
