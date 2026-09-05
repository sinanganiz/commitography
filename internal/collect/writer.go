package collect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sinanganiz/commitography/internal/model"
)

// WriteHistory serializes a History to the given path as JSON. The write is
// atomic: a partially written artifact is never left behind under path.
func WriteHistory(h *model.History, path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating %s: %w", tmp, err)
	}

	// Compact output: history artifacts run to hundreds of megabytes on large
	// repositories and indentation buys nothing a JSON tool cannot add back.
	enc := json.NewEncoder(f)
	if err := enc.Encode(h); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encoding history: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("closing %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming %s to %s: %w", tmp, path, err)
	}
	return nil
}

// ReadHistory loads a previously written History artifact.
func ReadHistory(path string) (*model.History, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var h model.History
	if err := json.NewDecoder(f).Decode(&h); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	if h.SchemaVersion != model.SchemaVersion {
		return nil, fmt.Errorf("history artifact schema version %d is not supported by this version of commitography", h.SchemaVersion)
	}
	return &h, nil
}
