package syncstate

import "sort"

// ChangeKind, algılanan değişiklik türüdür.
type ChangeKind string

const (
	ChangeAdded    ChangeKind = "added"
	ChangeModified ChangeKind = "modified"
	ChangeDeleted  ChangeKind = "deleted"
	ChangeRenamed  ChangeKind = "renamed"
)

// Change, tek bir dosya için yapılacak işlemi tanımlar.
// Path: yeni/mevcut göreli yol. OldPath: silme/yeniden adlandırmada eski yol.
type Change struct {
	Kind    ChangeKind
	Path    string
	OldPath string
	Hash    string
}

// Diff, önceki state (prev) ile güncel tarama (current) arasındaki farkı üretir.
// prev nil ise tüm dosyalar "eklendi" kabul edilir (ilk çalıştırma / tam tarama).
//
// Yeniden adlandırma tespiti: içerik hash'i aynı olan bir "silinen" ve bir
// "eklenen" yol eşleştirilir. İçerik de değişmişse rename tespit edilemez ve
// sil+ekle olarak işlenir (doğru sonuç, yalnızca daha az optimal).
func Diff(prev, current map[string]string) []Change {
	if prev == nil {
		prev = map[string]string{}
	}
	if current == nil {
		current = map[string]string{}
	}

	// Silinen adayları hash'e göre grupla.
	deletedByHash := make(map[string][]string)
	for path, hash := range prev {
		if _, ok := current[path]; !ok {
			deletedByHash[hash] = append(deletedByHash[hash], path)
		}
	}
	for hash := range deletedByHash {
		sort.Strings(deletedByHash[hash])
	}
	deleted := make(map[string]bool)
	for _, paths := range deletedByHash {
		for _, p := range paths {
			deleted[p] = true
		}
	}

	var changes []Change

	// Değişen (modified) ve yeni (added) dosyaları belirle.
	var addedPaths []string
	for path, hash := range current {
		prevHash, existed := prev[path]
		if !existed {
			addedPaths = append(addedPaths, path)
			continue
		}
		if prevHash != hash {
			changes = append(changes, Change{Kind: ChangeModified, Path: path, Hash: hash})
		}
		// Hash aynıysa hiç işlem yok.
	}
	sort.Strings(addedPaths)

	// Eklentileri, aynı hash'li silinenlerle eşleştirerek rename tespit et.
	for _, path := range addedPaths {
		hash := current[path]
		if olds := deletedByHash[hash]; len(olds) > 0 {
			old := olds[0]
			deletedByHash[hash] = olds[1:]
			deleted[old] = false
			changes = append(changes, Change{Kind: ChangeRenamed, Path: path, OldPath: old, Hash: hash})
			continue
		}
		changes = append(changes, Change{Kind: ChangeAdded, Path: path, Hash: hash})
	}

	// Kalan (rename'e eşleşmeyen) silinenler.
	var deletedPaths []string
	for path := range deleted {
		if deleted[path] {
			deletedPaths = append(deletedPaths, path)
		}
	}
	sort.Strings(deletedPaths)
	for _, path := range deletedPaths {
		changes = append(changes, Change{Kind: ChangeDeleted, OldPath: path})
	}

	// Deterministik ve okunabilir sıra: türe, sonra yola göre.
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Kind != changes[j].Kind {
			return changes[i].Kind < changes[j].Kind
		}
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].OldPath < changes[j].OldPath
	})

	return changes
}
