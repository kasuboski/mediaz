package manager

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestSeriesOrdinalFollowup_AddSeriesNativeIdentity(t *testing.T) {
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, filepath.Join(t.TempDir(), "ordinals.sqlite"))
	require.NoError(t, err)
	metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 9876, Title: "Ordinal Show"})
	require.NoError(t, err)
	metadata, err := store.GetSeriesMetadataByID(ctx, metadataID)
	require.NoError(t, err)

	// Include specials, nonconsecutive ordinals, and native int64 values. Reuse
	// episode numbers across seasons to exercise the parent-scoped identity.
	seasonMetadataIDs := make(map[int64]int64)
	episodeMetadataIDs := make(map[int64]map[int64]int64)
	for i, number := range []int64{0, 2, 2147483648} {
		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{
			SeriesMetadataID: metadataID, Number: number, TmdbID: int64(100 + i), Title: fmt.Sprintf("Season %d", number),
		})
		require.NoError(t, err)
		seasonMetadataIDs[number] = seasonMetadataID
		episodeMetadataIDs[number] = make(map[int64]int64)
		for j, episodeNumber := range []int64{0, 3, 2147483649} {
			id, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
				SeasonMetadataID: seasonMetadataID, Number: episodeNumber, TmdbID: int64(1000 + i*10 + j), Title: fmt.Sprintf("Episode %d", episodeNumber),
			})
			require.NoError(t, err)
			episodeMetadataIDs[number][episodeNumber] = id
		}
	}

	svc := NewSeriesService(nil, nil, store, store, NewQualityService(store), &mockMetadataProvider{metadata: &metadata})
	request := AddSeriesRequest{TMDBID: metadata.TmdbID, QualityProfileID: 1, MonitorNewSeasons: true}
	got, err := svc.AddSeriesToLibrary(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Positive(t, got.Series.ID)
	require.True(t, got.Series.Added.Valid)
	require.False(t, got.Series.Added.Time.IsZero())
	require.Equal(t, &sqlcdb.GetSeriesRow{
		Series: sqlcdb.Series{
			ID: got.Series.ID, Path: sql.NullString{String: metadata.Title, Valid: true}, Monitored: 1,
			Added: got.Series.Added, QualityProfileID: 1, SeriesMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}, MonitorNewSeasons: 1,
		},
		State: string(storage.SeriesStateUnreleased),
	}, got)

	seasons, err := store.ListSeasons(ctx, storage.SeasonFilter{})
	require.NoError(t, err)
	require.Len(t, seasons, 3)
	var wantSeasons []sqlcdb.ListSeasonsRow
	var wantEpisodes []sqlcdb.ListEpisodesRow
	seenSeasons := make(map[int64]bool)
	for _, row := range seasons {
		number := row.Season.SeasonNumber
		seasonMetadataID, ok := seasonMetadataIDs[number]
		require.True(t, ok, "unexpected season ordinal %d", number)
		require.False(t, seenSeasons[number], "duplicate season ordinal")
		seenSeasons[number] = true
		require.Positive(t, row.Season.ID)
		wantSeasons = append(wantSeasons, sqlcdb.ListSeasonsRow{
			Season: sqlcdb.Season{ID: row.Season.ID, SeriesID: got.Series.ID, SeasonNumber: number, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true}, Monitored: 1},
			State:  string(storage.SeasonStateMissing),
		})
		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &row.Season.ID})
		require.NoError(t, err)
		require.Len(t, episodes, 3)
		seenEpisodes := make(map[int64]bool)
		for _, episode := range episodes {
			epNumber := episode.Episode.EpisodeNumber
			epMetadataID, ok := episodeMetadataIDs[number][epNumber]
			require.True(t, ok, "unexpected episode ordinal %d", epNumber)
			require.False(t, seenEpisodes[epNumber], "duplicate episode ordinal")
			seenEpisodes[epNumber] = true
			require.Positive(t, episode.Episode.ID)
			wantEpisodes = append(wantEpisodes, sqlcdb.ListEpisodesRow{
				Episode: sqlcdb.Episode{ID: episode.Episode.ID, SeasonID: row.Season.ID, EpisodeNumber: epNumber, Monitored: 1, EpisodeMetadataID: sql.NullInt64{Int64: epMetadataID, Valid: true}},
				State:   string(storage.EpisodeStateMissing),
			})
		}
	}
	require.ElementsMatch(t, wantSeasons, seasons)
	episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	require.ElementsMatch(t, wantEpisodes, episodes)

	// Adding an already managed series must return the complete same row and
	// leave every child (including native identity and transition state) intact.
	again, err := svc.AddSeriesToLibrary(ctx, request)
	require.NoError(t, err)
	require.Equal(t, got, again)
	seasonsAgain, err := store.ListSeasons(ctx, storage.SeasonFilter{})
	require.NoError(t, err)
	require.ElementsMatch(t, wantSeasons, seasonsAgain)
	episodesAgain, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	require.ElementsMatch(t, wantEpisodes, episodesAgain)
}
