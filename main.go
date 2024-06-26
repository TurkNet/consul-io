package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
)

const version = "1.0.9"

var consulAddr string
var directory string
var rateLimit int
var retryLimit int

var rootCmd = &cobra.Command{
	Use:   "consul-io",
	Short: "Import and export config files to/from Consul KV store",
	Long: `Consul IO is a CLI tool used to import and export configuration files from a specified directory to the Consul KV store and vice versa.
Available commands are:
  - import: Upload config files to Consul KV store
  - export: Download config files from Consul KV store`,
}

var importCmd = &cobra.Command{
	Use:   "import [directory]",
	Short: "Import config files to Consul KV store",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		directory = args[0]
		processDirectory(directory, uploadToConsul)
	},
}

var exportCmd = &cobra.Command{
	Use:   "export [directory]",
	Short: "Export config files from Consul KV store",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		directory = args[0]
		exportFromConsul(directory)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Consul IO",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Consul IO v" + version)
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.PersistentFlags().StringVar(&consulAddr, "consul-addr", "http://localhost:8500", "Consul address")
	rootCmd.PersistentFlags().IntVar(&rateLimit, "rate-limit", 500, "Rate limit in milliseconds between each upload")
	rootCmd.PersistentFlags().IntVar(&retryLimit, "retry-limit", 5, "Number of retries for each upload in case of failure")
}

func main() {
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
	for i := 0; i < retryLimit; i++ {
		_, err = kv.Put(p, nil)
		if err == nil {
			break
		}
		fmt.Println("Error uploading to Consul, retrying:", err)
		time.Sleep(time.Duration(rateLimit) * time.Millisecond)
	}

	if err != nil {
		fmt.Println("Error uploading to Consul:", err)
		return
	}

	fmt.Printf("Uploaded %s to %s\n", filePath, kvPath)
	time.Sleep(time.Duration(rateLimit) * time.Millisecond)
}

func exportFromConsul(directory string) {
	config := api.DefaultConfig()
	config.Address = consulAddr
	client, err := api.NewClient(config)
	if err != nil {
		fmt.Println("Error creating Consul client:", err)
		return
	}

	kv := client.KV()
	pairs, _, err := kv.List("/", nil)
	if err != nil {
		fmt.Println("Error fetching keys from Consul:", err)
		return
	}

	for _, pair := range pairs {
		if strings.HasSuffix(pair.Key, "/") {
			// Anahtar bir dizin belirtir, bu nedenle dizin oluştur
			dirPath := filepath.Join(directory, pair.Key)
			err = os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				fmt.Println("Error creating directory:", err)
				continue
			}
			fmt.Printf("Downloaded %s to %s\n", pair.Key, dirPath)
		} else {
			// Anahtar bir dosya belirtir, dosyayı oluştur
			filePath := filepath.Join(directory, pair.Key)
			err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
			if err != nil {
				fmt.Println("Error creating directory:", err)
				continue
			}
			err = ioutil.WriteFile(filePath, pair.Value, 0644)
			if err != nil {
				fmt.Println("Error writing file:", err)
				continue
			}
			fmt.Printf("Downloaded %s to %s\n", pair.Key, filePath)
		}
	}
}

func processDirectory(directory string, processFunc func(string, string)) {
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			kvPath, err := filepath.Rel(directory, path)
			if err != nil {
				return err
			}
			processFunc(path, kvPath)
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error walking the path:", err)
	}
}
