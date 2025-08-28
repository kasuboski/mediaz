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
	"github.com/kasuboski/mediaz/server"

	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start the media server",
	Long:  `start the media server`,
	Run: func(cmd *cobra.Command, args []string) {
		log := logger.Get()
		ctx := context.Background()

		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatal("failed to read configurations", zap.Error(err))
		}

		tmdbHttpClient := mhttp.NewRateLimitedClient()
		tmdbClient, err := tmdb.New(cfg.TMDB.URI, cfg.TMDB.APIKey, tmdb.WithHTTPClient(tmdbHttpClient))
		if err != nil {
			log.Fatal("failed to create tmdb client", zap.Error(err))
		}

		prowlarrClient, err := prowlarr.New(cfg.Prowlarr.URI, cfg.Prowlarr.APIKey)
		if err != nil {
			log.Fatal("failed to create client", zap.Error(err))
		}

		store, err := sqlite.New(cfg.Storage.FilePath)
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

		movieFS := os.DirFS(cfg.Library.MovieDir)
		tvFS := os.DirFS(cfg.Library.TVDir)
		library := library.New(
			library.FileSystem{
				Path: cfg.Library.MovieDir,
				FS:   movieFS,
			},
			library.FileSystem{
				Path: cfg.Library.TVDir,
				FS:   tvFS,
			},
			&mio.MediaFileSystem{},
		)

		factory := download.NewDownloadClientFactory(cfg.Library.DownloadMountDir)
		manager := manager.New(tmdbClient, prowlarrClient, library, store, factory, cfg.Manager)

		go func() {
			log.Fatal(manager.Run(ctx))
		}()

		server := server.New(log, manager, cfg.Server)
		log.Error(server.Serve(cfg.Server.Port))
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
