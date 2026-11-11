package consul

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

// ReservedPrefix, consul-io'nun kendi meta verileri (ör. marker) için ayırdığı
// anahtar önekidir. export bu öneki atlar; böylece marker repoya sızmaz.
const ReservedPrefix = "_consul-io/"

// ExportFromConsul, Consul KV store'daki tüm kayıtları diske indirir.
// Mevcut davranış korunur.
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
		if strings.HasPrefix(pair.Key, ReservedPrefix) {
			continue
		}
		if strings.HasSuffix(pair.Key, "/") {
			dirPath := filepath.Join(directory, pair.Key)
			if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
				color.Red("Error creating directory: %v", err)
				continue
			}
			color.Green("Downloaded %s to %s", pair.Key, dirPath)
			continue
		}

		filePath := filepath.Join(directory, pair.Key)
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			color.Red("Error creating directory: %v", err)
			continue
		}
		if err := os.WriteFile(filePath, pair.Value, 0644); err != nil {
			color.Red("Error writing file: %v", err)
			continue
		}

		CheckForSensitiveData(filePath, string(pair.Value))
		color.Green("Downloaded %s to %s", pair.Key, filePath)
	}
}
