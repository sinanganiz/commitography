package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/aggregate"
	"github.com/sinanganiz/commitography/internal/analysis"
	"github.com/sinanganiz/commitography/internal/jobs"
)

func TestAPICapabilitiesAndJobList(t *testing.T) {
	app := NewApp(jobs.New(jobs.Options{}))
	handler := app.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("capabilities status = %d, want 200", res.Code)
	}
	var capabilities capabilitiesResponse
	if err := json.Unmarshal(res.Body.Bytes(), &capabilities); err != nil {
		t.Fatal(err)
	}
	if capabilities.APIVersion != "v1" || capabilities.ReportSchemaVersion != 1 || capabilities.ActiveJobLimit != 1 {
		t.Fatalf("capabilities = %+v", capabilities)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.AddCookie(app.sessionCookie())
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("jobs status = %d, want 200", res.Code)
	}
	var jobsList jobsResponse
	if err := json.Unmarshal(res.Body.Bytes(), &jobsList); err != nil {
		t.Fatal(err)
	}
	if len(jobsList.Jobs) != 0 {
		t.Fatalf("jobs = %+v, want empty", jobsList.Jobs)
	}
}

func TestAPILifecycleRoutesAreVersioned(t *testing.T) {
	app := NewApp(jobs.New(jobs.Options{}))
	handler := app.Handler()
	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodGet, path: "/api/v1/jobs/example", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/v2/jobs", status: http.StatusNotFound},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.AddCookie(app.sessionCookie())
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tc.status {
			t.Errorf("%s %s status = %d, want %d", tc.method, tc.path, res.Code, tc.status)
		}
	}
}

func TestAPICreatesJobAndRejectsUnknownFields(t *testing.T) {
	manager := jobs.New(jobs.Options{
		Runner: func(context.Context, analysis.Options, analysis.ProgressSink) (*analysis.Result, error) {
			return &analysis.Result{}, nil
		},
	})
	app := testApp(t, manager)
	handler := app.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(jobBody(t, `,"options":{"noBlame":true}`)))
	req.AddCookie(app.sessionCookie())
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want 202: %s", res.Code, res.Body.String())
	}
	var created createJobResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Status != jobs.StatusQueued {
		t.Fatalf("created = %+v", created)
	}

	// Wait for the fake worker to release the active slot before the next case.
	for i := 0; i < 100; i++ {
		if current, err := manager.Get(created.ID); err == nil && current.Status == jobs.StatusSucceeded {
			break
		}
		time.Sleep(time.Millisecond)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(jobBody(t, `,"outputDir":"/tmp/out"`)))
	req.AddCookie(app.sessionCookie())
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400", res.Code)
	}
}

func TestAPILifecycleServesStatusReportAndDelete(t *testing.T) {
	manager := jobs.New(jobs.Options{
		Runner: func(context.Context, analysis.Options, analysis.ProgressSink) (*analysis.Result, error) {
			return &analysis.Result{Report: &aggregate.Report{}}, nil
		},
	})
	app := testApp(t, manager)
	handler := app.Handler()
	id := createAPIJob(t, app)

	status := waitForStatus(t, app, id, jobs.StatusSucceeded)
	if status.Status != jobs.StatusSucceeded {
		t.Fatalf("status = %+v", status)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+id+"/report", nil)
	req.AddCookie(app.sessionCookie())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("report status = %d, want 200", res.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/"+id, nil)
	req.AddCookie(app.sessionCookie())
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", res.Code)
	}
}

func TestAPICreatePassesOptionsToAnalysis(t *testing.T) {
	received := make(chan analysis.Options, 1)
	manager := jobs.New(jobs.Options{
		Runner: func(_ context.Context, opts analysis.Options, _ analysis.ProgressSink) (*analysis.Result, error) {
			received <- opts
			return &analysis.Result{}, nil
		},
	})
	app := testApp(t, manager)
	body := jobBody(t, `,"options":{"noBlame":true,"perAuthor":true,"anonymize":true,"countMerges":true,"since":"2025-01-01","until":"2025-12-31"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(body))
	req.AddCookie(app.sessionCookie())
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("create status = %d: %s", res.Code, res.Body.String())
	}

	var opts analysis.Options
	select {
	case opts = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}
	if !opts.NoBlame || !opts.PerAuthor || !opts.Anonymize || opts.Since != "2025-01-01" || opts.Until != "2025-12-31" {
		t.Errorf("analysis options = %+v", opts)
	}
	// A requested merge count must override the repository config, exactly as
	// the CLI --count-merges flag does.
	if !opts.CountMerges || !opts.CountMergesSet {
		t.Errorf("countMerges reached analysis as CountMerges=%v CountMergesSet=%v, want both true", opts.CountMerges, opts.CountMergesSet)
	}
}

func TestAPIStatusUsesContractProgressFields(t *testing.T) {
	fraction := 0.42
	manager := jobs.New(jobs.Options{
		Runner: func(_ context.Context, _ analysis.Options, sink analysis.ProgressSink) (*analysis.Result, error) {
			sink(analysis.ProgressEvent{
				Sequence:  1,
				Stage:     analysis.StageCollecting,
				Detail:    "4200 of 10000 commits",
				Fraction:  &fraction,
				Current:   4200,
				Total:     10000,
				Estimated: true,
			})
			return &analysis.Result{Report: &aggregate.Report{}}, nil
		},
	})
	app := testApp(t, manager)
	id := createAPIJob(t, app)
	waitForStatus(t, app, id, jobs.StatusSucceeded)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+id, nil)
	req.AddCookie(app.sessionCookie())
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	// Go's decoder matches field names case-insensitively, so the contract is
	// checked against the raw keys rather than a round-trip through the struct.
	var raw struct {
		Progress map[string]json.RawMessage `json:"progress"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"sequence", "stage", "detail", "fraction", "current", "total", "estimated"} {
		if _, ok := raw.Progress[key]; !ok {
			t.Errorf("progress is missing %q: %s", key, res.Body.String())
		}
	}
	if len(raw.Progress) != 7 {
		t.Errorf("progress has %d keys, want 7: %s", len(raw.Progress), res.Body.String())
	}
}

func TestAPICancelTransitionsJob(t *testing.T) {
	started := make(chan struct{}, 1)
	manager := jobs.New(jobs.Options{
		Runner: func(ctx context.Context, _ analysis.Options, _ analysis.ProgressSink) (*analysis.Result, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	app := testApp(t, manager)
	handler := app.Handler()
	id := createAPIJob(t, app)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/"+id+"/cancel", nil)
	req.AddCookie(app.sessionCookie())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("cancel status = %d, want 202", res.Code)
	}
	waitForStatus(t, app, id, jobs.StatusCancelled)
}

func TestSessionBootstrapAndOriginProtection(t *testing.T) {
	app := testApp(t, jobs.New(jobs.Options{
		Runner: func(context.Context, analysis.Options, analysis.ProgressSink) (*analysis.Result, error) {
			return &analysis.Result{}, nil
		},
	}))
	handler := app.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(jobBody(t, "")))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status = %d, want 401", res.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Header().Get("Set-Cookie"), "SameSite=Strict") {
		t.Fatalf("bootstrap response = %d, cookie = %q", res.Code, res.Header().Get("Set-Cookie"))
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(jobBody(t, "")))
	req.AddCookie(app.sessionCookie())
	req.Header.Set("Origin", "http://evil.example")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want 403", res.Code)
	}
}

func createAPIJob(t *testing.T, app *App) string {
	t.Helper()
	handler := app.Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(jobBody(t, "")))
	req.AddCookie(app.sessionCookie())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("create status = %d: %s", res.Code, res.Body.String())
	}
	var created createJobResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func testApp(t *testing.T, manager *jobs.Manager) *App {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewAppWithAllowedRoots(manager, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func jobBody(t *testing.T, suffix string) string {
	t.Helper()
	path, err := json.Marshal(testRepoPath(t))
	if err != nil {
		t.Fatal(err)
	}
	return `{"repoPath":` + string(path) + suffix + `}`
}

func testRepoPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func waitForStatus(t *testing.T, app *App, id string, want jobs.Status) jobStatusResponse {
	t.Helper()
	handler := app.Handler()
	deadline := time.After(2 * time.Second)
	for {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+id, nil)
		req.AddCookie(app.sessionCookie())
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status request = %d", res.Code)
		}
		var status jobStatusResponse
		if err := json.Unmarshal(res.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status.Status == want {
			return status
		}
		select {
		case <-deadline:
			t.Fatalf("status = %s, want %s", status.Status, want)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
