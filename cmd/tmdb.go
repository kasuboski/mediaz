package cmd

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/client"
	"github.com/spf13/cobra"
)

// tmdbCmd represents the tmdb command
var tmdbCmd = &cobra.Command{
	Use:   "tmdb",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		http := http.DefaultClient
		c, err := client.NewClient("https://api.themoviedb.org/", client.WithHTTPClient(http))
		if err != nil {
			log.Fatalf("failed to create tmdb client: %v", err)
		}
		
		ctx := context.TODO()
		r, err := c.SearchMovie(ctx, &client.SearchMovieParams{
			Query: "300",
		})
		if err != nil {
			log.Fatalf("failed to query movie: %v", err)
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Fatalf("failed to read movie query response body: %v", err)
		}

		log.Printf("%s", b)

		var searchMovieResponse client.SearchMovieResponse
		err = json.Unmarshal(b, &searchMovieResponse)
		if err != nil {
			log.Fatalf("failed to unmarshal response: %v", err)
		}

		log.Printf("got response: %+v", searchMovieResponse)
	},
}

func init() {
	rootCmd.AddCommand(tmdbCmd)
}
