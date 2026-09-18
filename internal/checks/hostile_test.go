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
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
	"github.com/sinanganiz/commitography/internal/server"
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
	repo := openRepository(t)
	fixture := fixtureWithCondition(t, repo, hostileCondition)
	root := filepath.Join(repo.root, "testdata", "fixtures")
	if _, err := os.Stat(filepath.Join(root, fixture)); err != nil {
		fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
	}
	forbidden := machineValues(t, repo)

	app, err := server.NewAppWithAllowedRoots(server.NewManager(server.ManagerOptions{}), []string{root})
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

// TestHostileNamesFixtureIsClassified keeps the checker above from passing
// because the manifest quietly stopped naming the condition.
func TestHostileNamesFixtureIsClassified(t *testing.T) {
	repo := openRepository(t)
	if fixture := fixtureWithCondition(t, repo, hostileCondition); fixture == "" {
		report(t, 19, "no fixture carries the %q condition", hostileCondition)
	}
	if !core.ValidReason(core.ReasonBinaryFileSkipped) {
		report(t, 62, "the enumeration lost %q, which the hostile fixture's binary content needs",
			core.ReasonBinaryFileSkipped)
	}
}
