package cmd

import (
	"github.com/spf13/cobra"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "get something",
	Long:  `get something`,
}

func init() {
	rootCmd.AddCommand(getCmd)
}
