package cmd

import (
	"github.com/spf13/cobra"
)

// searchCmd represents the tmdb command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "search for something",
	Long:  `search for something`,
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
