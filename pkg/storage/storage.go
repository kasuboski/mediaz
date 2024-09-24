package storage

import (
	"context"
	"os"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/model"
)

type Storage interface {
	Init(ctx context.Context, schemas ...string) error
	IndexerStorage
	MovieStorage
}

type IndexerStorage interface {
	CreateIndexer(ctx context.Context, indexer model.Indexers) (int64, error)
	DeleteIndexer(ctx context.Context, id int64) error
	ListIndexers(ctx context.Context) ([]*model.Indexers, error)
}

type MovieStorage interface {
	CreateMovie(ctx context.Context, movie model.Movies) (int32, error)
	DeleteMovie(ctx context.Context, id int64) error
	ListMovies(ctx context.Context) ([]*model.Movies, error)

	CreateMovieFile(ctx context.Context, movieFile model.MovieFiles) (int32, error)
	DeleteMovieFile(ctx context.Context, id int64) error
	ListMovieFiles(ctx context.Context) ([]*model.MovieFiles, error)
}

func ReadSchemaFiles(files ...string) ([]string, error) {
	var schemas []string
	for _, f := range files {
		f, err := os.ReadFile(f)
		if err != nil {
			return schemas, err
		}

		schemas = append(schemas, string(f))
	}

	return schemas, nil
}
