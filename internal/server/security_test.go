package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/jobs"
)

func TestResponsesCarrySecurityHeaders(t *testing.T) {
	handler := NewApp(jobs.New(jobs.Options{})).Handler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	for _, header := range []string{"Cache-Control", "X-Content-Type-Options", "Referrer-Policy", "Content-Security-Policy"} {
		if res.Header().Get(header) == "" {
			t.Errorf("missing security header %s", header)
		}
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", res.Header().Get("Cache-Control"))
	}
}

func TestPathErrorDoesNotEchoFilesystemPath(t *testing.T) {
	app := NewApp(jobs.New(jobs.Options{}))
	handler := app.Handler()
	secretPath := "C:/private/secret-repository"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"repoPath":"`+secretPath+`"}`))
	req.AddCookie(app.sessionCookie())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden && res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want path rejection", res.Code)
	}
	if strings.Contains(res.Body.String(), secretPath) {
		t.Fatalf("response leaked repository path: %s", res.Body.String())
	}
}
