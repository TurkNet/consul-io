package file

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ShouldIgnorePath, verilen yolun ignore listesindeki bir önekle başlayıp
// başlamadığını döner. Mevcut filtre davranışı korunur.
func ShouldIgnorePath(path string, ignorePaths []string) bool {
	for _, ignore := range ignorePaths {
		if ignore == "" {
			continue
		}
		if strings.HasPrefix(path, ignore) {
			return true
		}
	}
	return false
}

// ScanDirectory, dizin ağacını dolaşıp her dosya için göreli yol -> sha256 hash
// eşlemesini döner. ignorePaths ile eşleşen yollar ve excludeAbs içindeki mutlak
// yollar (ör. state dosyası) atlanır.
//
// Not: yalnızca yerel dosya sistemi okunur, ağ çağrısı yapılmaz.
func ScanDirectory(root string, ignorePaths []string, excludeAbs ...string) (map[string]string, error) {
	excluded := make(map[string]struct{}, len(excludeAbs))
	for _, e := range excludeAbs {
		if e == "" {
			continue
		}
		if abs, err := filepath.Abs(e); err == nil {
			excluded[abs] = struct{}{}
		}
	}

	result := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ShouldIgnorePath(path, ignorePaths) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if abs, aerr := filepath.Abs(path); aerr == nil {
			if _, ok := excluded[abs]; ok {
				return nil
			}
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}

		hash, herr := HashFile(path)
		if herr != nil {
			return herr
		}
		result[rel] = hash
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// HashFile, bir dosyanın içeriğinin sha256 hash'ini hex olarak döner.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashBytes, bir byte diliminin içeriğinin sha256 hash'ini hex olarak döner.
func HashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
