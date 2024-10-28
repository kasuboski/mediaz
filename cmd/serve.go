package cmd

import (
	"context"
	"errors"
	"net/url"
	"os"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
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

		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatal("failed to read configurations", zap.Error(err))
		}

		tmdbURL := url.URL{
			Scheme: cfg.TMDB.Scheme,
			Host:   cfg.TMDB.Host,
		}

		tmdbClient, err := tmdb.New(tmdbURL.String(), cfg.TMDB.APIKey)
		if err != nil {
			log.Fatal("failed to create tmdb client", zap.Error(err))
		}

		prowlarrURL := url.URL{
			Scheme: cfg.Prowlarr.Scheme,
			Host:   cfg.Prowlarr.Host,
		}

		prowlarrClient, err := prowlarr.New(prowlarrURL.String(), cfg.Prowlarr.APIKey)
		if err != nil {
			log.Fatal("failed to create prowlarr client", zap.Error(err))
		}

		defaultSchemas := cfg.Storage.Schemas
		if _, err := os.Stat(cfg.Storage.FilePath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Debug("database does not exist, defaulting table values", zap.Any("schemas", cfg.Storage.TableValueSchemas))
				defaultSchemas = append(defaultSchemas, cfg.Storage.TableValueSchemas...)
			}
		}

		store, err := sqlite.New(cfg.Storage.FilePath)
		if err != nil {
			log.Fatal("failed to create storage connection", zap.Error(err))
		}

		schemas, err := storage.ReadSchemaFiles(defaultSchemas...)
		if err != nil {
			log.Fatal("failed to read schema files", zap.Error(err))
		}

		err = store.Init(context.TODO(), schemas...)
		if err != nil {
			log.Fatal("failed to init database", zap.Error(err))
		}

		movieFS := os.DirFS(cfg.Library.MovieDir)
		tvFS := os.DirFS(cfg.Library.TVDir)
		library := library.New(movieFS, tvFS)

		factory := download.NewDownloadClientFactory()
		manager := manager.New(tmdbClient, prowlarrClient, library, store, factory)
		server := server.New(log, manager)
		log.Error(server.Serve(cfg.Server.Port))
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
