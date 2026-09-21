package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

// hostileCondition is the fixture condition of ADR-0019 clause 1: names that
// are legal on every supported platform and awkward to handle. Which fixture
// carries it is read from the manifest rather than named here, because the
// manifest is what keeps coverage visible (WP-0004).
const hostileCondition = "hostile-names"

// TestHostileNamesProduceNoPanicAndNoLeak enforces ADR-0041 clause 6 and
// ADR-0045 together: a repository is untrusted input, and no path reachable
// from its content may panic.
//
// The assertion is the whole analysis path, not a parser in isolation, because
// a panic from attacker-controlled content is a property of the path and not of
// any one function. Both destinations are checked as well, so a hostile name
// cannot reach an artifact even if it survives the analysis.
func TestHostileNamesProduceNoPanicAndNoLeak(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fixture := fixtureWithCondition(t, repo, hostileCondition)
	root := filepath.Join(repo.root, "testdata", "fixtures")
	dir := filepath.Join(root, fixture)
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
	}
	forbidden := machineValues(t, repo)

	// A panic anywhere below fails the test rather than the process, so the
	// failure names the condition instead of only printing a stack.
	defer func() {
		if value := recover(); value != nil {
			fatal(t, 41, "analysing the %s fixture %s panicked: %v; no path reachable from repository "+
				"content may panic", hostileCondition, fixture, value)
		}
	}()

	result, err := newAnalyzer().Run(context.Background(),
		pipeline.Options{RepoPath: dir, OperatorSupplied: true, PerAuthor: true}, nil)
	if err != nil {
		fatal(t, 41, "analysing the %s fixture %s failed: %v", hostileCondition, fixture, err)
	}

	// The report is the artifact, so it carries nothing that names the machine.
	out := filepath.Join(t.TempDir(), render.ReportFile)
	if err := render.WriteReportJSON(result.Report, out); err != nil {
		fatal(t, 41, "writing the report of fixture %s: %v", fixture, err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		fatal(t, 41, "reading the report of fixture %s: %v", fixture, err)
	}
	scanArtifact(t, "the report of the "+hostileCondition+" fixture", string(data), forbidden)

	// Hostile names must survive the round trip as data rather than as
	// structure: a report that will not decode would have carried a name into
	// the document's syntax.
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		report(t, 45, "the report of the %s fixture is not valid JSON, so a name reached the document's "+
			"syntax: %v", hostileCondition, err)
	}
}

// TestHostileNamesThroughTheServerPath runs the same fixture through the server
// and checks every response, because the API is the other destination and its
// rule is the stricter one (ADR-0067 clause 2).
func TestHostileNamesThroughTheServerPath(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fixture := fixtureWithCondition(t, repo, hostileCondition)
	root := filepath.Join(repo.root, "testdata", "fixtures")
	if _, err := os.Stat(filepath.Join(root, fixture)); err != nil {
		fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
	}
	forbidden := machineValues(t, repo)

	app, err := newApp([]string{root})
	if err != nil {
		fatal(t, 41, "building the local application: %v", err)
	}
	handler := app.Handler()

	cookie := ""
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Host = "127.0.0.1:8080"
		if cookie != "" {
			request.Header.Set("Cookie", cookie)
		}
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if set := response.Header().Get("Set-Cookie"); set != "" {
			cookie, _, _ = strings.Cut(set, ";")
		}
		scanArtifact(t, fmt.Sprintf("the response to %s %s for the %s fixture", method, path, hostileCondition),
			response.Body.String(), forbidden)
		return response
	}

	call(http.MethodGet, "/api/v1/capabilities", "")
	body, err := json.Marshal(map[string]any{
		"repoPath": filepath.Join(root, fixture),
		"options":  map[string]any{"perAuthor": true},
	})
	if err != nil {
		fatal(t, 41, "encoding a request: %v", err)
	}
	created := call(http.MethodPost, "/api/v1/jobs", string(body))
	if created.Code != http.StatusAccepted {
		fatal(t, 41, "starting an analysis of the %s fixture: %d %s",
			hostileCondition, created.Code, created.Body.String())
	}
	var start struct{ ID string }
	if err := json.Unmarshal(created.Body.Bytes(), &start); err != nil || start.ID == "" {
		fatal(t, 41, "reading the created job's identifier: %v", err)
	}

	// Bounded by a count rather than a deadline, so the checker reads no clock
	// (ADR-0042).
	const (
		pollInterval = 50 * time.Millisecond
		maxPolls     = 1200
	)
	statusPath := "/api/v1/jobs/" + start.ID
	for poll := 0; ; poll++ {
		response := call(http.MethodGet, statusPath, "")
		var status struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
			fatal(t, 41, "reading the job status: %v", err)
		}
		if status.Status != "queued" && status.Status != "running" {
			// A panic in the worker now surfaces as a failed job rather than a
			// crash (ADR-0041 clause 6), so a non-success is a defect here even
			// though the process survived it.
			if status.Status != "succeeded" {
				fatal(t, 41, "analysing the %s fixture ended %s: %s",
					hostileCondition, status.Status, response.Body.String())
			}
			break
		}
		if poll == maxPolls {
			fatal(t, 41, "analysing the %s fixture did not finish within %d polls", hostileCondition, maxPolls)
		}
		time.Sleep(pollInterval)
	}
	call(http.MethodGet, statusPath+"/report", "")
}

// fixtureWithCondition returns the fixture the manifest marks with a condition.
func fixtureWithCondition(t *testing.T, repo repository, condition string) string {
	t.Helper()
	var found []string
	for fixture, conditions := range loadFixtureConditions(t, repo) {
		for _, c := range conditions {
			if c == condition {
				found = append(found, fixture)
			}
		}
	}
	if len(found) == 0 {
		fatal(t, 64, "%s names no fixture with the condition %q, so the checker has nothing to run",
			fixtureConditions, condition)
	}
	return found[0]
}

// TestHostileNamesRecordIntegrity enforces the record half of ADR-0065
// clause 2 on the fixture the manifest marks hostile: the collect stage's
// reading of the history agrees, commit for commit and path for path, with
// what git itself reports. A record that was split would add a commit or a
// file, one that was merged would lose one, and one that was dropped would
// lose both.
//
// The comparison is against a second, independent reading of the same
// NUL-delimited stream rather than against a recorded expectation, so it
// stays true when the fixture changes.
func TestHostileNamesRecordIntegrity(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fixture := fixtureWithCondition(t, repo, hostileCondition)
	dir := filepath.Join(repo.root, "testdata", "fixtures", fixture)
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
	}

	history, err := collect.New(core.FixedClock(fixedClock()), core.SystemFilesystem()).
		Collect(collect.Options{RepoPath: dir, Context: context.Background()})
	if err != nil {
		fatal(t, 65, "reading the %s fixture: %v", hostileCondition, err)
	}

	want := gitFileEntries(t, dir)
	if len(history.Commits) != len(want) {
		report(t, 65, "the collect stage read %d commits from the %s fixture and git reports %d",
			len(history.Commits), hostileCondition, len(want))
	}
	if len(want) == 0 {
		fatal(t, 64, "git reports no commits in the %s fixture, so the comparison cannot fail", hostileCondition)
	}
	for _, commit := range history.Commits {
		paths, ok := want[commit.Hash]
		if !ok {
			report(t, 65, "the collect stage produced commit %.12s, which git does not report", commit.Hash)
			continue
		}
		if len(commit.Files) != len(paths) {
			report(t, 65, "commit %.12s came back with %d file entries and git reports %d; a record was "+
				"split, merged or dropped", commit.Hash, len(commit.Files), len(paths))
			continue
		}
		for i, file := range commit.Files {
			if file.Path != paths[i] {
				report(t, 65, "commit %.12s file %d came back as %q and git reports %q",
					commit.Hash, i, file.Path, paths[i])
			}
		}
	}
}

// gitFileEntries reads the same history a second time, straight from git, and
// returns each commit's file paths in order. It applies the framing rules of
// `git log -z --numstat` and nothing else: records are NUL-delimited, a
// header is an object name followed by the field separator, an empty record
// separates commits, and a path is whatever follows the second tab. A rename
// has an empty path there, and its previous path and its path follow as the
// next two records, which are taken by position; the path after the rename is
// the entry's path, as the collect stage records it.
func gitFileEntries(t *testing.T, dir string) map[string][]string {
	t.Helper()
	entries := map[string][]string{}
	current := ""
	owed := 0
	add := func(entry string) {
		path := pathOfNumstatEntry(entry)
		if path == "" && strings.Count(entry, "\t") == 2 {
			owed = 2
			return
		}
		entries[current] = append(entries[current], path)
	}
	err := git.Scan(context.Background(), git.At(dir, "log", "-z", "--numstat",
		"--all", "--date-order", "--pretty=format:%H\x1f").Pathspecs(), func(record string) error {
		if owed > 0 {
			owed--
			if owed == 0 {
				entries[current] = append(entries[current], record)
			}
			return nil
		}
		head, first, hasFiles := strings.Cut(record, "\n")
		switch {
		case record == "":
		case strings.HasSuffix(head, "\x1f") && len(head) == 41:
			current = strings.TrimSuffix(head, "\x1f")
			entries[current] = nil
			if hasFiles {
				add(first)
			}
		case current != "":
			add(record)
		}
		return nil
	})
	if err != nil {
		fatal(t, 65, "reading the fixture's history from git: %v", err)
	}
	return entries
}

// craftedHeaderPath is a path built to be a well-formed record header of the
// collect stage's own format: an object name, the field separator, and every
// field a header carries. A reader that asked what a record looks like would
// start a commit in the middle of the rename it belongs to.
func craftedHeaderPath() string {
	return strings.Repeat("f", 40) + "\x1fAda\x1fada@example.com\x1fada@example.com" +
		"\x1f2010-04-10T16:57:36Z\x1f2010-04-10T16:57:36Z\x1f\x1fnot a commit"
}

// renameRepository builds a repository of two commits: the first adds a file
// at the crafted path, and the second moves it, unchanged, to src/moved.go.
// The trees are written directly, so no filesystem has to hold the name, and
// the dates are fixed so that nothing here reads a clock (ADR-0042 clause 4).
func renameRepository(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	run := func(spec git.Spec) string {
		t.Helper()
		out, err := git.Output(ctx, spec.WithEnv(
			"GIT_AUTHOR_NAME=Fixture Builder", "GIT_AUTHOR_EMAIL=fixtures@example.com",
			"GIT_AUTHOR_DATE=1700000000 +0000", "GIT_COMMITTER_NAME=Fixture Builder",
			"GIT_COMMITTER_EMAIL=fixtures@example.com", "GIT_COMMITTER_DATE=1700000000 +0000"))
		if err != nil {
			fatal(t, 64, "building the rename repository: %v", err)
		}
		return out
	}
	blob := func(content string) string {
		return run(git.At(dir, "hash-object", "-w", "--stdin").WithStdin(strings.NewReader(content)))
	}
	tree := func(entries ...string) string {
		return run(git.At(dir, "mktree", "-z").WithStdin(strings.NewReader(strings.Join(entries, "\x00") + "\x00")))
	}

	run(git.At("", "init", "-q", dir))
	moved := blob(strings.Repeat("a line that moves without change\n", 20))
	kept := blob("package keep\n")
	before := tree("100644 blob "+kept+"\tkeep.go", "100644 blob "+moved+"\t"+craftedHeaderPath())
	after := tree("100644 blob "+kept+"\tkeep.go", "040000 tree "+tree("100644 blob "+moved+"\tmoved.go")+"\tsrc")
	first := run(git.At(dir, "commit-tree", before, "-m", "feat: add a file at a crafted path"))
	second := run(git.At(dir, "commit-tree", after, "-p", first, "-m", "refactor: move it"))
	run(git.At(dir, "update-ref", "HEAD", second))
	return dir
}

// TestHostileRenameSourceParsesAsAPath is WP-0012 clause 10a on a real
// history: a rename whose source is a crafted record header comes back as
// that path's rename, and no commit starts in the middle of it.
func TestHostileRenameSourceParsesAsAPath(t *testing.T) {
	t.Parallel()
	dir := renameRepository(t)
	var warnings []string
	history, err := newCollector().Collect(collect.Options{
		RepoPath: dir, Context: context.Background(),
		OnWarning: func(message string) { warnings = append(warnings, message) },
	})
	if err != nil {
		fatal(t, 45, "reading a history whose rename source is a crafted header: %v", err)
	}
	if len(history.Commits) != 2 || len(warnings) != 0 {
		report(t, 45, "the history came back as %d commits with the warnings %q, want its 2 and none; the "+
			"crafted path was read as a record", len(history.Commits), warnings)
	}
	renamed := false
	for _, c := range history.Commits {
		for _, f := range c.Files {
			if f.PreviousPath == craftedHeaderPath() && f.Path == "src/moved.go" {
				renamed = true
				if f.Added != 0 || f.Deleted != 0 || c.EffectiveLines != 0 {
					report(t, 45, "the move without change counts %d added, %d removed and %d effective lines, "+
						"want none (docs/metrics.md section 1)", f.Added, f.Deleted, c.EffectiveLines)
				}
			}
		}
	}
	if !renamed {
		report(t, 45, "no record carries the rename from the crafted path to src/moved.go: %+v", history.Commits)
	}
}

// pathOfNumstatEntry is everything after the second tab of a file entry.
func pathOfNumstatEntry(entry string) string {
	parts := strings.SplitN(entry, "\t", 3)
	if len(parts) != 3 {
		return entry
	}
	return parts[2]
}

// fixedClock is the instant the collect stage stamps a history with here.
// Nothing in the comparison reads it, but the stage requires a clock and no
// checker reads the process's (ADR-0042 clause 4).
func fixedClock() time.Time {
	return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
}

// TestHostileNamesFixtureIsClassified keeps the checker above from passing
// because the manifest quietly stopped naming the condition.
func TestHostileNamesFixtureIsClassified(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	if fixture := fixtureWithCondition(t, repo, hostileCondition); fixture == "" {
		report(t, 19, "no fixture carries the %q condition", hostileCondition)
	}
	if !core.ValidReason(core.ReasonBinaryFileSkipped) {
		report(t, 62, "the enumeration lost %q, which the hostile fixture's binary content needs",
			core.ReasonBinaryFileSkipped)
	}
}
