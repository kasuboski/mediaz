package cmd

import (
	"net/url"
	"os"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
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
			log.Error("failed to read configurations", err)
		}

		u := url.URL{
			Scheme: cfg.TMDB.Scheme,
			Host:   cfg.TMDB.Host,
		}

		tmdbClient, err := tmdb.NewClient(u.String())
		if err != nil {
			log.Error("failed to create tmdb client", err)
		}

		movieFS := os.DirFS(cfg.Library.MovieDir)
		tvFS := os.DirFS(cfg.Library.TVDir)
		library := library.New(movieFS, tvFS)

		server := server.New(tmdbClient, library, log)
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
