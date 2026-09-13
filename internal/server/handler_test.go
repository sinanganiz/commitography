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
			req := localRequest(http.MethodGet, tc.path, nil)
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
	for _, path := range []string{"/unknown"} {
		req := localRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, res.Code)
		}
	}
}

func TestEmbeddedAssetsCannotBeListed(t *testing.T) {
	handler := NewHandler()
	for _, path := range []string{"/assets/", "/assets"} {
		req := localRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		// "/assets" redirects to "/assets/", which must then be refused.
		if res.Code == http.StatusOK || strings.Contains(res.Body.String(), "app.js") {
			t.Errorf("%s status = %d, body %q: the asset directory is listable", path, res.Code, res.Body.String())
		}
	}
	req := localRequest(http.MethodGet, "/assets/", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Errorf("/assets/ status = %d, want 404", res.Code)
	}
}
