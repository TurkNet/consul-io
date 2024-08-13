package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the Consul IO CLI to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Updating Consul IO CLI...")

		var installCmd *exec.Cmd
		if runtime.GOOS == "windows" {
			installCmd = exec.Command("go", "install", "github.com/turknet/consul-io@latest")
		} else {
			installCmd = exec.Command("sh", "-c", "go install github.com/turknet/consul-io@latest")
		}

		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		err := installCmd.Run()
		if err != nil {
			fmt.Printf("Error updating Consul IO CLI: %v\n", err)
			return
		}

		fmt.Println("Consul IO CLI updated to the latest version.")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
