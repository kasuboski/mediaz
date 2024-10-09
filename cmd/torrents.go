package cmd

import (
	"context"
	"log"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/download/torrent/transmission"
	"github.com/spf13/cobra"
)

// listTorrentsCmd represents the list torrents command
var listTorrentsCmd = &cobra.Command{
	Use:   "torrents",
	Short: "list torrents",
	Long:  `list torrents`,
	Run: func(cmd *cobra.Command, args []string) {
		tranmissionClient := transmission.NewClient(http.DefaultClient, "https", "transmission.int.kyledev.co", 9091)

		torrents, err := tranmissionClient.List(context.Background())
		if err != nil {
			log.Fatal(err)
		}

		for _, t := range torrents {
			log.Printf("%+v", t)
		}
	},
}

func init() {
	listCmd.AddCommand(listTorrentsCmd)
}
