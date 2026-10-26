package manager

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	libraryMocks "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMovieDiscoveryPersistsWithMovieProfile(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, ctx)
	profiles, err := store.ListQualityProfilesByMediaType(ctx, "movie")
	require.NoError(t, err)
	require.NotEmpty(t, profiles)
	ctrl := gomock.NewController(t)
	mediaLibrary := libraryMocks.NewMockLibrary(ctrl)
	mediaLibrary.EXPECT().FindMovies(ctx).Return([]library.MovieFile{{RelativePath: "Discovered/movie.mkv", Size: 1024}}, nil).Times(2)
	manager := New(nil, nil, mediaLibrary, store, nil, config.Manager{}, config.Config{})
	require.NoError(t, manager.IndexMovieLibrary(ctx))
	movies, err := store.ListMovies(ctx)
	require.NoError(t, err)
	require.Len(t, movies, 1)
	assert.Equal(t, []sqlcdb.ListMoviesRow{{Movie: sqlcdb.Movie{
		ID: 1, Path: sql.NullString{String: "Discovered", Valid: true}, QualityProfileID: profiles[0].ID,
		MovieFileID: sql.NullInt64{Int64: 1, Valid: true}, Added: movies[0].Movie.Added,
	}, State: string(storage.MovieStateDiscovered)}}, movies)
	// Reindexing retains the same persisted movie and its valid profile.
	require.NoError(t, manager.IndexMovieLibrary(ctx))
	again, err := store.ListMovies(ctx)
	require.NoError(t, err)
	assert.Equal(t, movies, again)
}

func TestMovieDiscoveryMissingMovieProfileIsActionable(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, ctx)
	profiles, err := store.ListQualityProfilesByMediaType(ctx, "movie")
	require.NoError(t, err)
	for _, profile := range profiles {
		require.NoError(t, store.DeleteQualityProfile(ctx, profile.ID))
	}
	// Episode profiles remain, but must not be used for movie discovery.
	episodeProfiles, err := store.ListQualityProfilesByMediaType(ctx, "episode")
	require.NoError(t, err)
	require.NotEmpty(t, episodeProfiles)
	ctrl := gomock.NewController(t)
	mediaLibrary := libraryMocks.NewMockLibrary(ctrl)
	mediaLibrary.EXPECT().FindMovies(ctx).Return([]library.MovieFile{{RelativePath: "Discovered/movie.mkv"}}, nil)
	manager := New(nil, nil, mediaLibrary, store, nil, config.Manager{}, config.Config{})
	require.ErrorContains(t, manager.IndexMovieLibrary(ctx), "create a movie quality profile")
	movies, err := store.ListMovies(ctx)
	require.NoError(t, err)
	assert.Empty(t, movies)
}

func TestMovieDiscoveryPropagatesPersistenceErrors(t *testing.T) {
	for _, stage := range []string{"file", "lookup", "profile", "movie"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			store := storageMocks.NewMockStorage(ctrl)
			mediaLibrary := libraryMocks.NewMockLibrary(ctrl)
			mediaLibrary.EXPECT().FindMovies(ctx).Return([]library.MovieFile{{RelativePath: "Discovered/movie.mkv"}}, nil)
			failure := errors.New("persistence unavailable")
			store.EXPECT().ListMovieFiles(ctx).Return([]sqlcdb.MovieFile{}, nil)
			file := sqlcdb.MovieFile{ID: 1, RelativePath: sql.NullString{String: "Discovered/movie.mkv", Valid: true}}
			if stage == "file" {
				store.EXPECT().CreateMovieFile(ctx, gomock.Any()).Return(int64(0), failure)
			} else {
				store.EXPECT().CreateMovieFile(ctx, gomock.Any()).Return(int64(1), nil)
				store.EXPECT().ListMovieFiles(ctx).Return([]sqlcdb.MovieFile{file}, nil)
				if stage == "lookup" {
					store.EXPECT().GetMovieByPath(ctx, "Discovered").Return(sqlcdb.GetMovieByPathRow{}, failure)
				} else {
					store.EXPECT().GetMovieByPath(ctx, "Discovered").Return(sqlcdb.GetMovieByPathRow{}, storage.ErrNotFound)
					if stage == "profile" {
						store.EXPECT().ListQualityProfilesByMediaType(ctx, "movie").Return(nil, failure)
					} else {
						store.EXPECT().ListQualityProfilesByMediaType(ctx, "movie").Return([]sqlcdb.QualityProfile{{ID: 3}}, nil)
						store.EXPECT().CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Discovered", Valid: true}, MovieFileID: sql.NullInt64{Int64: 1, Valid: true}, QualityProfileID: 3}, storage.MovieStateDiscovered).Return(int64(0), failure)
					}
				}
			}
			manager := New(nil, nil, mediaLibrary, store, nil, config.Manager{}, config.Config{})
			require.ErrorIs(t, manager.IndexMovieLibrary(ctx), failure)
		})
	}
}
