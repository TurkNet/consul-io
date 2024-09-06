package consul

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/hashicorp/consul/api"
	"github.com/turknet/consul-io/internal/file"
)

func getClient(consulAddr, token string) (*api.Client, error) {
	config := api.DefaultConfig()
	config.Address = consulAddr

	if token != "" {
		config.Token = token
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("Error creating Consul client: %v", err)
	}

	_, err = client.Agent().Self()
	if err != nil {
		return nil, fmt.Errorf("Failed to authenticate with Consul using the provided token: %v", err)
	}

	return client, nil
}

func GetKV(client *api.Client, key string, retryLimit, rateLimit int, ticker *time.Ticker) (string, error) {
	var kvPair *api.KVPair
	var err error
	for i := 0; i < retryLimit; i++ {
		<-ticker.C
		kvPair, _, err = client.KV().Get(key, nil)
		if err == nil && kvPair != nil {
			return string(kvPair.Value), nil
		}
		time.Sleep(time.Duration(rateLimit) * time.Millisecond)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get KV from Consul: %v", err)
	}
	return "", nil
}

func CheckForSensitiveData(filePath string, content string) {
	sensitiveKeys := []string{"Password", "Token"}
	var problems []string

	for _, key := range sensitiveKeys {
		if strings.Contains(content, key) && !strings.Contains(content, `{{ with secret "kv/`) {
			problems = append(problems, fmt.Sprintf("Warning: The configuration contains a sensitive key '%s' that is not stored in Vault in file %s.", key, filePath))
		}
	}

	if len(problems) > 0 {
		f, err := os.OpenFile("problems.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			color.Red("Error opening problems.txt: %v", err)
			return
		}
		defer f.Close()

		for _, problem := range problems {
			if _, err := f.WriteString(problem + "\n"); err != nil {
				color.Red("Error writing to problems.txt: %v", err)
			}
			color.Yellow(problem)
		}
	}
}

func UploadToConsul(filePath, kvPath, consulAddr, token string, retryLimit, rateLimit int, sem chan struct{}, wg *sync.WaitGroup, ticker *time.Ticker) {
	defer wg.Done()
	client, err := getClient(consulAddr, token)
	if err != nil {
		color.Red("Error: %v", err)
		<-sem
		return
	}

	consulValue, err := GetKV(client, kvPath, retryLimit, rateLimit, ticker)
	if err != nil {
		color.Red("Error getting KV from Consul: %v", err)
		<-sem
		return
	}

	isSame, err := file.CompareFiles(filePath, consulValue)
	if err != nil {
		color.Red("Error comparing files: %v", err)
		<-sem
		return
	}

	if isSame {
		color.Green("No changes detected for file: %s", filePath)
		<-sem
		return
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		color.Red("Error reading file: %v", err)
		<-sem
		return
	}

	CheckForSensitiveData(filePath, string(data))

	kv := client.KV()
	p := &api.KVPair{Key: kvPath, Value: data}
	for i := 0; i < retryLimit; i++ {
		<-ticker.C
		_, err = kv.Put(p, nil)
		if err == nil {
			break
		}
		color.Yellow("Error uploading to Consul, retrying: %v", err)
		time.Sleep(time.Duration(rateLimit) * time.Millisecond)
	}

	if err != nil {
		color.Red("Error uploading to Consul: %v", err)
		<-sem
		return
	}

	color.Green("Uploaded %s to %s", filePath, kvPath)
	time.Sleep(time.Duration(rateLimit) * time.Millisecond)
	<-sem
}

func ExportFromConsul(directory, consulAddr, token string) {
	client, err := getClient(consulAddr, token)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}

	kv := client.KV()
	pairs, _, err := kv.List("/", nil)
	if err != nil {
		color.Red("Error fetching keys from Consul: %v", err)
		return
	}

	for _, pair := range pairs {
		if strings.HasSuffix(pair.Key, "/") {
			dirPath := filepath.Join(directory, pair.Key)
			err = os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				color.Red("Error creating directory: %v", err)
				continue
			}
			color.Green("Downloaded %s to %s", pair.Key, dirPath)
		} else {
			filePath := filepath.Join(directory, pair.Key)
			err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
			if err != nil {
				color.Red("Error creating directory: %v", err)
				continue
			}
			err = ioutil.WriteFile(filePath, pair.Value, 0644)
			if err != nil {
				color.Red("Error writing file: %v", err)
				continue
			}

			CheckForSensitiveData(filePath, string(pair.Value))

			color.Green("Downloaded %s to %s", pair.Key, filePath)
		}
	}
}
