package cmd

import (
	"context"
	"log"
	"net/url"
	"os"

	"github.com/kasuboski/mediaz/config"
	mhttp "github.com/kasuboski/mediaz/pkg/http"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/tmdb"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	movieQuery string
)

// discoverMovieCmd represents the movie command
var discoverMovieCmd = &cobra.Command{
	Use:   "movie",
	Short: "discover a movie",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatalf("failed to read configurations: %v", err)
		}

		u := url.URL{
			Scheme: cfg.TMDB.Scheme,
			Host:   cfg.TMDB.Host,
		}

		c, err := tmdb.NewClient(u.String(), tmdb.WithHTTPClient(mhttp.NewRateLimitedClient()))
		if err != nil {
			log.Fatalf("failed to create tmdb client: %v", err)
		}

		ctx := context.TODO()
		r, err := c.SearchMovie(ctx, &tmdb.SearchMovieParams{Query: movieQuery}, tmdb.SetRequestAPIKey(cfg.TMDB.APIKey))
		if err != nil {
			log.Fatalf("failed to query movie: %v", err)
		}

		resp, err := tmdb.ParseSearchMovieResponse(r)
		if err != nil {
			log.Fatalf("failed to parse movie response: %v", err)
		}

		if resp.JSON200 == nil || resp.JSON200.Results == nil {
			log.Fatal("no results found")
		}

		for _, r := range *resp.JSON200.Results {
			log.Println(*r.Title)
		}
	},
}

// listMovieCmd lists movies in a library
var listMovieCmd = &cobra.Command{
	Use:        "movie",
	Short:      "List movies found at a path",
	Long:       `List movies found at a path`,
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"path to movies"},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log := logger.Get()
		ctx = logger.WithCtx(ctx, log)

		path := args[0]
		movieFS := os.DirFS(path)
		lib := library.New(
			library.FileSystem{FS: movieFS},
			library.FileSystem{},
			&mio.MediaFileSystem{},
		)
		movies, err := lib.FindMovies(ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, m := range movies {
			log.Info(m)
		}
	},
}

func init() {
	discoverMovieCmd.Flags().StringVarP(&movieQuery, "query", "q", "", "a query for movies")
	_ = discoverMovieCmd.MarkFlagRequired("query")

	discoverCmd.AddCommand(discoverMovieCmd)

	listCmd.AddCommand(listMovieCmd)
}
