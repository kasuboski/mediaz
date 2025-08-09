package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mediaz",
	Short: "mediaz cli",
	Long:  `mediaz cli`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

const (
	defaultJobTicker = time.Minute * 10
)

func initConfig() {
	viper.SetConfigFile(cfgFile)

	viper.SetEnvPrefix("MEDIAZ")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", ""))
	viper.AutomaticEnv()

	viper.SetDefault("filePath", cfgFile)

	viper.SetDefault("tmdb.uri", "https://api.themoviedb.org")
	viper.SetDefault("tmdb.apiKey", "")

	viper.SetDefault("prowlarr.uri", "")
	viper.SetDefault("prowlarr.apiKey", "")

	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.distDir", "./frontend/dist")

	viper.SetDefault("library.tv", "")
	viper.SetDefault("library.movie", "")

	viper.SetDefault("storage.filePath", "mediaz.sqlite")
	viper.SetDefault("storage.schemas", []string{"./pkg/storage/sqlite/schema/schema.sql"})
	viper.SetDefault("storage.tableValueSchemas", []string{"./pkg/storage/sqlite/schema/defaults.sql"})

	viper.SetDefault("manager.jobs.movieIndex", defaultJobTicker)
	viper.SetDefault("manager.jobs.movieReconcile", defaultJobTicker)
	viper.SetDefault("manager.jobs.seriesIndex", defaultJobTicker)
	viper.SetDefault("manager.jobs.seriesReconcile", defaultJobTicker)
}
