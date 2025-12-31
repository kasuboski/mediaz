package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/kasuboski/mediaz/config"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"

	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listIndexerCmd represents the indexer command
var listIndexerCmd = &cobra.Command{
	Use:   "indexer",
	Short: "list indexers that are currently managed",
	Long:  `list indexers that are currently managed`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatalf("failed to read configurations: %v", err)
		}

		c, err := prowlarr.NewClient(cfg.Prowlarr.URI, prowlarr.WithRequestEditorFn(prowlarr.SetRequestAPIKey((cfg.Prowlarr.APIKey))))
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

// searchIndexerCmd represents searching an indexer
var searchIndexerCmd = &cobra.Command{
	Use:        "indexer",
	Short:      "search indexers that are currently managed",
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"query"},
	Run: func(cmd *cobra.Command, args []string) {
		log := logger.Get()
		ctx := logger.WithCtx(context.Background(), log)

		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatalf("failed to read configurations: %v", err)
		}

		prowlarrClient, err := prowlarr.New(cfg.Prowlarr.URI, cfg.Prowlarr.APIKey)
		if err != nil {
			log.Fatalf("failed to create client: %v", err)
		}

		tmdbClient, err := tmdb.New(cfg.TMDB.URI, cfg.TMDB.APIKey)
		if err != nil {
			log.Fatal("failed to create tmdb client", err)
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

		store, err := sqlite.New(ctx, cfg.Storage.FilePath)
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

		m := manager.New(tmdbClient, prowlarrClient, library, store, nil, cfg.Manager, cfg)

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
		if m, err := cmd.Flags().GetBool("movie"); err == nil && m {
			categories = append(categories, manager.MOVIE_CATEGORIES...)
		}
		if s, err := cmd.Flags().GetBool("show"); err == nil && s {
			categories = append(categories, manager.TV_CATEGORIES...)
		}

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

			// clear some url fields
			r.GUID = nullable.NewNullNullable[string]()
			r.Indexer = nullable.NewNullNullable[string]()
			r.CommentURL = nullable.NewNullNullable[string]()
			r.DownloadURL = nullable.NewNullNullable[string]()
			r.InfoURL = nullable.NewNullNullable[string]()
			r.PosterURL = nullable.NewNullNullable[string]()
			r.MagnetURL = nullable.NewNullNullable[string]()
		}

		if out, err := cmd.Flags().GetString("output"); err == nil {
			data, err := json.MarshalIndent(releases, "", "  ")
			if err != nil {
				log.Fatal(err)
			}
			f, err := os.Create(out)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintln(f, string(data))
		}

		log.Infof("found %d releases", len(releases))
	},
}

func init() {
	listCmd.AddCommand(listIndexerCmd)
	searchIndexerCmd.Flags().Bool("movie", true, "search for arg as a movie")
	searchIndexerCmd.Flags().Bool("show", true, "search for arg as a tv show")
	searchIndexerCmd.Flags().StringP("output", "o", "", "path to output the found releases as json")
	searchCmd.AddCommand(searchIndexerCmd)
}
