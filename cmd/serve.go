/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/kasuboski/mediaz/config"
	tmdb "github.com/kasuboski/mediaz/pkg/client"
	"github.com/kasuboski/mediaz/server"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.New(viper.GetViper())
		if err != nil {
			log.Fatalf("failed to read configurations: %v", err)
		}

		u := url.URL{
			Scheme: cfg.TMDB.Scheme,
			Host:   cfg.TMDB.Host,
		}

		tmdbClient, err := tmdb.NewClient(u.String())
		if err != nil {
			log.Fatalf("failed to create tmdb client: %v", err)
		}

		router := server.New(tmdbClient)
		// Add your routes as needed

		srv := &http.Server{
			Addr: "0.0.0.0:8080",
			// Good practice to set timeouts to avoid Slowloris attacks.
			WriteTimeout: time.Second * 15,
			ReadTimeout:  time.Second * 15,
			IdleTimeout:  time.Second * 60,
			Handler:      r, // Pass our instance of gorilla/mux in.
		}

		// Run our server in a goroutine so that it doesn't block.
		go func() {
			if err := srv.ListenAndServe(); err != nil {
				log.Println(err)
			}
		}()

		c := make(chan os.Signal, 1)
		// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
		// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
		signal.Notify(c, os.Interrupt)

		// Block until we receive our signal.
		<-c

		// Create a deadline to wait for.
		ctx, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()
		// Doesn't block if no connections, but will otherwise wait
		// until the timeout deadline.
		srv.Shutdown(ctx)
		// Optionally, you could run srv.Shutdown in a goroutine and block on
		// <-ctx.Done() if your application should wait for other services
		// to finalize based on context cancellation.
		log.Println("shutting down")
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
