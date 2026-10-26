package manager

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTVNativeDiscoveredSeriesUsesTVQualityProfile(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, ctx)
	m := MediaManager{seriesStorage: store, qualityService: NewQualityService(store)}
	profiles, err := store.ListQualityProfilesByMediaType(ctx, "episode")
	require.NoError(t, err)
	require.NotEmpty(t, profiles)
	id, err := m.ensureSeries(ctx, "Discovered Show")
	require.NoError(t, err)
	got, err := store.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	require.True(t, got.Series.Added.Valid)
	require.Equal(t, sqlcdb.GetSeriesRow{
		Series: sqlcdb.Series{ID: id, Path: sql.NullString{String: "Discovered Show", Valid: true}, Added: got.Series.Added, QualityProfileID: profiles[0].ID},
		State:  string(storage.SeriesStateDiscovered),
	}, got)
}

func TestTVNativeDiscoveredSeriesRequiresTVQualityProfile(t *testing.T) {
	ctx := context.Background()
	store := storageMocks.NewMockStorage(gomock.NewController(t))
	store.EXPECT().GetSeriesByPath(ctx, "Show").Return(sqlcdb.GetSeriesRow{}, storage.ErrNotFound)
	store.EXPECT().ListQualityProfilesByMediaType(ctx, "episode").Return([]sqlcdb.QualityProfile{}, nil)
	m := MediaManager{seriesStorage: store, qualityService: NewQualityService(store)}
	_, err := m.ensureSeries(ctx, "Show")
	require.EqualError(t, err, "cannot discover series without a TV quality profile")
}

func TestTVNativeEpisodeStatePreservesInt64ID(t *testing.T) {
	ctx := context.Background()
	store := storageMocks.NewMockStorage(gomock.NewController(t))
	const episodeID int64 = 1 << 40
	writeErr := errors.New("transition failed")
	store.EXPECT().UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, nil).Return(writeErr)
	m := MediaManager{seriesStorage: store}
	err := m.updateEpisodeState(ctx, sqlcdb.ListEpisodesRow{
		Episode: sqlcdb.Episode{ID: episodeID},
		State:   string(storage.EpisodeStateMissing),
	}, storage.EpisodeStateDownloading, nil)
	require.ErrorIs(t, err, writeErr)
}

func TestTVNativeEpisodeMatchingPreservesInt64MetadataID(t *testing.T) {
	ctx := context.Background()
	store := storageMocks.NewMockStorage(gomock.NewController(t))
	const metadataID int64 = 1 << 40
	store.EXPECT().GetEpisodeMetadataByID(ctx, metadataID).Return(sqlcdb.EpisodeMetadatum{ID: metadataID, Number: 7}, nil)
	m := MediaManager{seriesMetaStorage: store}
	episode := sqlcdb.ListEpisodesRow{
		Episode: sqlcdb.Episode{ID: metadataID + 1, EpisodeMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}},
		State:   string(storage.EpisodeStateDownloading),
	}
	require.Equal(t, &episode, m.matchEpisodeFileToEpisode(ctx, "Show.S02E07.mkv", []sqlcdb.ListEpisodesRow{episode}))
}

func TestTVNativeEpisodeMatchingSkipsInvalidMetadata(t *testing.T) {
	store := storageMocks.NewMockStorage(gomock.NewController(t))
	m := MediaManager{seriesMetaStorage: store}
	require.Nil(t, m.matchEpisodeFileToEpisode(context.Background(), "Show.S02E07.mkv", []sqlcdb.ListEpisodesRow{{
		Episode: sqlcdb.Episode{ID: 1 << 40, EpisodeMetadataID: sql.NullInt64{Int64: 123}},
	}}))
}

func TestTVNativeEpisodeAlreadyInStateSkipsWrite(t *testing.T) {
	store := storageMocks.NewMockStorage(gomock.NewController(t))
	m := MediaManager{seriesStorage: store}
	require.NoError(t, m.updateEpisodeState(context.Background(), sqlcdb.ListEpisodesRow{
		Episode: sqlcdb.Episode{ID: 1 << 40},
		State:   string(storage.EpisodeStateCompleted),
	}, storage.EpisodeStateCompleted, nil))
}

func TestTVNativeDownloadWithoutClientSkipsRuntimeLookup(t *testing.T) {
	// ON DELETE SET NULL preserves download history without a client. Do not
	// turn its invalid nullable ID into a runtime client lookup for ID zero.
	m := MediaManager{}
	episode := sqlcdb.ListEpisodesRow{
		Episode:    sqlcdb.Episode{ID: 1 << 40},
		State:      string(storage.EpisodeStateDownloading),
		DownloadID: sql.NullString{String: "retained-history", Valid: true},
	}
	require.NoError(t, m.reconcileDownloadingEpisode(context.Background(), episode, nil))
	require.NoError(t, m.reconcileSeasonPackDownload(context.Background(), episode, []sqlcdb.ListEpisodesRow{episode}, nil))
}
