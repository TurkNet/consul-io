package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/turknet/consul-io/internal/consul"
	"github.com/turknet/consul-io/internal/file"
	"github.com/turknet/consul-io/internal/gitdiff"
	"github.com/turknet/consul-io/internal/syncstate"
)

var (
	ignorePaths  []string
	stateFile    string
	fullSync     bool
	prune        bool
	maxDelete    int
	dryRun       bool
	gitDiffMode  bool
	changedSince string
	markerKey    string
)

var importCmd = &cobra.Command{
	Use:   "import [directory]",
	Short: "Import config files to Consul KV store",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		directory := args[0]

		ticker := time.NewTicker(time.Duration(rateLimit) * time.Millisecond)
		defer ticker.Stop()

		store, err := consul.NewStore(consulAddr, token, retryLimit, rateLimit, ticker)
		if err != nil {
			return err
		}

		if gitDiffMode || changedSince != "" || markerKey != "" {
			return runGitDiff(directory, store)
		}
		return runManifest(directory, store)
	},
}

// runManifest, yerel kullanım için manifest (state) tabanlı artımlı içe aktarım.
func runManifest(directory string, store syncstate.KVStore) error {
	sf := stateFile
	if sf == "" {
		sf = defaultStatePath(directory)
	}

	syncer := &syncstate.Syncer{Store: store, StatePath: sf, ConsulAddr: consulAddr}
	res, err := syncer.Run(directory, ignorePaths, syncstate.Options{
		Full:        fullSync,
		Prune:       prune,
		Concurrency: concurrency,
		MaxDelete:   maxDelete,
		DryRun:      dryRun,
		OnSensitive: consul.CheckForSensitiveData,
	})
	if err != nil {
		return err
	}
	printSummary(res)
	if dryRun {
		color.Cyan("dry-run: hiçbir şey uygulanmadı.")
		return nil
	}
	return finish(res)
}

// runGitDiff, pipeline için git diff tabanlı delta içe aktarımı.
// Baz commit marker'dan (Consul) veya --changed-since'den alınır. Baz yoksa
// güvenli şekilde tam taramaya düşülür (silme yapılmaz).
func runGitDiff(directory string, store syncstate.KVStore) error {
	opts := syncstate.Options{
		Prune:       prune,
		Concurrency: concurrency,
		MaxDelete:   maxDelete,
		DryRun:      dryRun,
		OnSensitive: consul.CheckForSensitiveData,
	}

	head, err := gitdiff.Head(directory)
	if err != nil {
		return fmt.Errorf("HEAD alınamadı: %w", err)
	}

	base := changedSince
	if base == "" && markerKey != "" {
		value, exists, err := store.Get(markerKey)
		if err != nil {
			return fmt.Errorf("marker okunamadı: %w", err)
		}
		if exists {
			base = strings.TrimSpace(string(value))
		}
	}

	var changes []syncstate.Change
	if base != "" && gitdiff.HasCommit(directory, base) {
		changes, err = gitdiff.Changes(directory, base, ignorePaths)
		if err != nil {
			return fmt.Errorf("git diff alınamadı: %w", err)
		}
		color.Cyan("Git diff: %s..HEAD -> %d değişiklik", base, len(changes))
	} else {
		if base != "" {
			color.Yellow("Baz commit (%s) yerel klonda yok; tam tarama yapılıyor (silme yok).", base)
		} else {
			color.Yellow("Marker yok; ilk çalıştırma, tam tarama yapılıyor (silme yok).")
		}
		all, err := file.ScanDirectory(directory, ignorePaths)
		if err != nil {
			return fmt.Errorf("dizin taranamadı: %w", err)
		}
		changes = make([]syncstate.Change, 0, len(all))
		for path, hash := range all {
			changes = append(changes, syncstate.Change{Kind: syncstate.ChangeAdded, Path: path, Hash: hash})
		}
	}

	syncer := &syncstate.Syncer{Store: store, ConsulAddr: consulAddr}
	res, err := syncer.ApplyChanges(directory, changes, opts)
	if err != nil {
		return err
	}
	printSummary(res)

	if dryRun {
		color.Cyan("dry-run: hiçbir şey uygulanmadı; marker güncellenmedi.")
		return nil
	}
	if res.Failed > 0 {
		// Marker ilerletilmez; sonraki çalıştırma aynı bazdan tekrar dener.
		return fail(res)
	}

	if markerKey != "" {
		if err := store.Put(markerKey, []byte(head)); err != nil {
			return fmt.Errorf("marker güncellenemedi: %w", err)
		}
		color.Green("Marker güncellendi: %s = %s", markerKey, head)
	}

	color.Green("Import process completed successfully.")
	return nil
}

func printSummary(res syncstate.Result) {
	color.Cyan("İşlenecek: %d | Eklenen: %d | Güncellenen: %d | Silinen: %d | Yeniden adlandırılan: %d | Atlanan: %d | Hatalı: %d",
		res.Scanned, res.Added, res.Modified, res.Deleted, res.Renamed, res.Skipped, res.Failed)
}

func finish(res syncstate.Result) error {
	if res.Failed > 0 {
		return fail(res)
	}
	color.Green("Import process completed successfully.")
	return nil
}

func fail(res syncstate.Result) error {
	color.Yellow("%d işlem başarısız; bu dosyalar işlenmiş olarak kaydedilmedi ve sonraki çalıştırmada yeniden denenecek.", res.Failed)
	return fmt.Errorf("%d işlem başarısız", res.Failed)
}

// defaultStatePath, hedef dizine özgü bir state dosyası yolu üretir.
// State, senkronize edilen ağacın dışında (kullanıcı config dizini) tutulur ki
// Consul'a yüklenmesin ve repoya sızmasın.
func defaultStatePath(directory string) string {
	abs, err := filepath.Abs(directory)
	if err != nil {
		abs = directory
	}
	sum := sha256.Sum256([]byte(abs))
	name := "state-" + hex.EncodeToString(sum[:8]) + ".json"

	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			home = "."
		}
		base = filepath.Join(home, ".consul-io")
	} else {
		base = filepath.Join(base, "consul-io")
	}
	return filepath.Join(base, name)
}

func init() {
	importCmd.Flags().StringSliceVar(&ignorePaths, "ignore", []string{}, "List of paths to ignore during import")
	importCmd.Flags().StringVar(&stateFile, "state-file", "", "Sync state (manifest) file path for local use. Defaults to user config dir")
	importCmd.Flags().BoolVar(&fullSync, "full", false, "Force a full scan and compare every file against Consul (local controlled resync)")
	importCmd.Flags().BoolVar(&gitDiffMode, "git-diff", false, "Only import files changed since the last applied commit (uses git diff)")
	importCmd.Flags().StringVar(&changedSince, "changed-since", "", "Explicit git base ref/SHA to diff against (implies --git-diff)")
	importCmd.Flags().StringVar(&markerKey, "marker-key", "", "Consul key storing the last applied commit SHA (e.g. _consul-io/main/last-applied). Implies --git-diff")
	importCmd.Flags().BoolVar(&prune, "prune", false, "Delete Consul keys for files removed/renamed in git. Disabled by default for safety")
	importCmd.Flags().IntVar(&maxDelete, "max-delete", 1, "Maximum number of deletions allowed per run (0 = unlimited). Aborts if exceeded")
	importCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned Put/Delete operations without applying them")
	rootCmd.AddCommand(importCmd)
}
