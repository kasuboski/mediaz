package cmd

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
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
		r, err := c.GetAPIV1Indexer(ctx, func(ctx context.Context, req *http.Request) error {
			prowlarr.SetRequestAPIKey(cfg.Prowlarr.APIKey, req)
			return nil
		})
		if err != nil {
			log.Fatalf("failed to list indexers: %v", err)
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Fatalf("failed to read response body: %v", err)
		}

		var indexers []*prowlarr.IndexerResource
		err = json.Unmarshal(b, &indexers)
		if err != nil {
			log.Fatalf("failed to unmarshal response: %v", err)
		}

		for _, i := range indexers {
			name, err := i.Name.Get()
			if err != nil {
				continue
			}

			log.Println(name)
		}
	},
}

func init() {
	listCmd.AddCommand(listIndexerCmd)
}
