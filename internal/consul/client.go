package consul

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/turknet/consul-io/internal/file"
)

func getClient(consulAddr string) (*api.Client, error) {
	config := api.DefaultConfig()
	config.Address = consulAddr
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("Error creating Consul client: %v", err)
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

func UploadToConsul(filePath, kvPath, consulAddr string, retryLimit, rateLimit int, sem chan struct{}, wg *sync.WaitGroup, ticker *time.Ticker) {
	defer wg.Done()
	client, err := getClient(consulAddr)
	if err != nil {
		fmt.Println(err)
		<-sem
		return
	}

	consulValue, err := GetKV(client, kvPath, retryLimit, rateLimit, ticker)
	if err != nil {
		fmt.Println("Error getting KV from Consul:", err)
		<-sem
		return
	}

	isSame, err := file.CompareFiles(filePath, consulValue)
	if err != nil {
		fmt.Println("Error comparing files:", err)
		<-sem
		return
	}

	if isSame {
		fmt.Printf("No changes detected for file: %s\n", filePath)
		<-sem
		return
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		<-sem
		return
	}

	kv := client.KV()
	p := &api.KVPair{Key: kvPath, Value: data}
	for i := 0; i < retryLimit; i++ {
		<-ticker.C
		_, err = kv.Put(p, nil)
		if err == nil {
			break
		}
		fmt.Println("Error uploading to Consul, retrying:", err)
		time.Sleep(time.Duration(rateLimit) * time.Millisecond)
	}

	if err != nil {
		fmt.Println("Error uploading to Consul:", err)
		<-sem
		return
	}

	fmt.Printf("Uploaded %s to %s\n", filePath, kvPath)
	time.Sleep(time.Duration(rateLimit) * time.Millisecond)
	<-sem
}

func ExportFromConsul(directory, consulAddr string) {
	client, err := getClient(consulAddr)
	if err != nil {
		fmt.Println(err)
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
