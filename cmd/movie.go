package cmd

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/client"
	"github.com/kasuboski/mediaz/pkg/library"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	movieQuery string
	path       string
)

// searchMovieCmd represents the movie command
var searchMovieCmd = &cobra.Command{
	Use:   "movie",
	Short: "search for a movie",
	Long:  `search for a movie`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatalf("failed to read configurations: %v", err)
		}

		u := url.URL{
			Scheme: cfg.TMDB.Scheme,
			Host:   cfg.TMDB.Host,
		}

		c, err := client.NewClient(u.String())
		if err != nil {
			log.Fatalf("failed to create tmdb client: %v", err)
		}

		ctx := context.TODO()
		r, err := c.SearchMovie(ctx, &client.SearchMovieParams{
			Query: movieQuery,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+cfg.TMDB.APIKey)
			return nil
		})
		if err != nil {
			log.Fatalf("failed to query movie: %v", err)
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Fatalf("failed to read movie query response body: %v", err)
		}

		log.Println(string(b))
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
		// cfg, err := config.New(viper.GetViper())
		// if err != nil {
		// 	log.Fatalf("failed to read configurations: %v", err)
		// }
		path := args[0]
		movieFS := os.DirFS(path)
		lib := library.New(movieFS, nil)
		movies, err := lib.FindMovies()
		if err != nil {
			log.Fatal(err)
		}

		for _, m := range movies {
			log.Println(m)
		}
	},
}

func init() {
	searchMovieCmd.Flags().StringVarP(&movieQuery, "query", "q", "", "a query for movies")
	_ = searchMovieCmd.MarkFlagRequired("query")

	searchCmd.AddCommand(searchMovieCmd)

	listCmd.AddCommand(listMovieCmd)
}
