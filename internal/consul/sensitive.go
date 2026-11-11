package consul

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
)

// problemsMu, eşzamanlı yüklemelerde problems.txt'ye yarışsız yazımı sağlar.
var problemsMu sync.Mutex

// CheckForSensitiveData, içeriğinde Vault referansı olmadan Password/Token
// geçen dosyaları uyarır ve problems.txt'ye yazar. Mevcut davranış korunur.
func CheckForSensitiveData(filePath string, content string) {
	sensitiveKeys := []string{"Password", "Token"}
	var problems []string

	for _, key := range sensitiveKeys {
		if strings.Contains(content, key) && !strings.Contains(content, `{{ with secret "kv/`) {
			problems = append(problems, fmt.Sprintf("Warning: The configuration contains a sensitive key '%s' that is not stored in Vault in file %s.", key, filePath))
		}
	}

	if len(problems) == 0 {
		return
	}

	problemsMu.Lock()
	defer problemsMu.Unlock()

	f, err := os.OpenFile("problems.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		color.Red("Error opening problems.txt: %v", err)
		return
	}
	defer f.Close()

	for _, problem := range problems {
		if _, err := f.WriteString(problem + "\n"); err != nil {
			color.Red("Error writing to problems.txt: %v", err)
		}
		color.Yellow(problem)
	}
}
