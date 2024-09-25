package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	store := initSqlite(t, context.Background())
	assert.NotNil(t, store)
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

	movie := model.Movie{
		ID:              1,
		Path:            "Title/Title.mkv",
		Monitored:       1,
		MovieFileID:     1,
		MovieMetadataID: 1,
	}
	res, err := store.CreateMovie(ctx, movie)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	id := int64(res)
	movies, err := store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	actual := movies[0]
	assert.Equal(t, &movie, actual)

	err = store.DeleteMovie(ctx, id)
	assert.Nil(t, err)

	movies, err = store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Empty(t, movies)

	file := model.MovieFile{
		ID:      1,
		Quality: "HDTV-720p",
		Size:    1_000_000_000,
		MovieID: 1,
	}
	res, err = store.CreateMovieFile(ctx, file)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	id = int64(res)
	files, err := store.ListMovieFiles(ctx)
	assert.Nil(t, err)
	assert.Len(t, files, 1)
	actualFile := files[0]
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
