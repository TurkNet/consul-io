package syncstate

import (
	"fmt"
	"os"
	"path/filepath"
	stdsync "sync"

	"github.com/fatih/color"
	"github.com/turknet/consul-io/internal/file"
)

// KVStore, senkronizasyonun ihtiyaç duyduğu KV işlemlerinin soyutlamasıdır.
// Consul ve testlerdeki bellek içi sahte store bu arayüzü uygular.
type KVStore interface {
	// Get, anahtarın değerini döner. exists=false ise anahtar yoktur.
	Get(key string) (value []byte, exists bool, err error)
	// Put, anahtarı değerle yazar/günceller.
	Put(key string, value []byte) error
	// Delete, anahtarı siler. Anahtar yoksa hata dönmemelidir.
	Delete(key string) error
}

// Options, senkronizasyon davranışını belirler.
type Options struct {
	// Full true ise state yok sayılır ve tüm dosyalar Consul ile karşılaştırılır
	// (eski tam tarama davranışı). Kontrollü yeniden eşitleme içindir.
	Full bool
	// Prune true ise silinen/yeniden adlandırılan dosyaların Consul anahtarları
	// da silinir. GÜVENLİK için varsayılan false'tur; yanlışlıkla silmeyi önler.
	Prune bool
	// Concurrency, eşzamanlı Consul işlemi sayısıdır. <=0 ise 1 kabul edilir.
	Concurrency int
	// MaxDelete, tek çalıştırmada izin verilen en fazla silme sayısıdır.
	// Prune true ve MaxDelete > 0 iken planlanan silme sayısı bu değeri aşarsa
	// işlem hiçbir şey uygulanmadan durdurulur. <=0 ise sınır yoktur.
	MaxDelete int
	// DryRun true ise hiçbir Put/Delete uygulanmaz; yalnızca plan yazdırılır.
	DryRun bool
	// OnSensitive, yükleme öncesi hassas veri kontrolü için opsiyonel kancadır.
	OnSensitive func(filePath, content string)
}

// Result, bir senkronizasyon turunun özetidir.
type Result struct {
	Scanned  int
	Added    int
	Modified int
	Deleted  int
	Renamed  int
	Skipped  int
	Failed   int
}

// Syncer, state tabanlı artımlı senkronizasyonu yürütür.
type Syncer struct {
	Store      KVStore
	StatePath  string
	ConsulAddr string
}

// stateMutation, başarılı bir işlemin state'e etkisini tanımlar.
type stateMutation struct {
	set     map[string]string
	del     []string
	skipped bool
}

// Run, kök dizini tarar, önceki state ile karşılaştırır ve yalnızca değişen
// dosyaları işler. Başarısız işlemler state'e yazılmaz; bir sonraki çalıştırmada
// yeniden denenir.
func (s *Syncer) Run(root string, ignorePaths []string, opts Options) (Result, error) {
	var res Result
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}

	current, err := file.ScanDirectory(root, ignorePaths, s.StatePath)
	if err != nil {
		return res, fmt.Errorf("dizin taranamadı: %w", err)
	}
	res.Scanned = len(current)

	st := NewState(s.ConsulAddr)
	var prev map[string]string

	if !opts.Full {
		loaded, err := LoadState(s.StatePath)
		if err != nil {
			return res, err
		}
		if loaded != nil {
			// Farklı bir Consul adresine geçildiyse eski state güvenilmez;
			// kontrollü olarak tam taramaya düş.
			sameCluster := loaded.ConsulAddr == "" || s.ConsulAddr == "" || loaded.ConsulAddr == s.ConsulAddr
			if sameCluster {
				st = loaded
				prev = loaded.Files
			} else {
				color.Yellow("State farklı bir Consul adresine (%s) ait; tam tarama yapılıyor.", loaded.ConsulAddr)
			}
		}
		// loaded == nil => ilk çalıştırma; prev nil kalır, tüm dosyalar eklenir.
	}

	changes := Diff(prev, current)
	if len(changes) == 0 {
		color.Cyan("Değişiklik yok, işlenecek dosya bulunmuyor.")
		return res, nil
	}

	if opts.DryRun {
		s.plan(changes, opts)
		return res, nil
	}
	if err := s.checkDeleteCap(changes, opts); err != nil {
		return res, err
	}

	s.applyChanges(root, changes, opts, st, &res)

	if err := SaveState(s.StatePath, st); err != nil {
		return res, err
	}
	return res, nil
}

// ApplyChanges, dışarıdan verilen (ör. git diff'ten üretilen) değişiklik
// kümesini uygular. State/manifest kullanmaz; bu yüzden pipeline gibi geçici
// ortamlarda güvenle kullanılır. Silme, yalnızca Options.Prune true ise yapılır.
func (s *Syncer) ApplyChanges(root string, changes []Change, opts Options) (Result, error) {
	var res Result
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	res.Scanned = len(changes)

	if opts.DryRun {
		s.plan(changes, opts)
		return res, nil
	}
	if err := s.checkDeleteCap(changes, opts); err != nil {
		return res, err
	}

	// Git modunda kalıcı state tutulmaz; geçici state yalnızca arayüz için.
	st := NewState(s.ConsulAddr)
	s.applyChanges(root, changes, opts, st, &res)
	return res, nil
}

// checkDeleteCap, planlanan silme sayısı MaxDelete sınırını aşarsa hata döner.
// Böylece yanlış baz/dizin nedeniyle toplu silme engellenir.
func (s *Syncer) checkDeleteCap(changes []Change, opts Options) error {
	if !opts.Prune || opts.MaxDelete <= 0 {
		return nil
	}
	deletions := 0
	for _, ch := range changes {
		if ch.Kind == ChangeDeleted {
			deletions++
		}
	}
	if deletions > opts.MaxDelete {
		return fmt.Errorf("planlanan silme sayısı (%d) --max-delete (%d) sınırını aşıyor; işlem durduruldu", deletions, opts.MaxDelete)
	}
	return nil
}

// plan, DryRun modunda yapılacak işlemleri uygulamadan yazdırır.
func (s *Syncer) plan(changes []Change, opts Options) {
	for _, ch := range changes {
		switch ch.Kind {
		case ChangeAdded, ChangeModified:
			color.Cyan("[dry-run] PUT    %s", ch.Path)
		case ChangeDeleted:
			if opts.Prune {
				color.Yellow("[dry-run] DELETE %s", ch.OldPath)
			} else {
				color.Yellow("[dry-run] SKIP   %s (prune kapalı, silinmeyecek)", ch.OldPath)
			}
		case ChangeRenamed:
			color.Cyan("[dry-run] PUT    %s", ch.Path)
			if opts.Prune {
				color.Yellow("[dry-run] DELETE %s", ch.OldPath)
			} else {
				color.Yellow("[dry-run] SKIP   %s (prune kapalı, silinmeyecek)", ch.OldPath)
			}
		}
	}
}

// applyChanges, değişiklikleri eşzamanlı olarak işler ve state'i yalnızca
// başarılı işlemler için günceller.
func (s *Syncer) applyChanges(root string, changes []Change, opts Options, st *State, res *Result) {
	sem := make(chan struct{}, opts.Concurrency)
	var wg stdsync.WaitGroup
	var mu stdsync.Mutex

	for _, ch := range changes {
		wg.Add(1)
		sem <- struct{}{}
		go func(ch Change) {
			defer wg.Done()
			defer func() { <-sem }()

			mut, err := s.processChange(root, ch, opts)

			mu.Lock()
			defer mu.Unlock()
			commitMutation(st, mut)

			switch {
			case err != nil:
				res.Failed++
				color.Red("Hata (%s %s): %v", ch.Kind, changeLabel(ch), err)
			case mut.skipped:
				res.Skipped++
				color.Cyan("Değişiklik yok (Consul güncel): %s", ch.Path)
			default:
				switch ch.Kind {
				case ChangeAdded:
					res.Added++
					color.Green("Eklendi: %s", ch.Path)
				case ChangeModified:
					res.Modified++
					color.Green("Güncellendi: %s", ch.Path)
				case ChangeDeleted:
					res.Deleted++
					color.Green("Silindi: %s", ch.OldPath)
				case ChangeRenamed:
					res.Renamed++
					color.Green("Yeniden adlandırıldı: %s -> %s", ch.OldPath, ch.Path)
				}
			}
		}(ch)
	}

	wg.Wait()
}

// processChange, tek bir değişikliği uygular ve state'e yazılacak mutasyonu
// döner. Dönen mutasyon yalnızca gerçekten başarılı olan kısmı içerir.
func (s *Syncer) processChange(root string, ch Change, opts Options) (stateMutation, error) {
	switch ch.Kind {
	case ChangeAdded, ChangeModified:
		return s.upload(root, ch.Path, ch.Hash, opts)
	case ChangeDeleted:
		if !opts.Prune {
			// Güvenlik: --prune verilmediği için Consul'dan silinmez.
			color.Yellow("Silme atlandı (--prune verilmedi): %s", ch.OldPath)
			return stateMutation{del: []string{ch.OldPath}}, nil
		}
		if err := s.Store.Delete(ch.OldPath); err != nil {
			return stateMutation{}, err
		}
		return stateMutation{del: []string{ch.OldPath}}, nil
	case ChangeRenamed:
		// Önce yeni anahtarı yaz (veri kaybını önlemek için), sonra eskisini sil.
		mut, err := s.upload(root, ch.Path, ch.Hash, opts)
		if err != nil {
			return stateMutation{}, err
		}
		if !opts.Prune {
			// Yeni anahtar yazıldı; eski anahtar güvenlik gereği korunur.
			color.Yellow("Eski anahtar korundu (--prune verilmedi): %s", ch.OldPath)
			return mut, nil
		}
		if delErr := s.Store.Delete(ch.OldPath); delErr != nil {
			// Yeni yazıldı; eski silinemedi. Yeni kaydedilir, eski state'te
			// kalır ve sonraki çalıştırmada silme tekrar denenir.
			return mut, fmt.Errorf("eski anahtar silinemedi %q: %w", ch.OldPath, delErr)
		}
		mut.del = append(mut.del, ch.OldPath)
		return mut, nil
	}
	return stateMutation{}, fmt.Errorf("bilinmeyen değişiklik türü: %s", ch.Kind)
}

// upload, dosyayı Consul'a yazar. Full modda önce mevcut değerle karşılaştırıp
// aynıysa yüklemeyi atlar (eski tam tarama davranışı).
func (s *Syncer) upload(root, relPath, hash string, opts Options) (stateMutation, error) {
	fullPath := filepath.Join(root, relPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return stateMutation{}, fmt.Errorf("dosya okunamadı: %w", err)
	}

	if opts.OnSensitive != nil {
		opts.OnSensitive(fullPath, string(content))
	}

	if opts.Full {
		value, exists, err := s.Store.Get(relPath)
		if err != nil {
			return stateMutation{}, err
		}
		if exists && file.HashBytes(value) == hash {
			return stateMutation{set: map[string]string{relPath: hash}, skipped: true}, nil
		}
	}

	if err := s.Store.Put(relPath, content); err != nil {
		return stateMutation{}, err
	}
	return stateMutation{set: map[string]string{relPath: hash}}, nil
}

// commitMutation, mutasyonu state'e uygular.
func commitMutation(st *State, mut stateMutation) {
	if st.Files == nil {
		st.Files = make(map[string]string)
	}
	for path, hash := range mut.set {
		st.Files[path] = hash
	}
	for _, path := range mut.del {
		delete(st.Files, path)
	}
}

func changeLabel(ch Change) string {
	if ch.OldPath != "" {
		return ch.OldPath
	}
	return ch.Path
}
