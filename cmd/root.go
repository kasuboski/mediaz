package cmd

import (
	"log"
	"os"
	"strings"

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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig sets viper configurations and default values
func initConfig() {
	log.Print(cfgFile)
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	viper.SetEnvPrefix("MEDIAZ")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", ""))
	viper.AutomaticEnv()

	viper.SetDefault("tmdb.scheme", "https")
	viper.SetDefault("tmdb.host", "api.themoviedb.org")
	viper.SetDefault("tmdb.apiKey", "")

	viper.SetDefault("prowlarr.scheme", "http")
	viper.SetDefault("prowlarr.host", "")
	viper.SetDefault("prowlarr.apiKey", "")

	viper.SetDefault("server.port", 8080)

	viper.SetDefault("library.tv", "")
	viper.SetDefault("library.movie", "")

	viper.SetDefault("storage.filePath", "mediaz.sqlite")
	viper.SetDefault("storage.schemas", []string{"schema.sql"})
}
