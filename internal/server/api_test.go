package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	app := NewApp(manager)
	handler := app.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"repoPath":"/repos/project","options":{"noBlame":true}}`))
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

	req = httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"repoPath":"/repos/project","outputDir":"/tmp/out"}`))
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
	app := NewApp(manager)
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

func TestAPICancelTransitionsJob(t *testing.T) {
	started := make(chan struct{}, 1)
	manager := jobs.New(jobs.Options{
		Runner: func(ctx context.Context, _ analysis.Options, _ analysis.ProgressSink) (*analysis.Result, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	app := NewApp(manager)
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
	app := NewApp(jobs.New(jobs.Options{
		Runner: func(context.Context, analysis.Options, analysis.ProgressSink) (*analysis.Result, error) {
			return &analysis.Result{}, nil
		},
	}))
	handler := app.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"repoPath":"/repos/project"}`))
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

	req = httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"repoPath":"/repos/project"}`))
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"repoPath":"/repos/project"}`))
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
