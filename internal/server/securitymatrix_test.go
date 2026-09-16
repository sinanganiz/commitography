package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/pipeline"
)

// These tests are the WP-6.7 security matrix: path traversal through routes,
// cross-origin state changes, response headers on every response class, the
// session cookie, raw data in responses and the default listener.

func TestRouteTraversalReadsNoFiles(t *testing.T) {
	app := testApp(t, NewManager(ManagerOptions{}))
	goMod, err := os.ReadFile(filepath.Join(testRepoPath(t), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	marker := strings.SplitN(string(goMod), "\n", 2)[0]

	fetch := func(path string) *httptest.ResponseRecorder {
		req := localRequest(http.MethodGet, path, nil)
		req.AddCookie(app.sessionCookie())
		res := httptest.NewRecorder()
		app.Handler().ServeHTTP(res, req)
		return res
	}
	for _, path := range []string{
		"/go.mod",
		"/assets/../go.mod",
		"/assets/..%2fgo.mod",
		"/assets/%2e%2e/go.mod",
		"/assets/%2e%2e%2f%2e%2e%2fgo.mod",
		"/assets/..%5cgo.mod",
		"/assets/app.js/../../go.mod",
		"/assets//../go.mod",
		"/api/../go.mod",
		"/api/v1/jobs/..%2f..%2f..%2fgo.mod",
		"/api/v1/jobs/%2e%2e/%2e%2e/%2e%2e/go.mod",
		"/api/v1/jobs/x/..%2f..%2f..%2fgo.mod",
	} {
		res := fetch(path)
		// A cleaned path is redirected; the target must be refused as well.
		for hops := 0; hops < 3 && res.Code >= 300 && res.Code < 400; hops++ {
			res = fetch(res.Header().Get("Location"))
		}
		if res.Code == http.StatusOK {
			t.Errorf("%s answered 200", path)
		}
		if strings.Contains(res.Body.String(), marker) {
			t.Errorf("%s returned the contents of go.mod", path)
		}
	}

	// A job cannot be pointed at a file to read it either.
	body, err := json.Marshal(map[string]string{"repoPath": filepath.Join(testRepoPath(t), "go.mod")})
	if err != nil {
		t.Fatal(err)
	}
	res := call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: string(body)})
	if res.Code != http.StatusBadRequest || strings.Contains(res.Body.String(), marker) {
		t.Errorf("a job for a file answered %d: %s", res.Code, res.Body.String())
	}
}

func TestStateChangingRoutesRequireSessionAndSameOrigin(t *testing.T) {
	app, outcomes, started := controlledApp(t)
	id := createdID(t, expect(t, "create", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: jobBody(t, "")}), http.StatusAccepted, ""))
	waitForStatus(t, app, id, StatusRunning)

	for _, route := range []apiCall{
		{method: http.MethodPost, path: "/api/v1/jobs", body: jobBody(t, "")},
		{method: http.MethodPost, path: "/api/v1/jobs/" + id + "/cancel"},
		{method: http.MethodDelete, path: "/api/v1/jobs/" + id},
	} {
		for _, attempt := range []struct {
			label     string
			noSession bool
			origin    string
			status    int
			code      string
		}{
			{label: "without a session", noSession: true, status: http.StatusUnauthorized, code: "invalid_session"},
			{label: "without a session from another site", noSession: true, origin: "https://attacker.example", status: http.StatusUnauthorized, code: "invalid_session"},
			{label: "from another site", origin: "https://attacker.example", status: http.StatusForbidden, code: "invalid_origin"},
			{label: "from another local port", origin: "http://127.0.0.1:9999", status: http.StatusForbidden, code: "invalid_origin"},
			{label: "from an opaque origin", origin: "null", status: http.StatusForbidden, code: "invalid_origin"},
		} {
			request := route
			request.noSession = attempt.noSession
			request.origin = attempt.origin
			expect(t, route.method+" "+route.path+" "+attempt.label, call(t, app, request), attempt.status, attempt.code)
		}
	}

	// Every refused request left the job running and started nothing else.
	if status := waitForStatus(t, app, id, StatusRunning); status.Status != StatusRunning {
		t.Fatalf("the job is %s after refused requests", status.Status)
	}
	if n := started.Load(); n != 1 || len(app.Jobs.List()) != 1 {
		t.Fatalf("refused requests changed state: %d analyses, %d jobs", n, len(app.Jobs.List()))
	}
	outcomes <- outcome{result: &pipeline.Result{}}
	waitForStatus(t, app, id, StatusSucceeded)
}

func requireSecurityHeaders(t *testing.T, label string, res *httptest.ResponseRecorder) {
	t.Helper()
	header := res.Header()
	for name, want := range map[string]string{
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := header.Get(name); got != want {
			t.Errorf("%s (%d): %s = %q, want %q", label, res.Code, name, got, want)
		}
	}
	policy := header.Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'self'", "script-src 'self'", "connect-src 'self'", "object-src 'none'", "base-uri 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(policy, directive) {
			t.Errorf("%s (%d): Content-Security-Policy %q lacks %q", label, res.Code, policy, directive)
		}
	}
	if origin := header.Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("%s (%d): Access-Control-Allow-Origin = %q, but CORS must stay disabled", label, res.Code, origin)
	}
}

func TestSecurityHeadersOnEveryResponseClass(t *testing.T) {
	app, outcomes, _ := controlledApp(t)
	valid := jobBody(t, "")
	for _, tc := range []struct {
		label  string
		call   apiCall
		status int
	}{
		{label: "application page", call: apiCall{method: http.MethodGet, path: "/"}, status: http.StatusOK},
		{label: "asset", call: apiCall{method: http.MethodGet, path: "/assets/app.css"}, status: http.StatusOK},
		{label: "capabilities", call: apiCall{method: http.MethodGet, path: "/api/v1/capabilities", noSession: true}, status: http.StatusOK},
		{label: "created job", call: apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid}, status: http.StatusAccepted},
		{label: "conflict", call: apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid}, status: http.StatusConflict},
		{label: "missing session", call: apiCall{method: http.MethodGet, path: "/api/v1/jobs", noSession: true}, status: http.StatusUnauthorized},
		{label: "cross-origin", call: apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: valid, origin: "https://attacker.example"}, status: http.StatusForbidden},
		{label: "unknown job", call: apiCall{method: http.MethodGet, path: "/api/v1/jobs/unknown"}, status: http.StatusNotFound},
		{label: "unknown page", call: apiCall{method: http.MethodGet, path: "/nothing"}, status: http.StatusNotFound},
		{label: "wrong method", call: apiCall{method: http.MethodPost, path: "/api/v1/capabilities"}, status: http.StatusMethodNotAllowed},
		{label: "oversized body", call: apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: `{"repoPath":"` + strings.Repeat("a", maxRequestBody) + `"}`}, status: http.StatusRequestEntityTooLarge},
		// Go's router redirects a path it cleans; the status code depends on the Go version.
		{label: "cleaned path", call: apiCall{method: http.MethodGet, path: "/assets/../x"}, status: http.StatusTemporaryRedirect},
	} {
		res := call(t, app, tc.call)
		if tc.label == "cleaned path" && res.Code >= 300 && res.Code < 400 {
			res.Code = tc.status
		}
		if res.Code != tc.status {
			t.Errorf("%s: status = %d, want %d", tc.label, res.Code, tc.status)
		}
		requireSecurityHeaders(t, tc.label, res)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	req.Host = "attacker.example"
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Errorf("foreign host: status = %d, want 403", res.Code)
	}
	requireSecurityHeaders(t, "foreign host", res)

	outcomes <- outcome{result: &pipeline.Result{}}
}

func TestSessionCookieIsProcessScopedAndStrict(t *testing.T) {
	first := NewApp(NewManager(ManagerOptions{}))
	res := httptest.NewRecorder()
	first.Handler().ServeHTTP(res, localRequest(http.MethodGet, "/api/v1/capabilities", nil))
	cookies := res.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("capabilities set %d cookies, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Errorf("session cookie = %+v, want HttpOnly, SameSite=Strict, Path=/ and no Domain", cookie)
	}
	if !cookie.Expires.IsZero() || cookie.MaxAge != 0 {
		t.Errorf("session cookie persists beyond the browser session: %+v", cookie)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(cookie.Value) {
		t.Errorf("session value %q is not a 256-bit random hex token", cookie.Value)
	}
	if second := NewApp(NewManager(ManagerOptions{})); second.sessionToken == first.sessionToken {
		t.Error("two server processes share a session secret")
	}
}

func jsonKeys(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("not a JSON object: %.200s", data)
	}
	keys := map[string]bool{}
	for key := range object {
		keys[key] = true
	}
	return keys
}

// A real analysis must answer with the documented shapes only: the status and
// list contracts, and a report whose top-level fields are the report schema's.
// Raw history, cache files and plaintext e-mail addresses have no place in them.
func TestResponsesCarryOnlyDocumentedData(t *testing.T) {
	app := testApp(t, NewManager(ManagerOptions{}))
	id := createdID(t, expect(t, "create", call(t, app, apiCall{method: http.MethodPost, path: "/api/v1/jobs", body: jobBody(t, `,"options":{"noBlame":true}`)}), http.StatusAccepted, ""))
	deadline := time.Now().Add(2 * time.Minute)
	var statusBody []byte
	for {
		res := call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs/" + id})
		statusBody = res.Body.Bytes()
		var status jobStatusResponse
		if err := json.Unmarshal(statusBody, &status); err != nil {
			t.Fatal(err)
		}
		if status.Status == StatusSucceeded {
			break
		}
		if status.Status != StatusQueued && status.Status != StatusRunning {
			t.Fatalf("analysis of the repository ended %s: %+v", status.Status, status.Error)
		}
		if time.Now().After(deadline) {
			t.Fatal("the analysis did not finish")
		}
		time.Sleep(50 * time.Millisecond)
	}

	wantStatus := []string{"id", "status", "repoName", "repoPath", "createdAt", "startedAt", "finishedAt", "elapsedMilliseconds", "progress", "warnings", "error"}
	if keys := jsonKeys(t, statusBody); len(keys) != len(wantStatus) {
		t.Errorf("status fields = %v, want exactly %v", keys, wantStatus)
	} else {
		for _, key := range wantStatus {
			if !keys[key] {
				t.Errorf("status lacks %q", key)
			}
		}
	}

	var list struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs"}).Body.Bytes(), &list); err != nil || len(list.Jobs) != 1 {
		t.Fatalf("job list = %+v (%v)", list, err)
	}
	wantSummary := []string{"id", "status", "repoName", "createdAt", "startedAt", "finishedAt", "warningCount"}
	if keys := jsonKeys(t, list.Jobs[0]); len(keys) != len(wantSummary) {
		t.Errorf("job summary fields = %v, want exactly %v (no repository path)", keys, wantSummary)
	}

	schemaData, err := os.ReadFile(filepath.Join(testRepoPath(t), "docs", "legacy", "report-schema-v0.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(schemaData, &schema); err != nil || len(schema.Properties) == 0 {
		t.Fatalf("reading the report schema: %v", err)
	}
	report := call(t, app, apiCall{method: http.MethodGet, path: "/api/v1/jobs/" + id + "/report"}).Body.Bytes()
	keys := jsonKeys(t, report)
	for key := range keys {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("the report carries %q, which the report schema does not define", key)
		}
	}
	for _, key := range schema.Required {
		if !keys[key] {
			t.Errorf("the report lacks required field %q", key)
		}
	}
	if email := regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`).Find(report); email != nil {
		t.Errorf("the default report contains an e-mail address: %s", email)
	}
}

func TestDefaultListenerIsLoopback(t *testing.T) {
	if defaultListenAddress != "127.0.0.1:8080" {
		t.Fatalf("default listen address = %q, want 127.0.0.1:8080", defaultListenAddress)
	}
}
