package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "1.0.9"

var (
	consulAddr  string
	rateLimit   int
	retryLimit  int
	concurrency int
)

var rootCmd = &cobra.Command{
	Use:   "consul-io",
	Short: "Import and export config files to/from Consul KV store",
	Long: `Consul IO is a CLI tool used to import and export configuration files from a specified directory to the Consul KV store and vice versa.
Available commands are:
  - import: Upload config files to Consul KV store
  - export: Download config files from Consul KV store`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&consulAddr, "consul-addr", "http://localhost:8500", "Consul address")
	rootCmd.PersistentFlags().IntVar(&rateLimit, "rate-limit", 500, "Rate limit in milliseconds between each upload")
	rootCmd.PersistentFlags().IntVar(&retryLimit, "retry-limit", 5, "Number of retries for each upload in case of failure")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 5, "Number of concurrent uploads")
}
