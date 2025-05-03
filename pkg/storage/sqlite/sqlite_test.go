package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	store := initSqlite(t, context.Background())
	assert.NotNil(t, store)
}

func ptr[A any](thing A) *A {
	return &thing
}

func initSqlite(t *testing.T, ctx context.Context) storage.Storage {
	store, err := New(":memory:")
	assert.Nil(t, err)

	schemas, err := storage.ReadSchemaFiles("./schema/schema.sql")
	assert.Nil(t, err)

	err = store.Init(ctx, schemas...)
	assert.Nil(t, err)
	return store
}

func TestIndexerStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	ix, err := store.ListIndexers(ctx)
	assert.Nil(t, err)
	assert.Empty(t, ix)

	apikey := "supersecret"
	create := model.Indexer{
		ID:       1,
		Name:     "Index",
		Priority: 20,
		URI:      "http://here",
		APIKey:   &apikey,
	}
	res, err := store.CreateIndexer(ctx, create)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	ix, err = store.ListIndexers(ctx)
	assert.Nil(t, err)
	assert.Len(t, ix, 1)
	actual := ix[0]
	assert.Equal(t, &create, actual)

	err = store.DeleteIndexer(ctx, int64(actual.ID))
	assert.Nil(t, err)

	ix, err = store.ListIndexers(ctx)
	assert.Nil(t, err)
	assert.Empty(t, ix)
}

func TestMovieStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	path := "Title"
	movie := storage.Movie{
		Movie: model.Movie{
			ID:              1,
			Path:            &path,
			Monitored:       1,
			MovieFileID:     ptr(int32(1)),
			MovieMetadataID: ptr(int32(1)),
		},
	}

	wantMovie := storage.Movie{
		Movie: model.Movie{
			ID:              1,
			Path:            &path,
			Monitored:       1,
			MovieFileID:     ptr(int32(1)),
			MovieMetadataID: ptr(int32(1)),
		},
		State: storage.MovieStateMissing,
	}

	res, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	id := int64(res)
	movies, err := store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	actual := movies[0]
	actual.Added = nil
	assert.Equal(t, &wantMovie, actual)

	movies, err = store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	actual = movies[0]
	actual.Added = nil
	assert.Equal(t, &wantMovie, actual)

	err = store.UpdateMovieState(ctx, int64(movies[0].ID), storage.MovieStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       ptr("123"),
		DownloadClientID: ptr(int32(1)),
	})
	assert.Nil(t, err)

	movies, err = store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	actual = movies[0]
	wantMovie.State = storage.MovieStateDownloading
	wantMovie.DownloadClientID = 1
	wantMovie.DownloadID = "123"
	actual.Added = nil
	assert.Equal(t, &wantMovie, actual)

	movies, err = store.ListMoviesByState(ctx, storage.MovieStateDownloading)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	movies[0].Added = nil
	assert.Equal(t, &wantMovie, movies[0])

	mov, err := store.GetMovieByMetadataID(ctx, 1)
	assert.NoError(t, err)
	assert.NotNil(t, mov)

	err = store.DeleteMovie(ctx, id)
	assert.Nil(t, err)

	movies, err = store.ListMovies(ctx)
	assert.Empty(t, movies)
	assert.Nil(t, err)

	file := model.MovieFile{
		ID:           1,
		Quality:      "HDTV-720p",
		Size:         1_000_000_000,
		RelativePath: ptr("Title/Title.mkv"),
	}
	res, err = store.CreateMovieFile(ctx, file)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	movie.ID = 2
	mRes, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
	assert.Nil(t, err)
	assert.NotEmpty(t, mRes)

	files, err := store.GetMovieFilesByMovieName(ctx, "Title")
	require.NoError(t, err)
	actualFile := files[0]
	actualFile.DateAdded = time.Time{}
	// clear non-deterministic date field
	assert.Equal(t, &file, actualFile)

	id = int64(res)
	files, err = store.ListMovieFiles(ctx)
	assert.Nil(t, err)
	assert.Len(t, files, 1)
	actualFile = files[0]
	assert.NotEmpty(t, actualFile.DateAdded)
	// clear non-deterministic date field
	actualFile.DateAdded = time.Time{}
	assert.Equal(t, &file, actualFile)

	err = store.DeleteMovieFile(ctx, id)
	assert.Nil(t, err)

	files, err = store.ListMovieFiles(ctx)
	assert.Nil(t, err)
	assert.Empty(t, files)
}

func TestMovieMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	movieMeta := model.MovieMetadata{
		ID:      1,
		TmdbID:  1234,
		Title:   "My Cool Movie",
		Runtime: 1000,
	}
	id, err := store.CreateMovieMetadata(ctx, movieMeta)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)

	metadata, err := store.ListMovieMetadata(ctx)
	assert.NoError(t, err)
	assert.Len(t, metadata, 1)
	actual := metadata[0]
	assert.Equal(t, &movieMeta, actual)

	one, err := store.GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(1234)))
	assert.NoError(t, err)
	assert.NotNil(t, one)

	notFound, err := store.GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(124)))
	assert.ErrorIs(t, err, storage.ErrNotFound)
	assert.Nil(t, notFound)

	err = store.DeleteMovieMetadata(ctx, id)
	assert.NoError(t, err)

	metadata, err = store.ListMovieMetadata(ctx)
	assert.NoError(t, err)
	assert.Empty(t, metadata)
}

func TestGetQualityStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	testProfile := model.QualityProfile{
		Name:            "test profile",
		UpgradeAllowed:  true,
		CutoffQualityID: 3,
	}

	id, err := store.CreateQualityProfile(ctx, testProfile)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), id)

	firstDefinition := model.QualityDefinition{
		QualityID:     1,
		Name:          "test quality definition 1",
		PreferredSize: 1999,
		MinSize:       15,
		MaxSize:       2000,
		MediaType:     "movie",
	}
	id, err = store.CreateQualityDefinition(ctx, firstDefinition)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), id)

	definitionOne, err := store.GetQualityDefinition(ctx, 1)
	assert.Nil(t, err)
	firstDefinition.ID = int32(id)
	assert.Equal(t, firstDefinition, definitionOne)

	secondDefinition := model.QualityDefinition{
		QualityID:     2,
		Name:          "test quality definition 2",
		PreferredSize: 1499,
		MinSize:       10,
		MaxSize:       1500,
		MediaType:     "movie",
	}
	id, err = store.CreateQualityDefinition(ctx, secondDefinition)
	assert.Nil(t, err)
	assert.Equal(t, int64(2), id)

	definitionTwo, err := store.GetQualityDefinition(ctx, 2)
	assert.Nil(t, err)
	secondDefinition.ID = int32(id)
	assert.Equal(t, secondDefinition, definitionTwo)

	definitions, err := store.ListQualityDefinitions(ctx)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []*model.QualityDefinition{
		&definitionOne, &definitionTwo,
	}, definitions)

	firstQualityItem := model.QualityProfileItem{
		ProfileID: 1,
		QualityID: 1,
	}
	id, err = store.CreateQualityProfileItem(ctx, firstQualityItem)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), id)

	firstItem, err := store.GetQualityProfileItem(ctx, 1)
	assert.Nil(t, err)
	i32ID := int32(id)
	firstQualityItem.ID = &i32ID
	assert.Equal(t, firstQualityItem, firstItem)

	secondQualityItem := model.QualityProfileItem{
		ProfileID: 1,
		QualityID: 2,
	}
	id, err = store.CreateQualityProfileItem(ctx, secondQualityItem)
	assert.Nil(t, err)
	assert.Equal(t, int64(2), id)

	secondItem, err := store.GetQualityProfileItem(ctx, 2)
	assert.Nil(t, err)
	i32ID = int32(id)
	secondQualityItem.ID = &i32ID
	assert.Equal(t, secondQualityItem, secondItem)

	items, err := store.ListQualityProfileItems(ctx)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []*model.QualityProfileItem{
		&firstItem, &secondItem,
	}, items)

	profile, err := store.GetQualityProfile(ctx, 1)
	assert.Nil(t, err)
	assert.Equal(t, "test profile", profile.Name)
	assert.Equal(t, true, profile.UpgradeAllowed)
	assert.Equal(t, int32(3), profile.CutoffQualityID)
	assert.ElementsMatch(t, []storage.QualityDefinition{
		{
			QualityID:     1,
			Name:          "test quality definition 1",
			PreferredSize: 1999,
			MinSize:       15,
			MaxSize:       2000,
			MediaType:     "movie",
		},
		{
			QualityID:     2,
			Name:          "test quality definition 2",
			PreferredSize: 1499,
			MinSize:       10,
			MaxSize:       1500,
			MediaType:     "movie",
		},
	}, profile.Qualities)

	profiles, err := store.ListQualityProfiles(ctx)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []*storage.QualityProfile{
		{
			ID:              1,
			Name:            "test profile",
			UpgradeAllowed:  true,
			CutoffQualityID: 3,
			Qualities: []storage.QualityDefinition{
				{
					QualityID:     1,
					Name:          "test quality definition 1",
					PreferredSize: 1999,
					MinSize:       15,
					MaxSize:       2000,
					MediaType:     "movie",
				},
				{
					QualityID:     2,
					Name:          "test quality definition 2",
					PreferredSize: 1499,
					MinSize:       10,
					MaxSize:       1500,
					MediaType:     "movie",
				},
			},
		},
	}, profiles)

	err = store.DeleteQualityDefinition(ctx, 1)
	assert.Nil(t, err)

	err = store.DeleteQualityDefinition(ctx, 2)
	assert.Nil(t, err)

	err = store.DeleteQualityProfileItem(ctx, 1)
	assert.Nil(t, err)

	err = store.DeleteQualityProfileItem(ctx, 2)
	assert.Nil(t, err)

	err = store.DeleteQualityProfile(ctx, 1)
	assert.Nil(t, err)
}

func TestDownloadClientStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	clientOne := model.DownloadClient{
		Type:           "torrent",
		Implementation: "transmission",
		Host:           "transmission",
		Scheme:         "http",
		Port:           9091,
	}

	id, err := store.CreateDownloadClient(ctx, clientOne)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), id)

	storedClient, err := store.GetDownloadClient(ctx, id)
	assert.Nil(t, err)
	assert.Equal(t, clientOne.Type, storedClient.Type)
	assert.Equal(t, clientOne.Implementation, storedClient.Implementation)
	assert.Equal(t, clientOne.Host, storedClient.Host)
	assert.Equal(t, clientOne.Scheme, storedClient.Scheme)
	assert.Equal(t, clientOne.Port, storedClient.Port)

	clientTwo := model.DownloadClient{
		Type:           "usenet",
		Implementation: "something",
		Host:           "host",
		Scheme:         "http",
		Port:           8080,
	}

	id, err = store.CreateDownloadClient(ctx, clientTwo)
	assert.Nil(t, err)
	assert.Equal(t, int64(2), id)

	storedClient, err = store.GetDownloadClient(ctx, id)
	assert.Nil(t, err)
	assert.Equal(t, clientTwo.Type, storedClient.Type)
	assert.Equal(t, clientTwo.Implementation, storedClient.Implementation)
	assert.Equal(t, clientTwo.Host, storedClient.Host)
	assert.Equal(t, clientTwo.Scheme, storedClient.Scheme)
	assert.Equal(t, clientTwo.Port, storedClient.Port)

	clients, err := store.ListDownloadClients(ctx)
	assert.Nil(t, err)
	expectedClients := []*model.DownloadClient{
		{
			ID:             1,
			Type:           "torrent",
			Implementation: "transmission",
			Host:           "transmission",
			Scheme:         "http",
			Port:           9091,
		},
		{
			ID:             2,
			Type:           "usenet",
			Implementation: "something",
			Host:           "host",
			Scheme:         "http",
			Port:           8080,
		},
	}

	assert.ElementsMatch(t, expectedClients, clients)

	err = store.DeleteDownloadClient(ctx, 1)
	assert.Nil(t, err)

	clients, err = store.ListDownloadClients(ctx)
	assert.Nil(t, err)
	expectedClients = []*model.DownloadClient{
		{
			ID:             2,
			Type:           "usenet",
			Implementation: "something",
			Host:           "host",
			Scheme:         "http",
			Port:           8080,
		},
	}
	assert.ElementsMatch(t, expectedClients, clients)

	err = store.DeleteDownloadClient(ctx, 2)
	assert.Nil(t, err)
}
func TestSQLite_UpdateMovieMovieFileID(t *testing.T) {
	t.Run("update movie file id", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		path := "Title/Title.mkv"
		newMovie := storage.Movie{
			Movie: model.Movie{
				ID:          1,
				Monitored:   1,
				Path:        &path,
				MovieFileID: ptr(int32(1)),
			},
		}

		movieID, err := store.CreateMovie(ctx, newMovie, storage.MovieStateDiscovered)
		require.NoError(t, err)
		require.Equal(t, int64(1), movieID)

		err = store.UpdateMovieMovieFileID(ctx, movieID, 2)
		require.NoError(t, err)

		movie, err := store.GetMovie(ctx, movieID)
		require.NoError(t, err)

		assert.Equal(t, int32(1), movie.ID)
		assert.Equal(t, int32(2), *movie.MovieFileID)
	})
}

func TestSQLite_GetMovieByMovieFileID(t *testing.T) {
	t.Run("get movie by movie file id", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		path := "Title/Title.mkv"
		movie1 := storage.Movie{
			Movie: model.Movie{
				Monitored:   1,
				Path:        &path,
				MovieFileID: ptr(int32(1)),
			},
		}

		_, err := store.CreateMovie(ctx, movie1, storage.MovieStateDiscovered)
		require.NoError(t, err)

		movie2 := storage.Movie{
			Movie: model.Movie{
				Monitored:   1,
				Path:        &path,
				MovieFileID: ptr(int32(2)),
			},
		}
		_, err = store.CreateMovie(ctx, movie2, storage.MovieStateDiscovered)
		require.NoError(t, err)

		movie, err := store.GetMovieByMovieFileID(ctx, 1)
		require.NoError(t, err)

		assert.Equal(t, int32(1), movie.ID)
		assert.Equal(t, int32(1), int32(*movie.MovieFileID))
	})
}
