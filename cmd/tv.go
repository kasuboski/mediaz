package cmd

import (
	"log"
	"os"

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/spf13/cobra"
)

// listMovieCmd lists movies in a library
var listTVCmd = &cobra.Command{
	Use:        "tv",
	Short:      "List tv episodes found at a path",
	Long:       `List tv episodes found at a path`,
	Args:       cobra.ExactArgs(1),
	ArgAliases: []string{"path to TV library"},
	Run: func(cmd *cobra.Command, args []string) {
		// cfg, err := config.New(viper.GetViper())
		// if err != nil {
		// 	log.Fatalf("failed to read configurations: %v", err)
		// }
		path := args[0]
		tvFS := os.DirFS(path)
		lib := library.New(nil, tvFS)
		episodes, err := lib.FindEpisodes()
		if err != nil {
			log.Fatal(err)
		}

		for _, m := range episodes {
			log.Println(m)
		}
	},
}

func init() {
	listCmd.AddCommand(listTVCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tvCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tvCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
