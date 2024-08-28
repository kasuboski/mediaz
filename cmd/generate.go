package cmd

import (
	"github.com/spf13/cobra"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generate something",
	Long:  `generate something`,
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
