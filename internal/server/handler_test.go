package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandlerServesShellAndEmbeddedAssets(t *testing.T) {
	handler := NewHandler()

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/", want: "commitography-root"},
		{path: "/assets/app.js", want: "commitography"},
		{path: "/assets/app.css", want: "--"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", res.Code)
			}
			if !strings.Contains(res.Body.String(), tc.want) {
				t.Fatalf("response does not contain %q", tc.want)
			}
		})
	}
}

func TestNewHandlerReservesAPIAndRejectsUnknownPaths(t *testing.T) {
	handler := NewHandler()
	for _, path := range []string{"/api/v1/jobs", "/unknown"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, res.Code)
		}
	}
}
