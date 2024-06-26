package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
)

var consulAddr string

var rootCmd = &cobra.Command{
	Use:   "consul-uploader",
	Short: "Upload config files to Consul KV store",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Please provide the directory path")
			os.Exit(1)
		}
		directory := args[0]
		processDirectory(directory)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&consulAddr, "consul-addr", "http://localhost:8500", "Consul address")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func uploadToConsul(filePath, kvPath string) {
	config := api.DefaultConfig()
	config.Address = consulAddr
	client, err := api.NewClient(config)
	if err != nil {
		fmt.Println("Error creating Consul client:", err)
		return
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	kv := client.KV()
	p := &api.KVPair{Key: kvPath, Value: data}
	_, err = kv.Put(p, nil)
	if err != nil {
		fmt.Println("Error uploading to Consul:", err)
		return
	}

	fmt.Printf("Uploaded %s to %s\n", filePath, kvPath)
}

func processDirectory(directory string) {
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".production" {
			kvPath, err := filepath.Rel(directory, path)
			if err != nil {
				return err
			}
			uploadToConsul(path, kvPath)
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error walking the path:", err)
	}
}
