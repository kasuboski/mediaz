package cmd

import (
	"context"
	"os"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	mhttp "github.com/kasuboski/mediaz/pkg/http"
	"github.com/kasuboski/mediaz/pkg/indexer"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"

	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// indexCmd represents the index command
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index media libraries for new content",
	Long:  `Index media library directories to discover new movies and series that are not yet monitored`,
}

// indexMoviesCmd indexes movies
var indexMoviesCmd = &cobra.Command{
	Use:   "movies",
	Short: "Index movie library for new content",
	Long:  `Index the movie library directory to discover new movies that are not yet monitored`,
	Run: func(cmd *cobra.Command, args []string) {
		// Setup logger and config
		log := logger.Get()
		ctx := logger.WithCtx(context.Background(), log)

		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatal("failed to read configurations", zap.Error(err))
		}

		// Create TMDB client
		tmdbHttpClient := mhttp.NewRateLimitedClient()
		tmdbClient, err := tmdb.New(cfg.TMDB.URI, cfg.TMDB.APIKey, tmdb.WithHTTPClient(tmdbHttpClient))
		if err != nil {
			log.Fatal("failed to create tmdb client", zap.Error(err))
		}

		indexerFactory := indexer.NewIndexerSourceFactory()

		store, err := sqlite.New(ctx, cfg.Storage.FilePath)
		if err != nil {
			log.Fatal("failed to create storage connection", zap.Error(err))
		}

		schemas, err := storage.GetSchemas()
		if err != nil {
			log.Fatal(err)
		}

		err = store.Init(ctx, schemas...)
		if err != nil {
			log.Fatal("failed to init database", zap.Error(err))
		}

		// Setup library filesystem
		movieFS := os.DirFS(cfg.Library.MovieDir)
		tvFS := os.DirFS(cfg.Library.TVDir)
		mediaFileSystem := &mio.MediaFileSystem{}

		library := library.New(
			library.FileSystem{
				Path: cfg.Library.MovieDir,
				FS:   movieFS,
			},
			library.FileSystem{
				Path: cfg.Library.TVDir,
				FS:   tvFS,
			},
			mediaFileSystem,
		)

		// Create MediaManager
		factory := download.NewDownloadClientFactory(cfg.Library.DownloadMountDir)
		m := manager.New(tmdbClient, indexerFactory, library, store, factory, cfg.Manager, cfg)

		log.Info("Starting movie library indexing")

		err = m.IndexMovieLibrary(ctx)
		if err != nil {
			log.Fatal("failed to index movie library", zap.Error(err))
		}

		log.Info("Movie library indexing completed successfully")
	},
}

// indexSeriesCmd indexes series/TV shows
var indexSeriesCmd = &cobra.Command{
	Use:   "series",
	Short: "Index series library for new content",
	Long:  `Index the series/TV library directory to discover new episodes and series that are not yet monitored`,
	Run: func(cmd *cobra.Command, args []string) {
		// Setup logger and config
		log := logger.Get()
		ctx := logger.WithCtx(context.Background(), log)

		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatal("failed to read configurations", zap.Error(err))
		}

		// Create TMDB client
		tmdbHttpClient := mhttp.NewRateLimitedClient()
		tmdbClient, err := tmdb.New(cfg.TMDB.URI, cfg.TMDB.APIKey, tmdb.WithHTTPClient(tmdbHttpClient))
		if err != nil {
			log.Fatal("failed to create tmdb client", zap.Error(err))
		}

		indexerFactory := indexer.NewIndexerSourceFactory()

		store, err := sqlite.New(ctx, cfg.Storage.FilePath)
		if err != nil {
			log.Fatal("failed to create storage connection", zap.Error(err))
		}

		schemas, err := storage.GetSchemas()
		if err != nil {
			log.Fatal(err)
		}

		err = store.Init(ctx, schemas...)
		if err != nil {
			log.Fatal("failed to init database", zap.Error(err))
		}

		// Setup library filesystem
		movieFS := os.DirFS(cfg.Library.MovieDir)
		tvFS := os.DirFS(cfg.Library.TVDir)
		mediaFileSystem := &mio.MediaFileSystem{}

		library := library.New(
			library.FileSystem{
				Path: cfg.Library.MovieDir,
				FS:   movieFS,
			},
			library.FileSystem{
				Path: cfg.Library.TVDir,
				FS:   tvFS,
			},
			mediaFileSystem,
		)

		// Create MediaManager
		factory := download.NewDownloadClientFactory(cfg.Library.DownloadMountDir)
		m := manager.New(tmdbClient, indexerFactory, library, store, factory, cfg.Manager, cfg)

		// Setup context and call IndexSeriesLibrary

		if indexVerbose {
			log.Info("Starting series library indexing")
		}

		err = m.IndexSeriesLibrary(ctx)
		if err != nil {
			log.Fatal("failed to index series library", zap.Error(err))
		}

		log.Info("Series library indexing completed successfully")
	},
}

var indexVerbose bool

func init() {
	rootCmd.AddCommand(indexCmd)

	indexCmd.AddCommand(indexMoviesCmd)
	indexCmd.AddCommand(indexSeriesCmd)

	// Add verbose flag support for detailed logging
	indexCmd.PersistentFlags().BoolVarP(&indexVerbose, "verbose", "v", false, "Enable verbose logging")
}
