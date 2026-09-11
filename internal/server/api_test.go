package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sinanganiz/commitography/internal/jobs"
)

func TestAPICapabilitiesAndJobList(t *testing.T) {
	handler := NewApp(jobs.New(jobs.Options{})).Handler()

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
	handler := NewApp(jobs.New(jobs.Options{})).Handler()
	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodPost, path: "/api/v1/jobs", status: http.StatusNotImplemented},
		{method: http.MethodGet, path: "/api/v1/jobs/example", status: http.StatusNotImplemented},
		{method: http.MethodGet, path: "/api/v2/jobs", status: http.StatusNotFound},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tc.status {
			t.Errorf("%s %s status = %d, want %d", tc.method, tc.path, res.Code, tc.status)
		}
	}
}
