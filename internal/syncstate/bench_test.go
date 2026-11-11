package syncstate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const benchFileCount = 200

// simulatedLatency, uzak Consul'a yapılan tek bir KV çağrısının gecikmesini
// modellemek için kullanılan VARSAYIMSAL değerdir (hızlı bir yerel ağ).
// Gerçek ölçüm değildir; yalnızca bellek-içi store'un "bedava" olmasının
// yarattığı yanılsamayı gidermek için kullanılır.
const simulatedLatency = 1 * time.Millisecond

// latencyStore, her KV çağrısına sabit gecikme ekleyerek ağ maliyetini taklit eder.
type latencyStore struct {
	*memStore
	latency time.Duration
}

func (l *latencyStore) Get(key string) ([]byte, bool, error) {
	time.Sleep(l.latency)
	return l.memStore.Get(key)
}

func (l *latencyStore) Put(key string, value []byte) error {
	time.Sleep(l.latency)
	return l.memStore.Put(key, value)
}

func (l *latencyStore) Delete(key string) error {
	time.Sleep(l.latency)
	return l.memStore.Delete(key)
}

func setupTree(tb testing.TB) (*memStore, string) {
	tb.Helper()
	root := tb.TempDir()
	store := newMemStore()
	for i := 0; i < benchFileCount; i++ {
		rel := filepath.Join("team", fmt.Sprintf("app%03d", i), "config.env")
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			tb.Fatal(err)
		}
		content := []byte(fmt.Sprintf("KEY_%d=value_%d", i, i))
		if err := os.WriteFile(abs, content, 0o644); err != nil {
			tb.Fatal(err)
		}
		store.data[rel] = content
	}
	return store, root
}

// TestOpCount_UnchangedTree, aynı 200 dosyalık veri üzerinde değişiklik
// olmadığında yeni yaklaşımın hiç Consul işlemi yapmadığını; eski yaklaşımın
// (her dosya için GetKV+CompareFiles) 200 okuma yaptığını doğrular.
func TestOpCount_UnchangedTree(t *testing.T) {
	store := newMemStore()
	root := t.TempDir()
	s := &Syncer{Store: store, StatePath: filepath.Join(t.TempDir(), "state.json")}
	for i := 0; i < benchFileCount; i++ {
		rel := filepath.Join("team", fmt.Sprintf("app%03d", i), "config.env")
		writeFile(t, filepath.Join(root, rel), fmt.Sprintf("KEY_%d=value_%d", i, i))
	}
	// İlk çalıştırma state'i kurar (store boş -> hepsi yüklenir).
	if _, err := s.Run(root, nil, Options{Concurrency: 4, Prune: true}); err != nil {
		t.Fatalf("ilk Run: %v", err)
	}

	store.puts, store.gets, store.deletes = 0, 0, 0
	newRes, err := s.Run(root, nil, Options{Concurrency: 4, Prune: true})
	if err != nil {
		t.Fatalf("ikinci Run: %v", err)
	}
	newOps := store.puts + store.gets + store.deletes
	oldOps := benchFileCount // eski davranış: dosya başına bir Consul Get

	if newRes.Scanned != benchFileCount {
		t.Fatalf("taranan dosya sayısı yanlış: %d", newRes.Scanned)
	}
	if newOps != 0 {
		t.Fatalf("değişmeyen ağaçta yeni yaklaşım store işlemi yapmamalı, yaptı: %d", newOps)
	}
	if oldOps <= newOps {
		t.Fatalf("beklenen iyileşme yok: old=%d new=%d", oldOps, newOps)
	}
	t.Logf("değişmeyen %d dosya: eski yaklaşım=%d Consul işlemi, yeni yaklaşım=%d Consul işlemi", benchFileCount, oldOps, newOps)
}

// BenchmarkIncrementalUnchanged, değişmeyen ağaçta yeni yaklaşımı ölçer.
// Yalnızca yerel tarama/hash maliyeti vardır, Consul çağrısı yoktur.
func BenchmarkIncrementalUnchanged(b *testing.B) {
	store := newMemStore()
	root := b.TempDir()
	for i := 0; i < benchFileCount; i++ {
		rel := filepath.Join("team", fmt.Sprintf("app%03d", i), "config.env")
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			b.Fatal(err)
		}
		content := []byte(fmt.Sprintf("KEY_%d=value_%d", i, i))
		if err := os.WriteFile(abs, content, 0o644); err != nil {
			b.Fatal(err)
		}
		store.data[rel] = content
	}
	s := &Syncer{Store: store, StatePath: filepath.Join(b.TempDir(), "state.json")}
	if _, err := s.Run(root, nil, Options{Concurrency: 4, Prune: true}); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Run(root, nil, Options{Concurrency: 4, Prune: true}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOldFullCompare, eski yaklaşımı simüle eder: ağacı gezer ve her dosya
// için Store.Get çağırır (GetKV + CompareFiles). Store, ağ gecikmesini modelleyen
// latencyStore'dur.
func BenchmarkOldFullCompare(b *testing.B) {
	mem, root := setupTree(b)
	store := &latencyStore{memStore: mem, latency: simulatedLatency}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			_, _, _ = store.Get(rel)
			return nil
		})
	}
}
