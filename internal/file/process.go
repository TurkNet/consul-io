package file

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func CompareFiles(filePath string, consulValue string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %v", err)
	}

	fileHash := md5.Sum(content)
	consulHash := md5.Sum([]byte(consulValue))

	return fileHash == consulHash, nil
}

func ProcessDirectory(directory, consulAddr string, rateLimit, retryLimit int, sem chan struct{}, wg *sync.WaitGroup, ticker *time.Ticker, processFunc func(string, string, string, int, int, chan struct{}, *sync.WaitGroup, *time.Ticker)) {
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			kvPath, err := filepath.Rel(directory, path)
			if err != nil {
				return err
			}
			sem <- struct{}{}
			wg.Add(1)
			go processFunc(path, kvPath, consulAddr, retryLimit, rateLimit, sem, wg, ticker)
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error walking the path:", err)
	}
}
