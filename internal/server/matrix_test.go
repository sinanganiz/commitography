package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sinanganiz/commitography/internal/aggregate"
	"github.com/sinanganiz/commitography/internal/analysis"
	"github.com/sinanganiz/commitography/internal/jobs"
)

// apiCall is one request in the endpoint matrix. Requests carry the session
// cookie unless noSession is set.
type apiCall struct {
	method    string
	path      string
	body      string
	noSession bool
	origin    string
}

func call(t *testing.T, app *App, c apiCall) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if c.body != "" {
		body = strings.NewReader(c.body)
	}
	req := localRequest(c.method, c.path, body)
	if !c.noSession {
		req.AddCookie(app.sessionCookie())
	}
	if c.origin != "" {
		req.Header.Set("Origin", c.origin)
	}
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	return res
}

// expect checks a response's status and, when code is set, its error code.
func expect(t *testing.T, label string, res *httptest.ResponseRecorder, status int, code string) *httptest.ResponseRecorder {
	t.Helper()
	if res.Code != status {
		t.Errorf("%s: status = %d, want %d: %s", label, res.Code, status, res.Body.String())
	}
	if code != "" {
		var body apiError
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.Error.Code != code {
			t.Errorf("%s: body = %s, want error code %q", label, res.Body.String(), code)
		}
	}
	return res
}

func createdID(t *testing.T, res *httptest.ResponseRecorder) string {
	t.Helper()
	var created createJobResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("create response = %s", res.Body.String())
	}
	return created.ID
}

// outcome is what a controlled analysis returns once the test releases it.
type outcome struct {
	result *analysis.Result
	err    error
}

// controlledApp runs each analysis until the test sends its outcome or the job
// is cancelled, and counts how many analyses started.
func controlledApp(t *testing.T) (*App, chan<- outcome, *atomic.Int32) {
	t.Helper()
	outcomes := make(chan outcome)
	var started atomic.Int32
	manager := jobs.New(jobs.Options{
		Runner: func(ctx context.Context, _ analysis.Options, sink analysis.ProgressSink) (*analysis.Result, error) {
			started.Add(1)
			sink(analysis.ProgressEvent{Sequence: 1, Stage: analysis.StageCollecting, Detail: "reading history"})
			select {
			case o := <-outcomes:
				return o.result, o.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	})
	return testApp(t, manager), outcomes, &started
}

// withoutElapsed decodes a status response with its moving elapsed time removed.
func withoutElapsed(t *testing.T, res *httptest.ResponseRecorder) jobStatusResponse {
	t.Helper()
	var status jobStatusResponse
	if err := json.Unmarshal(res.Body.Bytes(), &status); err != nil {
		t.Fatalf("status response = %s", res.Body.String())
	}
	status.ElapsedMilliseconds = 0
	return status
}

// TestAPIEndpointMatrix walks every documented endpoint through its success and
// failure responses, in the order a job's lifecycle reaches them.
func TestAPIEndpointMatrix(t *testing.T) {
	app, outcomes, started := controlledApp(t)
	valid := jobBody(t, "")
	evil := "http://evil.example"

	// GET /api/v1/capabilities
	expect(t, "capabilities", call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/capabilities", noSession: true}), http.StatusOK, "")
	expect(t, "capabilities POST", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/capabilities"}), http.StatusMethodNotAllowed, "method_not_allowed")

	// POST /api/v1/jobs failures
	expect(t, "create without session", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid, noSession: true}), http.StatusUnauthorized, "invalid_session")
	expect(t, "create cross-origin", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid, origin: evil}), http.StatusForbidden, "invalid_origin")
	expect(t, "create malformed JSON", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: "{"}), http.StatusBadRequest, "invalid_json")
	expect(t, "create with two objects", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid + "{}"}), http.StatusBadRequest, "invalid_json")
	expect(t, "create with outputDir", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: jobBody(t, `,"outputDir":"/tmp/out"`)}), http.StatusBadRequest, "invalid_json")
	expect(t, "create with configPath", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: jobBody(t, `,"configPath":"/tmp/c.yml"`)}), http.StatusBadRequest, "invalid_json")
	expect(t, "create without path", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: `{"repoPath":"  "}`}), http.StatusBadRequest, "invalid_repository_path")
	oversized := `{"repoPath":"` + strings.Repeat("a", maxRequestBody) + `"}`
	expect(t, "create oversized", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: oversized}), http.StatusRequestEntityTooLarge, "request_too_large")
	outside, err := json.Marshal(map[string]string{"repoPath": t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, "create outside root", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: string(outside)}), http.StatusForbidden, "path_not_allowed")
	if n := started.Load(); n != 0 {
		t.Fatalf("rejected requests started %d analyses", n)
	}

	// POST /api/v1/jobs success, then a second start while it is active.
	first := createdID(t, expect(t, "create", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid}), http.StatusAccepted, ""))
	waitForStatus(t, app, first, jobs.StatusRunning)
	expect(t, "second create while active", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid}), http.StatusConflict, "active_job")
	if n := started.Load(); n != 1 {
		t.Fatalf("%d analyses started, want exactly one while a job is active", n)
	}

	// GET /api/v1/jobs/{id}: repeated polling of an unchanged job is identical.
	statusPath := "/api/v1/jobs/" + first
	poll := withoutElapsed(t, expect(t, "status", call(t, app, apiCall{method: http.MethodGet, path: statusPath}), http.StatusOK, ""))
	for i := 0; i < 5; i++ {
		if again := withoutElapsed(t, call(t, app, apiCall{method: http.MethodGet, path: statusPath})); !reflect.DeepEqual(poll, again) {
			t.Fatalf("repeated poll changed the job:\nfirst %+v\nlater %+v", poll, again)
		}
	}
	expect(t, "status without session", call(t, app, apiCall{method: http.MethodGet, path: statusPath, noSession: true}), http.StatusUnauthorized, "invalid_session")
	expect(t, "status of unknown job", call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs/unknown"}), http.StatusNotFound, "not_found")
	expect(t, "status PUT", call(t, app, apiCall{method: http.MethodPut, path: statusPath}), http.StatusMethodNotAllowed, "method_not_allowed")

	// Report and delete are refused while the job is active.
	expect(t, "report while running", call(t, app, apiCall{method: http.MethodGet, path: statusPath + "/report"}), http.StatusConflict, "report_not_ready")
	expect(t, "delete while running", call(t, app, apiCall{method: http.MethodDelete, path: statusPath}), http.StatusConflict, "invalid_job_state")

	// POST /api/v1/jobs/{id}/cancel
	expect(t, "cancel cross-origin", call(t, app, apiCall{method: http.MethodPost, path: statusPath + "/cancel", origin: evil}), http.StatusForbidden, "invalid_origin")
	expect(t, "cancel without session", call(t, app, apiCall{method: http.MethodPost, path: statusPath + "/cancel", noSession: true}), http.StatusUnauthorized, "invalid_session")
	expect(t, "cancel GET", call(t, app, apiCall{method: http.MethodGet, path: statusPath + "/cancel"}), http.StatusMethodNotAllowed, "method_not_allowed")
	expect(t, "cancel", call(t, app, apiCall{method: http.MethodPost, path: statusPath + "/cancel"}), http.StatusAccepted, "")
	waitForStatus(t, app, first, jobs.StatusCancelled)
	expect(t, "cancel again", call(t, app, apiCall{method: http.MethodPost, path: statusPath + "/cancel"}), http.StatusOK, "")
	expect(t, "cancel unknown job", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs/unknown/cancel"}), http.StatusNotFound, "not_found")
	expect(t, "report of cancelled job", call(t, app, apiCall{method: http.MethodGet, path: statusPath + "/report"}), http.StatusConflict, "report_not_ready")

	// A succeeded job has a report and can no longer be cancelled.
	second := createdID(t, expect(t, "create second", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid}), http.StatusAccepted, ""))
	waitForStatus(t, app, second, jobs.StatusRunning)
	outcomes <- outcome{result: &analysis.Result{Report: &aggregate.Report{SchemaVersion: aggregate.SchemaVersion}}}
	waitForStatus(t, app, second, jobs.StatusSucceeded)
	secondPath := "/api/v1/jobs/" + second
	expect(t, "report", call(t, app, apiCall{method: http.MethodGet, path: secondPath + "/report"}), http.StatusOK, "")
	expect(t, "report POST", call(t, app, apiCall{method: http.MethodPost, path: secondPath + "/report"}), http.StatusMethodNotAllowed, "method_not_allowed")
	expect(t, "report without session", call(t, app, apiCall{method: http.MethodGet, path: secondPath + "/report", noSession: true}), http.StatusUnauthorized, "invalid_session")
	expect(t, "report of unknown job", call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs/unknown/report"}), http.StatusNotFound, "not_found")
	expect(t, "cancel succeeded job", call(t, app, apiCall{method: http.MethodPost, path: secondPath + "/cancel"}), http.StatusConflict, "invalid_job_state")
	terminal := call(t, app, apiCall{method: http.MethodGet, path: secondPath}).Body.String()
	for i := 0; i < 5; i++ {
		if again := call(t, app, apiCall{method: http.MethodGet, path: secondPath}).Body.String(); again != terminal {
			t.Fatalf("repeated poll of a finished job changed:\nfirst %s\nlater %s", terminal, again)
		}
	}

	// A failed job reports its failure code and has no report.
	third := createdID(t, expect(t, "create third", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid}), http.StatusAccepted, ""))
	waitForStatus(t, app, third, jobs.StatusRunning)
	outcomes <- outcome{err: errors.New("git log failed: fatal: bad object")}
	failed := waitForStatus(t, app, third, jobs.StatusFailed)
	if failed.Error == nil || failed.Error.Code != "analysis_failed" || strings.Contains(failed.Error.Message, "bad object") {
		t.Errorf("failed job error = %+v, want analysis_failed without Git output", failed.Error)
	}
	expect(t, "report of failed job", call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs/" + third + "/report"}), http.StatusConflict, "report_not_ready")

	// A stale job reports the change and has no report.
	fourth := createdID(t, expect(t, "create fourth", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid}), http.StatusAccepted, ""))
	waitForStatus(t, app, fourth, jobs.StatusRunning)
	outcomes <- outcome{result: &analysis.Result{Report: &aggregate.Report{}, Stale: true, StaleReason: analysis.StaleHeadChanged}}
	stale := waitForStatus(t, app, fourth, jobs.StatusStale)
	if stale.Error == nil || stale.Error.Code != "repository_changed" {
		t.Errorf("stale job error = %+v, want repository_changed", stale.Error)
	}
	fourthPath := "/api/v1/jobs/" + fourth
	expect(t, "report of stale job", call(t, app, apiCall{method: http.MethodGet, path: fourthPath + "/report"}), http.StatusConflict, "report_not_ready")

	// GET /api/v1/jobs lists newest first.
	var list jobsResponse
	listed := expect(t, "list", call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs"}), http.StatusOK, "")
	if err := json.Unmarshal(listed.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, job := range list.Jobs {
		ids = append(ids, job.ID)
	}
	if want := []string{fourth, third, second, first}; !reflect.DeepEqual(ids, want) {
		t.Errorf("list = %v, want newest first %v", ids, want)
	}
	expect(t, "list without session", call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs", noSession: true}), http.StatusUnauthorized, "invalid_session")
	expect(t, "list PUT", call(t, app, apiCall{method: http.MethodPut, path: "/api/v1/jobs"}), http.StatusMethodNotAllowed, "method_not_allowed")

	// DELETE /api/v1/jobs/{id}
	expect(t, "delete cross-origin", call(t, app, apiCall{method: http.MethodDelete, path: fourthPath, origin: evil}), http.StatusForbidden, "invalid_origin")
	expect(t, "delete without session", call(t, app, apiCall{method: http.MethodDelete, path: fourthPath, noSession: true}), http.StatusUnauthorized, "invalid_session")
	expect(t, "delete", call(t, app, apiCall{method: http.MethodDelete, path: fourthPath}), http.StatusNoContent, "")
	expect(t, "status after delete", call(t, app, apiCall{method: http.MethodGet, path: fourthPath}), http.StatusNotFound, "not_found")
	expect(t, "delete again", call(t, app, apiCall{method: http.MethodDelete, path: fourthPath}), http.StatusNotFound, "not_found")
}

func TestAPIRejectsAFolderThatIsNotARepository(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "not-a-repository")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	app, err := NewAppWithAllowedRoots(jobs.New(jobs.Options{}), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{"repoPath": folder})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, "create for a plain folder", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: string(body)}), http.StatusBadRequest, "invalid_repository")
	missing, err := json.Marshal(map[string]string{"repoPath": filepath.Join(root, "missing")})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, "create for a missing path", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: string(missing)}), http.StatusBadRequest, "invalid_repository_path")
}

func TestAPIHistoryEvictsTheOldestFinishedJob(t *testing.T) {
	manager := jobs.New(jobs.Options{
		Limit: 2,
		Runner: func(context.Context, analysis.Options, analysis.ProgressSink) (*analysis.Result, error) {
			return &analysis.Result{Report: &aggregate.Report{}}, nil
		},
	})
	app := testApp(t, manager)
	var ids []string
	for i := 0; i < 3; i++ {
		id := createdID(t, expect(t, "create", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: jobBody(t, "")}), http.StatusAccepted, ""))
		waitForStatus(t, app, id, jobs.StatusSucceeded)
		ids = append(ids, id)
	}
	var list jobsResponse
	if err := json.Unmarshal(call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs"}).Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Jobs) != 2 || list.Jobs[0].ID != ids[2] || list.Jobs[1].ID != ids[1] {
		t.Fatalf("history = %+v, want the two newest jobs %v", list.Jobs, ids[1:])
	}
	expect(t, "evicted job", call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs/" + ids[0]}), http.StatusNotFound, "not_found")
}
