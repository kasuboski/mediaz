package cmd

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/kasuboski/mediaz/server"
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

		c, err := prowlarr.NewClient(u.String())
		if err != nil {
			log.Fatalf("failed to create client: %v", err)
		}

		ctx := context.TODO()
		r, err := c.GetAPIV1Indexer(ctx, prowlarr.SetRequestAPIKey(cfg.Prowlarr.APIKey))
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
	Use:   "indexer",
	Short: "search indexers that are currently managed",
	Long:  `search indexers that are currently managed`,
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

		prowlarrClient, err := prowlarr.NewClient(u.String())
		if err != nil {
			log.Fatalf("failed to create client: %v", err)
		}

		tmdbURL := url.URL{
			Scheme: cfg.TMDB.Scheme,
			Host:   cfg.TMDB.Host,
		}

		tmdbClient, err := tmdb.NewClient(tmdbURL.String())
		if err != nil {
			log.Fatal("failed to create tmdb client", err)
		}

		movieFS := os.DirFS(cfg.Library.MovieDir)
		tvFS := os.DirFS(cfg.Library.TVDir)
		library := library.New(movieFS, tvFS)

		manager := server.NewManager(tmdbClient, prowlarrClient, library, cfg)

		ctx := logger.WithCtx(context.Background(), log)
		r, err := prowlarrClient.GetAPIV1Indexer(ctx, prowlarr.SetRequestAPIKey(cfg.Prowlarr.APIKey))
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

		indexers := make([]int32, len(*resp.JSON200))
		for i, indexer := range *resp.JSON200 {
			indexers[i] = *indexer.ID
			log.Debugw("will search", "indexer", indexer.Name.MustGet())
		}

		releases, err := manager.SearchIndexers(ctx, indexers, []int32{2000}, "Bourne Identity")
		if err != nil {
			log.Fatal(err)
		}

		for _, r := range releases {
			name := r.Title.MustGet()
			indexer := r.Indexer
			size := r.Size
			tmdb := r.TmdbID

			log.Infow(fmt.Sprintf("found %s", name), "indexer", indexer, "size", size, "tmdb", tmdb)
		}

		log.Infof("found %d releases", len(releases))
	},
}

func init() {
	listCmd.AddCommand(listIndexerCmd)
	searchCmd.AddCommand(searchIndexerCmd)
}
