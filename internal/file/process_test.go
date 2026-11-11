package file

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("dizin oluşturulamadı: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("dosya yazılamadı: %v", err)
	}
}

func TestScanDirectory_HashAndRelPaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "team1", ".env.production"), "A=1")
	writeFile(t, filepath.Join(root, "team2", "app", "settings.json"), "{}")

	got, err := ScanDirectory(root, nil)
	if err != nil {
		t.Fatalf("tarama hatası: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("beklenen 2 dosya, alınan %d: %v", len(got), got)
	}
	if got[filepath.Join("team1", ".env.production")] != HashBytes([]byte("A=1")) {
		t.Errorf("team1 hash eşleşmedi")
	}
	if got[filepath.Join("team2", "app", "settings.json")] != HashBytes([]byte("{}")) {
		t.Errorf("team2 hash eşleşmedi")
	}
}

func TestScanDirectory_Ignore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep", "a.txt"), "a")
	writeFile(t, filepath.Join(root, "skip", "b.txt"), "b")

	got, err := ScanDirectory(root, []string{filepath.Join(root, "skip")})
	if err != nil {
		t.Fatalf("tarama hatası: %v", err)
	}
	if _, ok := got[filepath.Join("skip", "b.txt")]; ok {
		t.Errorf("ignore edilen dosya sonuçta var")
	}
	if _, ok := got[filepath.Join("keep", "a.txt")]; !ok {
		t.Errorf("korunması gereken dosya eksik")
	}
}

func TestScanDirectory_ExcludeStateFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), "a")
	state := filepath.Join(root, "state.json")
	writeFile(t, state, "{}")

	got, err := ScanDirectory(root, nil, state)
	if err != nil {
		t.Fatalf("tarama hatası: %v", err)
	}
	if _, ok := got["state.json"]; ok {
		t.Errorf("state dosyası taranmamalıydı")
	}
	if _, ok := got["a.txt"]; !ok {
		t.Errorf("a.txt eksik")
	}
}
