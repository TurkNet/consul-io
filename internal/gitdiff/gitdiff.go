package gitdiff

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/turknet/consul-io/internal/file"
	"github.com/turknet/consul-io/internal/syncstate"
)

// RepoRoot, verilen dizinin içinde bulunduğu git repo kökünü döner.
func RepoRoot(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Head, mevcut HEAD commit SHA'sını döner.
func Head(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// HasCommit, verilen commit'in yerel klonda bulunup bulunmadığını döner.
func HasCommit(dir, sha string) bool {
	if sha == "" {
		return false
	}
	_, err := run(dir, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

// Changes, base..HEAD arasında importDir altında değişen dosyaları
// syncstate.Change listesine çevirir. Yalnızca yerel git kullanılır.
func Changes(importDir, base string, ignorePaths []string) ([]syncstate.Change, error) {
	gitRoot, err := RepoRoot(importDir)
	if err != nil {
		return nil, err
	}
	absImport, err := filepath.Abs(importDir)
	if err != nil {
		return nil, err
	}
	// macOS'ta /var -> /private/var gibi symlink'ler göreli yolu bozar.
	if resolved, e := filepath.EvalSymlinks(gitRoot); e == nil {
		gitRoot = resolved
	}
	if resolved, e := filepath.EvalSymlinks(absImport); e == nil {
		absImport = resolved
	}
	relDir, err := filepath.Rel(gitRoot, absImport)
	if err != nil {
		return nil, err
	}
	relDir = filepath.ToSlash(relDir)

	// -M: rename tespiti, --diff-filter: A(added) C(copied) M(modified) R(renamed) D(deleted)
	out, err := run(gitRoot, "diff", "--name-status", "-M", "--diff-filter=ACMRD", base, "HEAD", "--", relDir)
	if err != nil {
		return nil, err
	}

	var changes []syncstate.Change
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0][0]

		switch status {
		case 'A', 'C': // C (copy) iki yol verir; hedef son alandır.
			newPath := fields[len(fields)-1]
			if file.ShouldIgnorePath(newPath, ignorePaths) {
				continue
			}
			changes = append(changes, syncstate.Change{
				Kind: syncstate.ChangeAdded,
				Path: kvKey(gitRoot, absImport, newPath),
			})
		case 'M':
			p := fields[1]
			if file.ShouldIgnorePath(p, ignorePaths) {
				continue
			}
			changes = append(changes, syncstate.Change{
				Kind: syncstate.ChangeModified,
				Path: kvKey(gitRoot, absImport, p),
			})
		case 'D':
			p := fields[1]
			if file.ShouldIgnorePath(p, ignorePaths) {
				continue
			}
			changes = append(changes, syncstate.Change{
				Kind:    syncstate.ChangeDeleted,
				OldPath: kvKey(gitRoot, absImport, p),
			})
		case 'R':
			if len(fields) < 3 {
				continue
			}
			oldPath, newPath := fields[1], fields[2]
			if file.ShouldIgnorePath(oldPath, ignorePaths) || file.ShouldIgnorePath(newPath, ignorePaths) {
				continue
			}
			changes = append(changes, syncstate.Change{
				Kind:    syncstate.ChangeRenamed,
				OldPath: kvKey(gitRoot, absImport, oldPath),
				Path:    kvKey(gitRoot, absImport, newPath),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return changes, nil
}

// kvKey, repo-göreli git yolunu, import dizinine göreli Consul anahtarına
// çevirir (tam tarama modundaki filepath.Rel davranışıyla aynı).
func kvKey(gitRoot, absImport, gitPath string) string {
	abs := filepath.Join(gitRoot, filepath.FromSlash(gitPath))
	rel, err := filepath.Rel(absImport, abs)
	if err != nil {
		return filepath.FromSlash(gitPath)
	}
	return rel
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
