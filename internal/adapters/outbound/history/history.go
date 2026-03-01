package history

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/abdidvp/openkraft/internal/domain"
)

const historyFile = ".openkraft/history/scores.json"

// FileHistory implements domain.ScoreHistory using JSON file storage.
type FileHistory struct{}

func New() *FileHistory {
	return &FileHistory{}
}

func (h *FileHistory) Save(projectPath string, entry domain.ScoreEntry) error {
	entries, err := h.Load(projectPath)
	if err != nil {
		return err
	}

	entries = append(entries, entry)

	fp := filepath.Join(projectPath, historyFile)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fp, data, 0644)
}

func (h *FileHistory) Load(projectPath string) ([]domain.ScoreEntry, error) {
	fp := filepath.Join(projectPath, historyFile)

	data, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entries []domain.ScoreEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}
