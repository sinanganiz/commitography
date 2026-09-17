package collect

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// Every failure below is an internal error: the history artifact is the
// product's own file in the product's own directory, so a failure to write it
// is not something the operator expressed wrongly. The wrapping context names
// the step rather than the path, because the cause already carries the path and
// a message must not contain a resolved path twice over (ADR-0041 clause 3,
// ADR-0067 clause 5).

// WriteHistory serializes a History to the given path as JSON. The write is
// atomic: a partially written artifact is never left behind under path.
func WriteHistory(h *model.History, path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return core.Internalf(err, "creating the history artifact's directory")
		}
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return core.Internalf(err, "creating the temporary history artifact")
	}

	// Compact output: history artifacts run to hundreds of megabytes on large
	// repositories and indentation buys nothing a JSON tool cannot add back.
	enc := json.NewEncoder(f)
	if err := enc.Encode(h); err != nil {
		f.Close()
		os.Remove(tmp)
		return core.Internalf(err, "encoding the history artifact")
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return core.Internalf(err, "closing the temporary history artifact")
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return core.Internalf(err, "moving the temporary history artifact into place")
	}
	return nil
}

// ReadHistory loads a previously written History artifact.
func ReadHistory(path string) (*model.History, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, core.Internalf(err, "opening the history artifact")
	}
	defer f.Close()

	var h model.History
	if err := json.NewDecoder(f).Decode(&h); err != nil {
		return nil, core.Internalf(err, "decoding the history artifact")
	}
	if h.SchemaVersion != model.SchemaVersion {
		return nil, core.Internalf(nil, "the history artifact's schema version %d is not the %d this build reads",
			h.SchemaVersion, model.SchemaVersion)
	}
	return &h, nil
}
