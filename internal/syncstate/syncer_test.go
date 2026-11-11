package syncstate

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// memStore, KVStore'un bellek içi sahtesidir.
type memStore struct {
	mu         sync.Mutex
	data       map[string][]byte
	failPut    map[string]bool
	failDelete map[string]bool
	gets       int
	puts       int
	deletes    int
}

func newMemStore() *memStore {
	return &memStore{
		data:       map[string][]byte{},
		failPut:    map[string]bool{},
		failDelete: map[string]bool{},
	}
}

func (m *memStore) Get(key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gets++
	v, ok := m.data[key]
	if !ok {
		return nil, false, nil
	}
	cp := append([]byte(nil), v...)
	return cp, true, nil
}

func (m *memStore) Put(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	if m.failPut[key] {
		return errors.New("put hatası")
	}
	m.data[key] = append([]byte(nil), value...)
	return nil
}

func (m *memStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletes++
	if m.failDelete[key] {
		return errors.New("delete hatası")
	}
	delete(m.data, key)
	return nil
}

func (m *memStore) has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[key]
	return ok
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("dizin oluşturulamadı: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("dosya yazılamadı: %v", err)
	}
}

func newSyncer(t *testing.T, store KVStore) (*Syncer, string) {
	t.Helper()
	root := t.TempDir()
	return &Syncer{
		Store:      store,
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		ConsulAddr: "test",
	}, root
}

func TestSyncer_FirstRunUploadsAll(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "a")
	writeFile(t, filepath.Join(root, "sub", "b.txt"), "b")

	res, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if res.Added != 2 || res.Scanned != 2 {
		t.Fatalf("beklenen added=2 scanned=2, alınan %+v", res)
	}
	if !store.has("a.txt") || !store.has(filepath.Join("sub", "b.txt")) {
		t.Errorf("dosyalar store'da yok")
	}

	st, err := LoadState(s.StatePath)
	if err != nil || st == nil {
		t.Fatalf("state yüklenemedi: %v", err)
	}
	if len(st.Files) != 2 {
		t.Errorf("state'te 2 dosya bekleniyordu, alınan %d", len(st.Files))
	}
}

func TestSyncer_SecondRunNoStoreCalls(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "a")
	writeFile(t, filepath.Join(root, "b.txt"), "b")

	if _, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true}); err != nil {
		t.Fatalf("ilk Run hatası: %v", err)
	}
	putsAfterFirst := store.puts

	res, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("ikinci Run hatası: %v", err)
	}
	if res.Added != 0 || res.Modified != 0 || res.Deleted != 0 || res.Renamed != 0 {
		t.Fatalf("ikinci çalıştırmada işlem beklenmiyordu: %+v", res)
	}
	if store.puts != putsAfterFirst || store.deletes != 0 || store.gets != 0 {
		t.Errorf("değişmeyen dosyalar için store çağrısı yapıldı: %+v", store)
	}
}

func TestSyncer_ModifiedFile(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "a")
	if _, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true}); err != nil {
		t.Fatalf("ilk Run hatası: %v", err)
	}

	writeFile(t, filepath.Join(root, "a.txt"), "a2")
	res, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if res.Modified != 1 || res.Added != 0 {
		t.Fatalf("beklenen modified=1, alınan %+v", res)
	}
	if string(store.data["a.txt"]) != "a2" {
		t.Errorf("store güncellenmedi: %q", store.data["a.txt"])
	}
}

func TestSyncer_DeletedFileRemovesKeyAndState(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "a")
	writeFile(t, filepath.Join(root, "b.txt"), "b")
	if _, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true}); err != nil {
		t.Fatalf("ilk Run hatası: %v", err)
	}

	if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
		t.Fatalf("dosya silinemedi: %v", err)
	}
	res, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if res.Deleted != 1 {
		t.Fatalf("beklenen deleted=1, alınan %+v", res)
	}
	if store.has("b.txt") {
		t.Errorf("store'dan silinmedi")
	}
	st, _ := LoadState(s.StatePath)
	if _, ok := st.Files["b.txt"]; ok {
		t.Errorf("state'ten düşürülmedi")
	}
}

func TestSyncer_RenamedFile(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "old.txt"), "same")
	if _, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true}); err != nil {
		t.Fatalf("ilk Run hatası: %v", err)
	}

	if err := os.Rename(filepath.Join(root, "old.txt"), filepath.Join(root, "new.txt")); err != nil {
		t.Fatalf("rename hatası: %v", err)
	}
	res, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if res.Renamed != 1 {
		t.Fatalf("beklenen renamed=1, alınan %+v", res)
	}
	if !store.has("new.txt") || store.has("old.txt") {
		t.Errorf("rename store'a yansımadı: new=%v old=%v", store.has("new.txt"), store.has("old.txt"))
	}
}

func TestSyncer_PruneFalseKeepsKey(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "a")
	if _, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true}); err != nil {
		t.Fatalf("ilk Run hatası: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "a.txt")); err != nil {
		t.Fatalf("silme hatası: %v", err)
	}

	if _, err := s.Run(root, nil, Options{Concurrency: 1, Prune: false}); err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if !store.has("a.txt") {
		t.Errorf("Prune=false iken anahtar silinmemeliydi")
	}
}

func TestSyncer_FailedPutNotRecordedThenRecovers(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "a")
	store.failPut["a.txt"] = true

	res, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if res.Failed != 1 {
		t.Fatalf("beklenen failed=1, alınan %+v", res)
	}
	st, _ := LoadState(s.StatePath)
	if _, ok := st.Files["a.txt"]; ok {
		t.Fatalf("başarısız dosya state'e işlenmiş olarak kaydedildi")
	}

	// Hata giderildikten sonra aynı dosya yeniden denenmeli.
	delete(store.failPut, "a.txt")
	res, err = s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("ikinci Run hatası: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("kurtarma sonrası added=1 bekleniyordu, alınan %+v", res)
	}
	if !store.has("a.txt") {
		t.Errorf("kurtarma sonrası store'a yazılmadı")
	}
}

func TestSyncer_FailedDeleteInRenameRecovers(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "old.txt"), "same")
	if _, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true}); err != nil {
		t.Fatalf("ilk Run hatası: %v", err)
	}

	if err := os.Rename(filepath.Join(root, "old.txt"), filepath.Join(root, "new.txt")); err != nil {
		t.Fatalf("rename hatası: %v", err)
	}
	store.failDelete["old.txt"] = true

	res, err := s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if res.Failed != 1 {
		t.Fatalf("beklenen failed=1, alınan %+v", res)
	}
	// Yeni anahtar yazıldı, eski anahtar silinemediği için state'te kalmalı.
	if !store.has("new.txt") || !store.has("old.txt") {
		t.Fatalf("kısmi başarı durumu yanlış: new=%v old=%v", store.has("new.txt"), store.has("old.txt"))
	}
	st, _ := LoadState(s.StatePath)
	if _, ok := st.Files["old.txt"]; !ok {
		t.Fatalf("silinemeyen eski anahtar state'te kalmalıydı")
	}

	// Hata giderilince eski anahtar sonraki turda silinmeli.
	delete(store.failDelete, "old.txt")
	res, err = s.Run(root, nil, Options{Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("ikinci Run hatası: %v", err)
	}
	if res.Deleted != 1 || store.has("old.txt") {
		t.Fatalf("kurtarma sonrası eski anahtar silinmedi: %+v", res)
	}
}

func TestSyncer_FullModeSkipsEqual(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "a")
	store.data["a.txt"] = []byte("a")

	res, err := s.Run(root, nil, Options{Full: true, Concurrency: 1, Prune: true})
	if err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if res.Skipped != 1 || res.Added != 0 {
		t.Fatalf("beklenen skipped=1 added=0, alınan %+v", res)
	}
	if store.puts != 0 {
		t.Errorf("eşit içerik için store'a yazıldı")
	}
}

func TestSyncer_SensitiveHookCalled(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "Password=secret")

	var called int
	if _, err := s.Run(root, nil, Options{
		Concurrency: 1,
		Prune:       true,
		OnSensitive: func(string, string) { called++ },
	}); err != nil {
		t.Fatalf("Run hatası: %v", err)
	}
	if called != 1 {
		t.Errorf("hassas veri kancası çağrılmadı: %d", called)
	}
}

func TestSyncer_ApplyChangesWithPrune(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "new.txt"), "n")
	writeFile(t, filepath.Join(root, "renamed.txt"), "r")
	store.data["old.txt"] = []byte("r")
	store.data["gone.txt"] = []byte("g")

	changes := []Change{
		{Kind: ChangeAdded, Path: "new.txt"},
		{Kind: ChangeRenamed, OldPath: "old.txt", Path: "renamed.txt"},
		{Kind: ChangeDeleted, OldPath: "gone.txt"},
	}
	res, err := s.ApplyChanges(root, changes, Options{Prune: true, Concurrency: 1})
	if err != nil {
		t.Fatalf("ApplyChanges hatası: %v", err)
	}
	if res.Added != 1 || res.Renamed != 1 || res.Deleted != 1 {
		t.Fatalf("beklenmeyen sonuç: %+v", res)
	}
	if !store.has("new.txt") || !store.has("renamed.txt") {
		t.Errorf("yeni anahtarlar yazılmadı")
	}
	if store.has("old.txt") || store.has("gone.txt") {
		t.Errorf("silinecek anahtarlar kaldı")
	}
}

func TestSyncer_ApplyChangesWithoutPruneKeepsKeys(t *testing.T) {
	store := newMemStore()
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "new.txt"), "n")
	store.data["old.txt"] = []byte("r")
	store.data["gone.txt"] = []byte("g")

	changes := []Change{
		{Kind: ChangeAdded, Path: "new.txt"},
		{Kind: ChangeRenamed, OldPath: "old.txt", Path: "new.txt"},
		{Kind: ChangeDeleted, OldPath: "gone.txt"},
	}
	if _, err := s.ApplyChanges(root, changes, Options{Prune: false, Concurrency: 1}); err != nil {
		t.Fatalf("ApplyChanges hatası: %v", err)
	}
	if !store.has("new.txt") {
		t.Errorf("yeni dosya yazılmalıydı")
	}
	if !store.has("old.txt") || !store.has("gone.txt") {
		t.Errorf("prune kapalıyken hiçbir anahtar silinmemeliydi")
	}
	if store.deletes != 0 {
		t.Errorf("prune kapalıyken Delete çağrılmamalıydı")
	}
}

func TestSyncer_MaxDeleteAborts(t *testing.T) {
	store := newMemStore()
	store.data["a"] = []byte("a")
	store.data["b"] = []byte("b")
	s, root := newSyncer(t, store)

	changes := []Change{
		{Kind: ChangeDeleted, OldPath: "a"},
		{Kind: ChangeDeleted, OldPath: "b"},
	}
	_, err := s.ApplyChanges(root, changes, Options{Prune: true, MaxDelete: 1, Concurrency: 1})
	if err == nil {
		t.Fatalf("max-delete aşımında hata beklenirdi")
	}
	if store.deletes != 0 {
		t.Errorf("sınır aşımında hiçbir silme uygulanmamalıydı")
	}
	if !store.has("a") || !store.has("b") {
		t.Errorf("anahtarlar korunmalıydı")
	}
}

func TestSyncer_DryRunMakesNoChanges(t *testing.T) {
	store := newMemStore()
	store.data["gone.txt"] = []byte("g")
	s, root := newSyncer(t, store)
	writeFile(t, filepath.Join(root, "a.txt"), "a")

	changes := []Change{
		{Kind: ChangeAdded, Path: "a.txt"},
		{Kind: ChangeDeleted, OldPath: "gone.txt"},
	}
	if _, err := s.ApplyChanges(root, changes, Options{DryRun: true, Prune: true, Concurrency: 1}); err != nil {
		t.Fatalf("ApplyChanges hatası: %v", err)
	}
	if store.puts != 0 || store.deletes != 0 {
		t.Errorf("dry-run'da store'a yazılmamalıydı: puts=%d deletes=%d", store.puts, store.deletes)
	}
	if !store.has("gone.txt") {
		t.Errorf("dry-run'da silinmemeliydi")
	}
}
