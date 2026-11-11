package gitdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/turknet/consul-io/internal/syncstate"
)

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func changeKey(c syncstate.Change) string {
	return string(c.Kind) + "|" + c.OldPath + "|" + c.Path
}

func TestChanges_EndToEnd(t *testing.T) {
	repo := t.TempDir()
	gitRun(t, repo, "init")
	gitRun(t, repo, "checkout", "-b", "main")

	dev := filepath.Join(repo, "dev")
	write(t, filepath.Join(dev, "a.txt"), "1")
	write(t, filepath.Join(dev, "b.txt"), "2")
	write(t, filepath.Join(dev, "sub", "c.txt"), "3")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-m", "initial")
	base := gitRun(t, repo, "rev-parse", "HEAD")
	base = base[:len(base)-1] // trailing newline

	// Değişiklikler: modify a, add d, delete b, rename c -> c2
	write(t, filepath.Join(dev, "a.txt"), "1x")
	write(t, filepath.Join(dev, "d.txt"), "4")
	if err := os.Remove(filepath.Join(dev, "b.txt")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "mv", filepath.Join("dev", "sub", "c.txt"), filepath.Join("dev", "sub", "c2.txt"))
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-m", "changes")

	if head, err := Head(dev); err != nil || head == "" {
		t.Fatalf("Head hatası: %v", err)
	}
	if !HasCommit(dev, base) {
		t.Fatalf("base commit bulunmalıydı")
	}
	if HasCommit(dev, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef") {
		t.Fatalf("olmayan commit bulundu sanıldı")
	}

	changes, err := Changes(dev, base, nil)
	if err != nil {
		t.Fatalf("Changes hatası: %v", err)
	}
	got := map[string]bool{}
	for _, c := range changes {
		got[changeKey(c)] = true
	}

	want := []string{
		"modified||a.txt",
		"added||d.txt",
		"deleted|b.txt|",
		"renamed|sub/c.txt|sub/c2.txt",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("beklenen değişiklik yok: %s (alınan: %v)", w, got)
		}
	}
	if len(changes) != len(want) {
		t.Errorf("beklenen %d değişiklik, alınan %d: %v", len(want), len(changes), got)
	}
}

func TestChanges_IgnoreFilter(t *testing.T) {
	repo := t.TempDir()
	gitRun(t, repo, "init")
	gitRun(t, repo, "checkout", "-b", "main")
	dev := filepath.Join(repo, "dev")
	write(t, filepath.Join(dev, "keep.txt"), "k")
	write(t, filepath.Join(dev, "skip.txt"), "s")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-m", "initial")
	base := gitRun(t, repo, "rev-parse", "HEAD")
	base = base[:len(base)-1]

	write(t, filepath.Join(dev, "keep.txt"), "k2")
	write(t, filepath.Join(dev, "skip.txt"), "s2")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-m", "changes")

	changes, err := Changes(dev, base, []string{"dev/skip.txt"})
	if err != nil {
		t.Fatalf("Changes hatası: %v", err)
	}
	for _, c := range changes {
		if c.Path == "skip.txt" || c.OldPath == "skip.txt" {
			t.Errorf("ignore edilen dosya sonuçta var: %+v", c)
		}
	}
	foundKeep := false
	for _, c := range changes {
		if c.Path == "keep.txt" {
			foundKeep = true
		}
	}
	if !foundKeep {
		t.Errorf("keep.txt bekleniyordu")
	}
}
