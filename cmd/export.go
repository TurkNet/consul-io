package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/turknet/consul-io/internal/consul"
)

var exportCmd = &cobra.Command{
	Use:   "export [directory]",
	Short: "Export config files from Consul KV store",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		directory := args[0]
		consul.ExportFromConsul(directory, consulAddr, token)
		color.Green("Export process completed successfully.")
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
