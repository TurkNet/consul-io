package syncstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// stateVersion, state dosyası şemasının sürümüdür. Şema değişince eskisi
// güvenle tam taramaya düşülür.
const stateVersion = 1

// State, son başarılı senkronizasyonun anlık görüntüsüdür.
// Files: senkronizasyon köküne göreli yol -> içerik sha256 hash'i.
// Yalnızca Consul'a başarıyla yazılan dosyalar burada yer alır.
type State struct {
	Version    int               `json:"version"`
	ConsulAddr string            `json:"consul_addr,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Files      map[string]string `json:"files"`
}

// NewState, boş bir state oluşturur.
func NewState(consulAddr string) *State {
	return &State{
		Version:    stateVersion,
		ConsulAddr: consulAddr,
		Files:      make(map[string]string),
	}
}

// LoadState, state dosyasını okur. Dosya yoksa (nil, nil) döner; böylece çağıran
// taraf "ilk çalıştırma" olarak tam tarama yapar. Bozuk/eski şema yine nil döner.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("state okunamadı: %w", err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		// Bozuk state: panik yerine kontrollü tam taramaya düş.
		return nil, nil
	}
	if st.Version != stateVersion {
		return nil, nil
	}
	if st.Files == nil {
		st.Files = make(map[string]string)
	}
	return &st, nil
}

// SaveState, state'i atomik olarak (temp + rename) diske yazar.
func SaveState(path string, st *State) error {
	if st == nil {
		return nil
	}
	st.Version = stateVersion
	st.UpdatedAt = time.Now()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("state dizini oluşturulamadı: %w", err)
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("state serileştirilemedi: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("geçici state yazılamadı: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("state taşınamadı: %w", err)
	}

	return nil
}
