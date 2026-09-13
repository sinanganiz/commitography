package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/jobs"
)

// localRequest builds a request addressed to the loopback name a browser uses.
// httptest defaults the host to example.com, which the server refuses.
func localRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Host = "127.0.0.1:8080"
	return req
}

func TestHostHeaderAcceptsOnlyLoopbackNames(t *testing.T) {
	app := NewApp(jobs.New(jobs.Options{}))
	handler := app.Handler()
	for _, tc := range []struct {
		host    string
		allowed bool
	}{
		{host: "127.0.0.1:8080", allowed: true},
		{host: "localhost:9000", allowed: true},
		{host: "LOCALHOST", allowed: true},
		{host: "localhost.:8080", allowed: true},
		{host: "[::1]:8080", allowed: true},
		{host: "attacker.example:8080", allowed: false},
		{host: "127.0.0.1.nip.io:8080", allowed: false},
		{host: "localhost.attacker.example", allowed: false},
		{host: "192.168.1.20:8080", allowed: false},
		{host: "0.0.0.0:8080", allowed: false},
		{host: "", allowed: false},
	} {
		for _, path := range []string{"/", "/api/v1/capabilities"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Host = tc.host
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if tc.allowed {
				if res.Code != http.StatusOK {
					t.Errorf("Host %q %s status = %d, want 200", tc.host, path, res.Code)
				}
				continue
			}
			if res.Code != http.StatusForbidden {
				t.Errorf("Host %q %s status = %d, want 403", tc.host, path, res.Code)
			}
			if cookie := res.Header().Get("Set-Cookie"); cookie != "" {
				t.Errorf("Host %q %s received a session cookie: %s", tc.host, path, cookie)
			}
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Host %q %s rejection is missing security headers", tc.host, path)
			}
			if path == "/" {
				continue
			}
			var body apiError
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.Error.Code != "invalid_host" {
				t.Errorf("Host %q %s body = %s, want invalid_host", tc.host, path, res.Body.String())
			}
		}
	}
}

// A DNS-rebinding page shares its own name with the Origin it sends, so the
// origin check alone passes. Even with a valid session cookie it must be
// refused before a job exists.
func TestRebindingPageCannotStartJobs(t *testing.T) {
	app := testApp(t, jobs.New(jobs.Options{}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(jobBody(t, "")))
	req.Host = "attacker.example:8080"
	req.Header.Set("Origin", "http://attacker.example:8080")
	req.AddCookie(app.sessionCookie())
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("rebinding create status = %d, want 403: %s", res.Code, res.Body.String())
	}
	if list := app.Jobs.List(); len(list) != 0 {
		t.Fatalf("a rebinding request created %d jobs", len(list))
	}
}

func TestExplicitListenAddressIsAnAllowedHost(t *testing.T) {
	app := NewApp(jobs.New(jobs.Options{}))
	app.AllowListenHost("192.168.1.20:8080")
	app.AllowListenHost("0.0.0.0:8080")
	app.AllowListenHost("[::]:8080")
	app.AllowListenHost(":8080")
	handler := app.Handler()
	for host, want := range map[string]int{
		"192.168.1.20:9999": http.StatusOK,
		"127.0.0.1:8080":    http.StatusOK,
		"0.0.0.0:8080":      http.StatusForbidden,
		"[::]:8080":         http.StatusForbidden,
		"10.0.0.5:8080":     http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
		req.Host = host
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != want {
			t.Errorf("Host %q status = %d, want %d", host, res.Code, want)
		}
	}
}
