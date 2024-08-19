package cmd

import (
	"net/url"
	"os"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/kasuboski/mediaz/server"

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
			log.Fatal("failed to read configurations", err)
		}

		tmdbURL := url.URL{
			Scheme: cfg.TMDB.Scheme,
			Host:   cfg.TMDB.Host,
		}

		tmdbClient, err := tmdb.NewClient(tmdbURL.String(), tmdb.WithRequestEditorFn(tmdb.SetRequestAPIKey(cfg.TMDB.APIKey)))
		if err != nil {
			log.Fatal("failed to create tmdb client", err)
		}

		prowlarrURL := url.URL{
			Scheme: cfg.Prowlarr.Scheme,
			Host:   cfg.Prowlarr.Host,
		}

		prowlarrClient, err := prowlarr.NewClient(prowlarrURL.String(), prowlarr.WithRequestEditorFn(prowlarr.SetRequestAPIKey(cfg.Prowlarr.APIKey)))
		if err != nil {
			log.Fatal("failed to create prowlarr client", err)
		}

		movieFS := os.DirFS(cfg.Library.MovieDir)
		tvFS := os.DirFS(cfg.Library.TVDir)
		library := library.New(movieFS, tvFS)

		manager := manager.New(tmdbClient, prowlarrClient, library)
		server := server.New(log, manager)
		log.Error(server.Serve(cfg.Server.Port))
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
