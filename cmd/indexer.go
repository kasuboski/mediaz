package cmd

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// indexerCmd represents the indexer command
var listIndexerCmd = &cobra.Command{
	Use:   "indexer",
	Short: "list indexers that are currently managed",
	Long:  `list indexers that are currently managed`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatalf("failed to read configurations: %v", err)
		}

		u := url.URL{
			Scheme: cfg.Prowlarr.Scheme,
			Host:   cfg.Prowlarr.Host,
		}

		c, err := prowlarr.NewClient(u.String(), prowlarr.WithRequestEditorFn(prowlarr.SetRequestAPIKey((cfg.Prowlarr.APIKey))))
		if err != nil {
			log.Fatalf("failed to create client: %v", err)
		}

		ctx := context.TODO()
		r, err := c.GetAPIV1Indexer(ctx)
		if err != nil {
			log.Fatalf("failed to list indexers: %v", err)
		}

		resp, err := prowlarr.ParseGetAPIV1IndexerResponse(r)
		if err != nil {
			log.Fatalf("failed to parse indexer response: %v", err)
		}

		if resp.JSON200 == nil {
			log.Fatal("no results in response")
		}

		for _, i := range *resp.JSON200 {
			name, err := i.Name.Get()
			if err != nil {
				continue
			}

			log.Println(name)
		}
	},
}

// searchIdexerCmd represents searching an indexer
var searchIndexerCmd = &cobra.Command{
	Use:        "indexer",
	Short:      "search indexers that are currently managed",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"query"},
	Run: func(cmd *cobra.Command, args []string) {
		log := logger.Get()
		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatalf("failed to read configurations: %v", err)
		}

		u := url.URL{
			Scheme: cfg.Prowlarr.Scheme,
			Host:   cfg.Prowlarr.Host,
		}

		prowlarrClient, err := prowlarr.New(u.String(), cfg.Prowlarr.APIKey)
		if err != nil {
			log.Fatalf("failed to create client: %v", err)
		}

		tmdbURL := url.URL{
			Scheme: cfg.TMDB.Scheme,
			Host:   cfg.TMDB.Host,
		}

		tmdbClient, err := tmdb.New(tmdbURL.String(), cfg.TMDB.APIKey)
		if err != nil {
			log.Fatal("failed to create tmdb client", err)
		}

		movieFS := os.DirFS(cfg.Library.MovieDir)
		tvFS := os.DirFS(cfg.Library.TVDir)
		library := library.New(movieFS, tvFS)

		m := manager.New(tmdbClient, prowlarrClient, library, nil, nil)

		ctx := logger.WithCtx(context.Background(), log)
		idx, err := m.ListIndexers(ctx)
		if err != nil {
			log.Fatal(err)
		}
		indexers := make([]int32, len(idx))
		for i, indexer := range idx {
			indexers[i] = indexer.ID
			log.Debugw("will search", "indexer", indexer.Name)
		}

		query := args[0]
		categories := make([]int32, 0)
		categories = append(categories, manager.TV_CATEGORIES...)
		categories = append(categories, manager.MOVIE_CATEGORIES...)
		releases, err := m.SearchIndexers(ctx, indexers, categories, query)
		if err != nil {
			log.Fatal(err)
		}

		for _, r := range releases {
			name := r.Title.MustGet()
			indexer := r.Indexer
			size := *r.Size
			humanSize := humanize.Bytes(uint64(size))

			log.Infow(fmt.Sprintf("found %s", name), "indexer", indexer, "size", humanSize)
		}

		log.Infof("found %d releases", len(releases))
	},
}

func init() {
	listCmd.AddCommand(listIndexerCmd)
	searchCmd.AddCommand(searchIndexerCmd)
}
