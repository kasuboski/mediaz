package manager

import (
	"context"
	"database/sql"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestFilesystemSeriesIdentityWiring(t *testing.T) {
	for _, tc := range []struct {
		name, folder            string
		files                   []string
		results                 string
		alreadyOwned, wantError bool
	}{
		{"explicit folder ID without search", "The Flash {tmdb-42}", []string{"The Flash {tmdb-42}/Season 01/The.Flash.S01E01.{tmdb-999}.mkv"}, "", false, false},
		{"explicit series filename ID without search", "The Flash", []string{"The Flash/Season 01/The.Flash.{tmdb-42}.S01E01.mkv"}, "", false, false},
		{"filename premiere year and exact folder isolation", "The Flash", []string{"The Flash/Season 01/The.Flash.2014.S01E01.Pilot.mkv", "The Flash Extra/Season 01/Other.{tmdb-99}.S01E01.mkv"}, `[{"id":43,"name":"Wrong Show","first_air_date":"2014-01-01"},{"id":42,"name":"The Flash","first_air_date":"2014-10-07"},{"id":44,"name":"The Flash","first_air_date":"1990-01-01"}]`, false, false},
		{"yearless remakes stay ambiguous", "The Flash", []string{"The Flash/Season 01/S01E01.Pilot.mkv"}, `[{"id":42,"name":"The Flash","first_air_date":"2014-10-07"},{"id":44,"name":"The Flash","first_air_date":"1990-01-01"}]`, false, true},
		{"conflicting folder and filename year", "The Flash (2014)", []string{"The Flash (2014)/Season 01/The.Flash.1990.S01E01.mkv"}, `[{"id":42,"name":"The Flash","first_air_date":"2014-10-07"},{"id":44,"name":"The Flash","first_air_date":"1990-01-01"}]`, false, true},
		{"conflicting explicit IDs", "The Flash {tmdb-42}", []string{"The Flash {tmdb-42}/Season 01/The.Flash.{tmdb-43}.S01E01.mkv"}, "", false, true},
		{"explicit metadata owned by another series", "The Flash {tmdb-42}", nil, "", true, true},
		{"invalid explicit ID", "The Flash {tmdb-0}", nil, "", false, true},
		{"invalid search candidates", "The Flash", nil, `[null,{"name":"The Flash"},{"id":0,"name":"The Flash"}]`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store, err := mediaSqlite.New(ctx, ":memory:")
			require.NoError(t, err)
			metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 42, Title: "The Flash"})
			require.NoError(t, err)
			if tc.alreadyOwned {
				_, err = store.CreateSeries(ctx, sqlcdb.Series{Path: sql.NullString{String: "Other Folder", Valid: true}, QualityProfileID: 1, SeriesMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}}, storage.SeriesStateDiscovered)
				require.NoError(t, err)
			}
			seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{Path: sql.NullString{String: tc.folder, Valid: true}, QualityProfileID: 1}, storage.SeriesStateDiscovered)
			require.NoError(t, err)
			for _, path := range tc.files {
				_, err := store.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{RelativePath: sql.NullString{String: path, Valid: true}})
				require.NoError(t, err)
			}
			before, err := store.GetSeriesByID(ctx, seriesID)
			require.NoError(t, err)
			var searches atomic.Int32
			m := filesystemDiscoveryManager(t, store, func(w http.ResponseWriter, r *http.Request) {
				searches.Add(1)
				if tc.results == "" {
					t.Errorf("unexpected search: %s", r.URL)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if r.URL.Path != "/3/search/tv" {
					t.Errorf("unexpected search path: %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(`{"results":` + tc.results + `}`))
			})
			err = m.linkSeriesMetadata(ctx, before, logger.FromCtx(ctx))
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				before.Series.SeriesMetadataID = sql.NullInt64{Int64: metadataID, Valid: true}
			}
			after, err := store.GetSeriesByID(ctx, seriesID)
			require.NoError(t, err)
			require.Equal(t, before, after)
			if tc.results == "" {
				require.Zero(t, searches.Load())
			} else {
				require.Equal(t, int32(1), searches.Load())
			}
		})
	}
}

type discoveryCountingStore struct {
	storage.Storage
	episodeFileReads int
}

func (s *discoveryCountingStore) ListEpisodeFiles(ctx context.Context) ([]sqlcdb.EpisodeFile, error) {
	s.episodeFileReads++
	return s.Storage.ListEpisodeFiles(ctx)
}

func TestSeriesDiscoveryBatchDoesNotRetryAmbiguousSeriesForEveryEpisode(t *testing.T) {
	ctx := context.Background()
	base, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	store := &discoveryCountingStore{Storage: base}
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{Path: sql.NullString{String: "Brothers", Valid: true}, QualityProfileID: 1}, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1}, storage.SeasonStateDiscovered)
	require.NoError(t, err)
	for _, path := range []string{"Brothers/Season 01/Brothers.S01E01.mkv", "Brothers/Season 01/Brothers.S01E02.mkv"} {
		fileID, err := store.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{RelativePath: sql.NullString{String: path, Valid: true}})
		require.NoError(t, err)
		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: int64(fileID), EpisodeFileID: sql.NullInt64{Int64: fileID, Valid: true}}, storage.EpisodeStateDiscovered)
		require.NoError(t, err)
	}
	before, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	var searches atomic.Int32
	m := filesystemDiscoveryManager(t, store, func(w http.ResponseWriter, r *http.Request) {
		searches.Add(1)
		_, _ = w.Write([]byte(`{"results":[{"id":42,"name":"Brothers","first_air_date":"2014-01-01"},{"id":43,"name":"Brothers","first_air_date":"1990-01-01"}]}`))
	})
	require.NoError(t, m.ReconcileDiscoveredEpisodes(ctx, nil))
	after, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, 1, store.episodeFileReads)
	require.Equal(t, int32(1), searches.Load())
}
