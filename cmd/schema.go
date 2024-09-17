package cmd

import (
	"context"
	"log"
	"os"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/spf13/cobra"

	jet "github.com/go-jet/jet/v2/generator/sqlite"
)

var (
	outputDirectory string
	schemaFiles     []string
)

// schemaCmd represents the schema command
var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "generate database code",
	Long:  `generate database code`,
	Run: func(cmd *cobra.Command, args []string) {
		schemas, err := readSchemaFiles(schemaFiles...)
		if err != nil {
			log.Fatal(err)
		}

		tmpStorage, err := sqlite.New("tmp.sqlite")
		if err != nil {
			log.Fatal(err)
		}
		defer os.Remove("tmp.sqlite")

		err = tmpStorage.Init(context.Background(), schemas...)
		if err != nil {
			log.Fatal(err)
		}

		err = jet.GenerateDSN("tmp.sqlite", outputDirectory)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("successfully generated to %s", outputDirectory)
	},
}

func init() {
	generateCmd.AddCommand(schemaCmd)
	schemaCmd.Flags().StringSliceVarP(&schemaFiles, "schemas", "s", []string{"./pkg/storage/sqlite/schema/schema.sql"}, "list of schema files to generate code from")
	schemaCmd.Flags().StringVarP(&outputDirectory, "out", "o", "./pkg/storage/sqlite/schema/gen", "directory to output generated code to")
}
