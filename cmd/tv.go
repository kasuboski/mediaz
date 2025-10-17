package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/kasuboski/mediaz/config"
	mhttp "github.com/kasuboski/mediaz/pkg/http"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listTVCmd lists tv episodes found in library
var listTVCmd = &cobra.Command{
	Use:        "tv",
	Short:      "List tv episodes found at a path",
	Long:       `List tv episodes found at a path`,
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"path to TV library"},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log := logger.Get()
		ctx = logger.WithCtx(ctx, log)

		path := args[0]
		tvFS := os.DirFS(path)
		lib := library.New(library.FileSystem{}, library.FileSystem{
			Path: path,
			FS:   tvFS,
		},
			&mio.MediaFileSystem{},
		)
		episodes, err := lib.FindEpisodes(ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, m := range episodes {
			log.Info(m)
		}
	},
}

var (
	tmdbID int
)

// seriesDetailsCmd searches tmdb for a series and parses the response for all metadata
var seriesDetailsCmd = &cobra.Command{
	Use:   "series",
	Short: "get series details",
	Run: func(cmd *cobra.Command, args []string) {
		log := logger.Get()

		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatal("failed to read configurations", zap.Error(err))
		}

		tmdbHttpClient := mhttp.NewRateLimitedClient()
		tmdbClient, err := tmdb.New(cfg.TMDB.URI, cfg.TMDB.APIKey, tmdb.WithHTTPClient(tmdbHttpClient))
		if err != nil {
			log.Fatal("failed to create tmdb client", zap.Error(err))
		}

		m := manager.New(tmdbClient, nil, &library.MediaLibrary{}, nil, nil, cfg.Manager, cfg)

		ctx := logger.WithCtx(context.Background(), log)
		details, err := m.GetSeriesDetails(ctx, tmdbID)
		if err != nil {
			log.Fatal("failed to get series details", zap.Error(err))
		}

		b, err := json.Marshal(details)
		if err != nil {
			log.Fatal("failed to marshal series details", zap.Error(err))
		}

		log.Info(string(b))
	},
}

func init() {
	listCmd.AddCommand(listTVCmd)

	seriesDetailsCmd.Flags().IntVarP(&tmdbID, "id", "i", 0, "tmdb id")
	seriesDetailsCmd.MarkFlagRequired("id")
	getCmd.AddCommand(seriesDetailsCmd)
}
