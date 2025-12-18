package cmd

import (
	"context"
	"os"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	mhttp "github.com/kasuboski/mediaz/pkg/http"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"

	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// reconcileCmd represents the reconcile command
var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile discovered media with metadata and downloads",
	Long:  `Reconcile discovered media by matching metadata and setting up downloads`,
}

// reconcileMoviesCmd reconciles movies
var reconcileMoviesCmd = &cobra.Command{
	Use:   "movies",
	Short: "Reconcile discovered movies with metadata and downloads",
	Long:  `Reconcile discovered movies by matching TMDB metadata and setting up downloads`,
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

		// Create Prowlarr client
		prowlarrClient, err := prowlarr.New(cfg.Prowlarr.URI, cfg.Prowlarr.APIKey)
		if err != nil {
			log.Fatal("failed to create prowlarr client", zap.Error(err))
		}

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
		m := manager.New(tmdbClient, prowlarrClient, library, store, factory, cfg.Manager, cfg)

		log.Info("Starting movie reconciliation")

		err = m.ReconcileMovies(ctx)
		if err != nil {
			log.Fatal("failed to reconcile movies", zap.Error(err))
		}

		log.Info("Movie reconciliation completed successfully")
	},
}

// reconcileSeriesCmd reconciles series/TV shows
var reconcileSeriesCmd = &cobra.Command{
	Use:   "series",
	Short: "Reconcile discovered series with metadata and downloads",
	Long:  `Reconcile discovered series/TV shows by matching TMDB metadata and setting up downloads`,
	Run: func(cmd *cobra.Command, args []string) {
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

		// Create Prowlarr client
		prowlarrClient, err := prowlarr.New(cfg.Prowlarr.URI, cfg.Prowlarr.APIKey)
		if err != nil {
			log.Fatal("failed to create prowlarr client", zap.Error(err))
		}

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
		m := manager.New(tmdbClient, prowlarrClient, library, store, factory, cfg.Manager, cfg)

		log.Debug("Starting series reconciliation")

		err = m.ReconcileSeries(ctx)
		if err != nil {
			log.Fatal("failed to reconcile series", zap.Error(err))
		}

		log.Info("Series reconciliation completed successfully")
	},
}

func init() {
	rootCmd.AddCommand(reconcileCmd)
	reconcileCmd.AddCommand(reconcileMoviesCmd)
	reconcileCmd.AddCommand(reconcileSeriesCmd)
}
