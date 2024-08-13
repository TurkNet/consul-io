package cmd

import (
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/turknet/consul-io/internal/consul"
	"github.com/turknet/consul-io/internal/file"
)

var (
	ignorePaths []string
)

var importCmd = &cobra.Command{
	Use:   "import [directory]",
	Short: "Import config files to Consul KV store",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		directory := args[0]
		var wg sync.WaitGroup
		sem := make(chan struct{}, concurrency)
		ticker := time.NewTicker(time.Duration(rateLimit) * time.Millisecond)
		defer ticker.Stop()

		file.ProcessDirectory(directory, consulAddr, rateLimit, retryLimit, ignorePaths, sem, &wg, ticker, consul.UploadToConsul)
		wg.Wait()
		close(sem)
	},
}

func init() {
	importCmd.Flags().StringSliceVar(&ignorePaths, "ignore", []string{}, "List of paths to ignore during import")
	rootCmd.AddCommand(importCmd)
}
