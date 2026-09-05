package collect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/model"
)

func syntheticHistory(commits int) *model.History {
	zones := []*time.Location{
		time.UTC,
		time.FixedZone("+0300", 3*3600),
		time.FixedZone("-0500", -5*3600),
	}
	h := &model.History{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
		ToolVersion:   "test",
		Repository: model.RepositoryInfo{
			Path: "/repo", Name: "repo", HeadCommit: "abc", DefaultBranch: "main",
		},
	}
	base := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < commits; i++ {
		zone := zones[i%len(zones)]
		when := base.Add(time.Duration(i) * time.Hour).In(zone)
		_, offset := when.Zone()
		h.Commits = append(h.Commits, model.Commit{
			Hash:                     fmt.Sprintf("%040x", i),
			AuthorName:               "Ada Lovelace",
			AuthorEmail:              "ada@example.com",
			AuthorDate:               when,
			AuthorTZOffsetMinutes:    offset / 60,
			CommitterDate:            when,
			CommitterTZOffsetMinutes: offset / 60,
			Parents:                  []string{fmt.Sprintf("%040x", i-1)},
			Subject:                  fmt.Sprintf("feat: change %d", i),
			Files: []model.FileChange{
				{Path: "src/main.go", Added: i, Deleted: i / 2},
			},
		})
	}
	h.Commits[0].Parents = nil
	return h
}

func TestWriteReadHistoryRoundTrip(t *testing.T) {
	h := syntheticHistory(10000)
	path := filepath.Join(t.TempDir(), "commits.json")

	if err := WriteHistory(h, path); err != nil {
		t.Fatalf("WriteHistory: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("temporary file was left behind")
	}

	got, err := ReadHistory(path)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if len(got.Commits) != len(h.Commits) {
		t.Fatalf("read %d commits, wrote %d", len(got.Commits), len(h.Commits))
	}
	for i := range h.Commits {
		want, have := h.Commits[i], got.Commits[i]
		if !want.AuthorDate.Equal(have.AuthorDate) {
			t.Fatalf("commit %d: author date %v != %v", i, have.AuthorDate, want.AuthorDate)
		}
		_, wantOffset := want.AuthorDate.Zone()
		_, haveOffset := have.AuthorDate.Zone()
		if wantOffset != haveOffset {
			t.Fatalf("commit %d: zone offset %d != %d (timezone was normalized away)", i, haveOffset, wantOffset)
		}
		if have.AuthorTZOffsetMinutes != want.AuthorTZOffsetMinutes {
			t.Fatalf("commit %d: offset minutes %d != %d", i, have.AuthorTZOffsetMinutes, want.AuthorTZOffsetMinutes)
		}
		if have.Subject != want.Subject || have.Hash != want.Hash {
			t.Fatalf("commit %d: identity fields differ", i)
		}
		if len(have.Files) != len(want.Files) || have.Files[0] != want.Files[0] {
			t.Fatalf("commit %d: file changes differ", i)
		}
	}
}

func TestWriteHistoryIsCompact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commits.json")
	if err := WriteHistory(syntheticHistory(3), path); err != nil {
		t.Fatalf("WriteHistory: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var compact []byte
	{
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}
		compact, _ = json.Marshal(v)
	}
	if len(data) > len(compact)+2 { // allow the trailing newline
		t.Errorf("output appears indented: %d bytes vs %d compact", len(data), len(compact))
	}
}

func TestReadHistoryRejectsForeignSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commits.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":999,"commits":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadHistory(path)
	if err == nil {
		t.Fatal("expected an error for an unsupported schema version")
	}
	want := "history artifact schema version 999 is not supported by this version of commitography"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
