package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
)

// A panic in a route becomes a 500 carrying nothing about the panic, and the
// panic does not escape the handler.
func TestRecoveryLayerReportsAPanicAsAnInternalError(t *testing.T) {
	secret := "/private/mounted/repository"
	handler := withRecovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("index out of range reading " + secret)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("the response is not the documented error body: %v (%s)", err, response.Body.String())
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("code = %q, want %q", body.Error.Code, "internal_error")
	}
	if body.Error.Reason != "" {
		t.Errorf("reason = %q, want none: a recovered panic is never a user error", body.Error.Reason)
	}
	if strings.Contains(response.Body.String(), secret) || strings.Contains(response.Body.String(), "index out of range") {
		t.Errorf("the response repeats the panic: %s", response.Body.String())
	}
}

// Once a status has been sent it cannot be replaced, so the connection is
// aborted rather than completed. A truncated response is what stops a client
// reading a panic as a success.
func TestRecoveryLayerAbortsAPanicAfterTheResponseBegan(t *testing.T) {
	handler := withRecovery(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jobs":[`))
		panic("nil map write")
	}))

	defer func() {
		value := recover()
		if value == nil {
			t.Fatal("a panic after the response began was not turned into an abort")
		}
		if !errors.Is(value.(error), http.ErrAbortHandler) {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", value)
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil))
}

// A deliberate abort is the standard library's own signal and passes through.
func TestRecoveryLayerPassesAnIntentionalAbortThrough(t *testing.T) {
	handler := withRecovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if value := recover(); !errors.Is(value.(error), http.ErrAbortHandler) {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", value)
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

// The layer is wired into the handler the command serves, not only available
// as a function. A panicking identifier generator is reached from inside a
// route, so this exercises the whole chain.
func TestHandlerIsBehindTheRecoveryLayer(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAppWithRoots(newTestManager(ManagerOptions{
		NewID: func() (string, error) { panic("no entropy source") },
	}), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Handler()

	body, err := json.Marshal(map[string]string{"repoPath": root})
	if err != nil {
		t.Fatal(err)
	}
	request := localRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(string(body)))
	request.AddCookie(app.sessionCookie())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (body %s)", response.Code, http.StatusInternalServerError, response.Body.String())
	}
}

// A panic inside the analysis fails the job and releases the active slot. It
// is never reported as a success, and it is never a user error.
func TestWorkerPanicFailsTheJob(t *testing.T) {
	manager := newTestManager(ManagerOptions{
		Runner: func(context.Context, pipeline.Options, pipeline.ProgressSink) (*pipeline.Result, error) {
			panic("slice bounds out of range")
		},
	})
	if _, err := manager.Start(t.TempDir(), pipeline.Options{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	const maxPolls = 400
	var snapshot Snapshot
	for poll := 0; ; poll++ {
		items := manager.List()
		if len(items) != 1 {
			t.Fatalf("jobs = %d, want 1", len(items))
		}
		snapshot = items[0]
		if snapshot.Status != StatusQueued && snapshot.Status != StatusRunning {
			break
		}
		if poll == maxPolls {
			t.Fatal("the panicking job never reached a terminal state")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if snapshot.Status != StatusFailed {
		t.Fatalf("status = %s, want %s", snapshot.Status, StatusFailed)
	}
	if snapshot.Failure == nil {
		t.Fatal("the failed job carries no failure")
	}
	if snapshot.Failure.Code != "analysis_failed" {
		t.Errorf("code = %q, want %q", snapshot.Failure.Code, "analysis_failed")
	}
	if snapshot.Failure.Reason != "" {
		t.Errorf("reason = %q, want none: a recovered panic is never a user error", snapshot.Failure.Reason)
	}
	if strings.Contains(snapshot.Failure.Message, "slice bounds") {
		t.Errorf("the failure repeats the panic: %q", snapshot.Failure.Message)
	}

	// The single active slot is released, so the next job can start.
	if _, err := manager.Create(t.TempDir()); err != nil {
		t.Errorf("the active slot was not released: %v", err)
	}
}

// A recovered panic is classified internal, which is what keeps it off the
// user exit code and out of a 4xx.
func TestRecoveredPanicIsAnInternalError(t *testing.T) {
	err := recovered("nil pointer dereference", "serving a request")
	if core.ClassOf(err) != core.ClassInternal {
		t.Errorf("class = %v, want internal", core.ClassOf(err))
	}
	if core.ExitCode(err) != core.ExitInternal {
		t.Errorf("exit code = %d, want %d", core.ExitCode(err), core.ExitInternal)
	}
	if core.HTTPStatus(err) != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", core.HTTPStatus(err), http.StatusInternalServerError)
	}
	if !strings.Contains(err.Error(), "nil pointer dereference") {
		t.Errorf("the diagnostic does not locate the origin: %q", err.Error())
	}
	if core.Artifact(err) == err.Error() {
		t.Error("the artifact rendering repeats the diagnostic, so the stack would travel")
	}
}
