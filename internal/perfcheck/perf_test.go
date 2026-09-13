//go:build perfcheck

package perfcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/analysis"
	"github.com/sinanganiz/commitography/internal/jobs"
	"github.com/sinanganiz/commitography/internal/server"
)

// Bounds this package enforces. They are generous: the tests record numbers,
// and fail only when behavior is clearly wrong.
const (
	responsivenessP95Bound = 250 * time.Millisecond
	cancellationBound      = 5 * time.Second
	retainedHeapSlack      = 8 << 20
)

var (
	checkout string
	// repos is the allowed root that holds the generated repositories.
	repos  string
	large  string
	medium string
	// realRepo is an optional real repository for the analysis timings.
	realRepo = os.Getenv("COMMITOGRAPHY_PERF_REPO")
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	checkout = root
	dir, err := os.MkdirTemp("", "commitography-perf-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer removeTree(dir)
	repos = dir
	large = filepath.Join(dir, "large")
	medium = filepath.Join(dir, "medium")

	started := time.Now()
	for _, repo := range []struct {
		path    string
		commits int
	}{{large, 15000}, {medium, 3000}} {
		if err := generateRepository(repo.path, repo.commits, 400); err != nil {
			fmt.Fprintf(os.Stderr, "generating %s: %v\n", repo.path, err)
			return 1
		}
	}
	fmt.Printf("generated a 15,000-commit and a 3,000-commit repository in %s\n", time.Since(started).Round(time.Millisecond))
	return m.Run()
}

// generateRepository writes a linear history with git fast-import: each commit
// rewrites one of files files, so blame has real history to walk.
func generateRepository(dir string, commits, files int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if out, err := git(dir, nil, "init", "-q", "-b", "main"); err != nil {
		return fmt.Errorf("git init: %v: %s", err, out)
	}
	var stream strings.Builder
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	for i := 1; i <= commits; i++ {
		author := fmt.Sprintf("Dev %d <dev%d@example.com> %d +0100", i%7, i%7, start+int64(i)*3600)
		message := fmt.Sprintf("feat: update module %d\n", i%files)
		content := fmt.Sprintf("module %d\nrevision %d\n", i%files, i)
		fmt.Fprintf(&stream, "commit refs/heads/main\nmark :%d\nauthor %s\ncommitter %s\ndata %d\n%s", i, author, author, len(message), message)
		if i > 1 {
			fmt.Fprintf(&stream, "from :%d\n", i-1)
		}
		fmt.Fprintf(&stream, "M 100644 inline src/module-%d.txt\ndata %d\n%s\n", i%files, len(content), content)
	}
	if out, err := git(dir, strings.NewReader(stream.String()), "fast-import", "--quiet"); err != nil {
		return fmt.Errorf("git fast-import: %v: %s", err, out)
	}
	if out, err := git(dir, nil, "reset", "-q", "--hard", "main"); err != nil {
		return fmt.Errorf("git reset: %v: %s", err, out)
	}
	return nil
}

func git(dir string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.autocrlf=false"}, args...)...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	return cmd.CombinedOutput()
}

// removeTree also removes the read-only object files Git writes.
func removeTree(dir string) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil {
			_ = os.Chmod(path, 0o777)
		}
		return nil
	})
	_ = os.RemoveAll(dir)
}

// --- analysis timings ------------------------------------------------------------

func timeAnalysis(t *testing.T, repo string, noBlame bool) time.Duration {
	t.Helper()
	started := time.Now()
	if _, err := analysis.Run(context.Background(), analysis.Options{RepoPath: repo, NoBlame: noBlame, PerAuthor: true}, nil); err != nil {
		t.Fatalf("analyzing %s: %v", repo, err)
	}
	return time.Since(started)
}

func TestAnalysisDuration(t *testing.T) {
	targets := []struct{ name, path string }{
		{"basic fixture", filepath.Join(checkout, "testdata", "fixtures", "basic")},
		{"generated, 3,000 commits", medium},
		{"generated, 15,000 commits", large},
	}
	if realRepo != "" {
		targets = append(targets, struct{ name, path string }{"COMMITOGRAPHY_PERF_REPO", realRepo})
	}
	for _, target := range targets {
		if _, err := os.Stat(target.path); err != nil {
			t.Logf("%-28s skipped: %v", target.name, err)
			continue
		}
		withBlame := timeAnalysis(t, target.path, false)
		withoutBlame := timeAnalysis(t, target.path, true)
		t.Logf("%-28s with blame %8s   --no-blame %8s", target.name, withBlame.Round(time.Millisecond), withoutBlame.Round(time.Millisecond))
	}
}

// --- in-process server -----------------------------------------------------------

type harness struct {
	t      *testing.T
	app    *server.App
	server *httptest.Server
	client *http.Client
}

type jobStatus struct {
	Status   string `json:"status"`
	Progress *struct {
		Stage   string `json:"stage"`
		Current int    `json:"current"`
	} `json:"progress"`
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	app, err := server.NewAppWithAllowedRoots(jobs.New(jobs.Options{}), []string{repos})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, app: app, server: httptest.NewServer(app.Handler())}
	jar, _ := cookiejar.New(nil)
	h.client = &http.Client{Jar: jar, Timeout: time.Minute}
	t.Cleanup(func() {
		app.Jobs.CancelAll()
		for i := 0; i < 300; i++ {
			active := false
			for _, job := range app.Jobs.List() {
				active = active || job.Status == jobs.StatusQueued || job.Status == jobs.StatusRunning
			}
			if !active {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		h.server.Close()
	})
	h.do(http.MethodGet, "/api/v1/capabilities", nil, nil)
	return h
}

func (h *harness) do(method, path string, body, out any) (int, time.Duration) {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	started := time.Now()
	res, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	data, _ := io.ReadAll(res.Body)
	res.Body.Close()
	elapsed := time.Since(started)
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			h.t.Fatalf("%s %s answered %d: %s", method, path, res.StatusCode, data)
		}
	}
	return res.StatusCode, elapsed
}

func (h *harness) start(repo string, noBlame bool) string {
	h.t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	code, _ := h.do(http.MethodPost, "/api/v1/jobs", map[string]any{
		"repoPath": repo,
		"options": map[string]any{
			"noBlame": noBlame, "perAuthor": true, "anonymize": false, "allowShallow": false,
			"countMerges": false, "since": "", "until": "",
		},
	}, &created)
	if code != http.StatusAccepted {
		h.t.Fatalf("starting a job for %s answered %d", repo, code)
	}
	return created.ID
}

func (h *harness) status(id string) jobStatus {
	h.t.Helper()
	var status jobStatus
	h.do(http.MethodGet, "/api/v1/jobs/"+id, nil, &status)
	return status
}

func (h *harness) waitUntil(id string, timeout time.Duration, done func(jobStatus) bool) (jobStatus, bool) {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		status := h.status(id)
		if done(status) {
			return status, true
		}
		if time.Now().After(deadline) {
			return status, false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func finished(status jobStatus) bool { return status.Status != "queued" && status.Status != "running" }

func percentile(samples []time.Duration, p float64) time.Duration {
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(float64(len(sorted)-1)*p)]
}

func TestServerStaysResponsiveDuringAnalysis(t *testing.T) {
	h := newHarness(t)
	id := h.start(large, false)
	if _, ok := h.waitUntil(id, time.Minute, func(s jobStatus) bool { return s.Status == "running" }); !ok {
		t.Fatal("the job did not start")
	}
	var status, page []time.Duration
	for i := 0; i < 300; i++ {
		var s jobStatus
		_, elapsed := h.do(http.MethodGet, "/api/v1/jobs/"+id, nil, &s)
		status = append(status, elapsed)
		_, elapsed = h.do(http.MethodGet, "/", nil, nil)
		page = append(page, elapsed)
		if finished(s) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(status) < 50 {
		t.Fatalf("the analysis ended after only %d samples; the repository is too small to measure", len(status))
	}
	for _, series := range []struct {
		name    string
		samples []time.Duration
	}{{"GET /api/v1/jobs/{id}", status}, {"GET /", page}} {
		p50, p95, worst := percentile(series.samples, 0.5), percentile(series.samples, 0.95), percentile(series.samples, 1)
		t.Logf("%-22s %d requests during analysis: p50 %s, p95 %s, max %s", series.name, len(series.samples), p50, p95, worst)
		if p95 > responsivenessP95Bound {
			t.Errorf("%s p95 %s exceeds %s", series.name, p95, responsivenessP95Bound)
		}
	}
}

func TestCancellationLatency(t *testing.T) {
	var worst time.Duration
	for _, target := range []struct {
		stage   string
		noBlame bool
	}{
		{stage: "collecting", noBlame: false},
		{stage: "identity", noBlame: false},
		{stage: "code", noBlame: false},
		{stage: "messages", noBlame: false},
	} {
		h := newHarness(t)
		id := h.start(large, target.noBlame)
		// Reaching the messages stage means waiting out blame on 15,000 commits.
		if _, ok := h.waitUntil(id, 5*time.Minute, func(s jobStatus) bool {
			return finished(s) || (s.Status == "running" && s.Progress != nil && s.Progress.Stage == target.stage)
		}); !ok {
			t.Fatalf("the job never reached %s", target.stage)
		}
		if status := h.status(id); finished(status) {
			t.Logf("stage %-10s not measured: the job finished first", target.stage)
			continue
		}
		requested := time.Now()
		h.do(http.MethodPost, "/api/v1/jobs/"+id+"/cancel", nil, nil)
		status, ok := h.waitUntil(id, time.Minute, finished)
		latency := time.Since(requested)
		if !ok || status.Status != "cancelled" {
			t.Fatalf("cancelling during %s ended %s", target.stage, status.Status)
		}
		t.Logf("stage %-10s cancelled in %s", target.stage, latency.Round(time.Millisecond))
		if latency > worst {
			worst = latency
		}
	}
	t.Logf("worst cancellation latency %s (bound %s)", worst.Round(time.Millisecond), cancellationBound)
	if worst > cancellationBound {
		t.Errorf("cancellation took %s, over the %s bound", worst, cancellationBound)
	}
}

func heapInUse() uint64 {
	runtime.GC()
	debug.FreeOSMemory()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

func TestRetainedReportsAreBoundedByTheJobLimit(t *testing.T) {
	h := newHarness(t)
	heap := map[int]uint64{0: heapInUse()}
	for i := 1; i <= 20; i++ {
		id := h.start(medium, true)
		if status, ok := h.waitUntil(id, 5*time.Minute, finished); !ok || status.Status != "succeeded" {
			t.Fatalf("job %d ended %s", i, status.Status)
		}
		if i%5 == 0 {
			heap[i] = heapInUse()
			t.Logf("after %2d jobs: %3d retained, heap %6.1f MiB", i, len(h.app.Jobs.List()), float64(heap[i])/(1<<20))
		}
	}
	if retained := len(h.app.Jobs.List()); retained != 10 {
		t.Errorf("%d jobs retained, want 10", retained)
	}
	perReport := (int64(heap[10]) - int64(heap[0])) / 10
	growth := int64(heap[20]) - int64(heap[10])
	t.Logf("about %.1f KiB per retained report; heap growth from 10 to 20 jobs %.1f MiB", float64(perReport)/(1<<10), float64(growth)/(1<<20))
	if growth > retainedHeapSlack {
		t.Errorf("heap grew by %.1f MiB after the job limit was reached", float64(growth)/(1<<20))
	}
}

// --- process startup and Docker ----------------------------------------------------

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForOK(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if res, err := http.Get(url); err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func buildBinary(t *testing.T, goos, goarch string) string {
	t.Helper()
	dir := t.TempDir()
	name := "commitography"
	if goos == "windows" {
		name += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(dir, name), "./cmd/commitography")
	build.Dir = checkout
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	return filepath.Join(dir, name)
}

func median(samples []time.Duration) time.Duration { return percentile(samples, 0.5) }

func TestNativeStartupAndCLI(t *testing.T) {
	binary := buildBinary(t, runtime.GOOS, runtime.GOARCH)
	var startups []time.Duration
	for i := 0; i < 5; i++ {
		port := freePort(t)
		started := time.Now()
		cmd := exec.Command(binary, "serve", "--listen", fmt.Sprintf("127.0.0.1:%d", port), "--allowed-root", repos)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		ok := waitForOK(fmt.Sprintf("http://127.0.0.1:%d/", port), 30*time.Second)
		startups = append(startups, time.Since(started))
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if !ok {
			t.Fatal("the native server did not answer")
		}
	}
	t.Logf("native serve, start to first response: median %s, max %s", median(startups).Round(time.Millisecond), percentile(startups, 1).Round(time.Millisecond))

	for _, target := range []struct{ name, path string }{{"basic fixture", filepath.Join(checkout, "testdata", "fixtures", "basic")}, {"generated, 15,000 commits", large}} {
		for _, noBlame := range []bool{false, true} {
			args := []string{target.path, "-o", t.TempDir(), "-q"}
			if noBlame {
				args = append(args, "--no-blame")
			}
			started := time.Now()
			if out, err := exec.Command(binary, args...).CombinedOutput(); err != nil {
				t.Fatalf("native CLI on %s: %v: %s", target.name, err, out)
			}
			t.Logf("native CLI %-26s no-blame=%-5v %s", target.name, noBlame, time.Since(started).Round(time.Millisecond))
		}
	}
}

func docker(args ...string) (string, error) {
	out, err := exec.Command("docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func TestDockerStartupAndMountOverhead(t *testing.T) {
	arch, err := docker("version", "--format", "{{.Server.Arch}}")
	if err != nil {
		t.Skip("Docker is not available")
	}
	context := t.TempDir()
	linux := buildBinary(t, "linux", arch)
	data, err := os.ReadFile(linux)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(context, "commitography"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	dockerfile, err := os.ReadFile(filepath.Join(checkout, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(context, "Dockerfile"), dockerfile, 0o644); err != nil {
		t.Fatal(err)
	}
	image := fmt.Sprintf("commitography:perf-%d", time.Now().UnixNano())
	if out, err := docker("build", "-q", "-t", image, context); err != nil {
		t.Fatalf("docker build: %v: %s", err, out)
	}
	t.Cleanup(func() { docker("image", "rm", "-f", image) })

	var empty []time.Duration
	for i := 0; i < 3; i++ {
		started := time.Now()
		if out, err := docker("run", "--rm", "--entrypoint", "true", image); err != nil {
			t.Fatalf("docker run: %v: %s", err, out)
		}
		empty = append(empty, time.Since(started))
	}
	t.Logf("docker run of an empty command: median %s", median(empty).Round(time.Millisecond))

	var startups []time.Duration
	for i := 0; i < 3; i++ {
		port := freePort(t)
		started := time.Now()
		id, err := docker("run", "-d", "--publish", fmt.Sprintf("127.0.0.1:%d:8080", port),
			"--mount", "type=bind,source="+repos+",target=/repos,readonly",
			image, "serve", "--listen", "0.0.0.0:8080", "--allowed-root", "/repos")
		if err != nil {
			t.Fatalf("docker run serve: %v: %s", err, id)
		}
		ok := waitForOK(fmt.Sprintf("http://127.0.0.1:%d/", port), 60*time.Second)
		startups = append(startups, time.Since(started))
		docker("rm", "-f", id)
		if !ok {
			t.Fatal("the container did not answer")
		}
	}
	t.Logf("docker serve, docker run to first response: median %s, max %s", median(startups).Round(time.Millisecond), percentile(startups, 1).Round(time.Millisecond))

	targets := []struct{ name, path string }{{"basic fixture", filepath.Join(checkout, "testdata", "fixtures", "basic")}, {"generated, 15,000 commits", large}}
	if realRepo != "" {
		targets = append(targets, struct{ name, path string }{"COMMITOGRAPHY_PERF_REPO", realRepo})
	}
	for _, target := range targets {
		for _, noBlame := range []bool{false, true} {
			args := []string{"run", "--rm", "--mount", "type=bind,source=" + target.path + ",target=/repo,readonly", image, "/repo", "-o", "/tmp/out", "-q"}
			if noBlame {
				args = append(args, "--no-blame")
			}
			started := time.Now()
			if out, err := docker(args...); err != nil {
				t.Fatalf("docker CLI on %s: %v: %s", target.name, err, out)
			}
			t.Logf("docker CLI %-26s no-blame=%-5v %s", target.name, noBlame, time.Since(started).Round(time.Millisecond))
		}
	}
}
