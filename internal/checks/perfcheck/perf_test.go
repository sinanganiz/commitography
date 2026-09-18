//go:build perfcheck

// Process execution in this file is permitted by ADR-0065 clause 3: it runs the
// toolchain, the container runtime and the built binary. Every process gets a
// fixed argument vector, no shell and a timeout (ADR-0065 clause 4).

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

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/server"
)

// Bounds this package enforces. They are generous: the tests record numbers,
// and fail only when behavior is clearly wrong.
const (
	responsivenessP95Bound = 250 * time.Millisecond
	cancellationBound      = 5 * time.Second
	retainedHeapSlack      = 8 << 20
)

// perf is what the measurements share: the checkout, the generated
// repositories, an optional real repository, and the clock every duration is
// read from. The measurements need the process clock, and receive it by
// injection like everything else (ADR-0042 clause 4). TestPerformance
// establishes it once and passes it to each case, so nothing is held in a
// package variable (ADR-0042 clause 2).
type perf struct {
	checkout string
	// repos is the allowed root that holds the generated repositories.
	repos  string
	large  string
	medium string
	// realRepo is an optional real repository for the analysis timings.
	realRepo string
	clock    core.Clock
}

// TestPerformance generates the repositories once and runs every measurement
// against them.
func TestPerformance(t *testing.T) {
	p := setUp(t)
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"AnalysisDuration", p.analysisDuration},
		{"ServerStaysResponsiveDuringAnalysis", p.serverStaysResponsiveDuringAnalysis},
		{"CancellationLatency", p.cancellationLatency},
		{"RetainedReportsAreBoundedByTheJobLimit", p.retainedReportsAreBoundedByTheJobLimit},
		{"NativeStartupAndCLI", p.nativeStartupAndCLI},
		{"DockerStartupAndMountOverhead", p.dockerStartupAndMountOverhead},
	} {
		t.Run(tc.name, tc.run)
	}
}

func setUp(t *testing.T) perf {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("", "commitography-perf-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeTree(dir) })
	p := perf{
		checkout: root,
		repos:    dir,
		large:    filepath.Join(dir, "large"),
		medium:   filepath.Join(dir, "medium"),
		realRepo: os.Getenv("COMMITOGRAPHY_PERF_REPO"),
		clock:    core.SystemClock(),
	}

	started := p.clock.Now()
	for _, repo := range []struct {
		path    string
		commits int
	}{{p.large, 15000}, {p.medium, 3000}} {
		if err := generateRepository(repo.path, repo.commits, 400); err != nil {
			t.Fatalf("generating %s: %v", repo.path, err)
		}
	}
	t.Logf("generated a 15,000-commit and a 3,000-commit repository in %s", p.clock.Now().Sub(started).Round(time.Millisecond))
	return p
}

// generateRepository writes a linear history with git fast-import: each commit
// rewrites one of files files, so blame has real history to walk.
func generateRepository(dir string, commits, files int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if out, err := runGit(dir, nil, "init", "-q", "-b", "main"); err != nil {
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
	if out, err := runGit(dir, strings.NewReader(stream.String()), "fast-import", "--quiet"); err != nil {
		return fmt.Errorf("git fast-import: %v: %s", err, out)
	}
	if out, err := runGit(dir, nil, "reset", "-q", "--hard", "main"); err != nil {
		return fmt.Errorf("git reset: %v: %s", err, out)
	}
	return nil
}

func runGit(dir string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := git.Command(dir, append([]string{"-c", "core.autocrlf=false"}, args...)...)
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

func (p perf) timeAnalysis(t *testing.T, repo string, noBlame bool) time.Duration {
	t.Helper()
	started := p.clock.Now()
	clock, files := p.clock, core.SystemFilesystem()
	analyzer := pipeline.New(collect.New(clock, files), aggregate.New(clock, files), files)
	if _, err := analyzer.Run(context.Background(), pipeline.Options{RepoPath: repo, NoBlame: noBlame, PerAuthor: true}, nil); err != nil {
		t.Fatalf("analyzing %s: %v", repo, err)
	}
	return p.clock.Now().Sub(started)
}

func (p perf) analysisDuration(t *testing.T) {
	targets := []struct{ name, path string }{
		{"basic fixture", filepath.Join(p.checkout, "testdata", "fixtures", "basic")},
		{"generated, 3,000 commits", p.medium},
		{"generated, 15,000 commits", p.large},
	}
	if p.realRepo != "" {
		targets = append(targets, struct{ name, path string }{"COMMITOGRAPHY_PERF_REPO", p.realRepo})
	}
	for _, target := range targets {
		if _, err := os.Stat(target.path); err != nil {
			t.Logf("%-28s skipped: %v", target.name, err)
			continue
		}
		withBlame := p.timeAnalysis(t, target.path, false)
		withoutBlame := p.timeAnalysis(t, target.path, true)
		t.Logf("%-28s with blame %8s   --no-blame %8s", target.name, withBlame.Round(time.Millisecond), withoutBlame.Round(time.Millisecond))
	}
}

// --- in-process server -----------------------------------------------------------

type harness struct {
	t      *testing.T
	clock  core.Clock
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

func (p perf) newHarness(t *testing.T) *harness {
	t.Helper()
	app, err := newApp(p.clock, []string{p.repos})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, clock: p.clock, app: app, server: httptest.NewServer(app.Handler())}
	jar, _ := cookiejar.New(nil)
	h.client = &http.Client{Jar: jar, Timeout: time.Minute}
	t.Cleanup(func() {
		app.Jobs.CancelAll()
		for i := 0; i < 300; i++ {
			active := false
			for _, job := range app.Jobs.List() {
				active = active || job.Status == server.StatusQueued || job.Status == server.StatusRunning
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
	started := h.clock.Now()
	res, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	data, _ := io.ReadAll(res.Body)
	res.Body.Close()
	elapsed := h.clock.Now().Sub(started)
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
	deadline := h.clock.Now().Add(timeout)
	for {
		status := h.status(id)
		if done(status) {
			return status, true
		}
		if h.clock.Now().After(deadline) {
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

func (p perf) serverStaysResponsiveDuringAnalysis(t *testing.T) {
	h := p.newHarness(t)
	id := h.start(p.large, false)
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

func (p perf) cancellationLatency(t *testing.T) {
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
		h := p.newHarness(t)
		id := h.start(p.large, target.noBlame)
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
		requested := p.clock.Now()
		h.do(http.MethodPost, "/api/v1/jobs/"+id+"/cancel", nil, nil)
		status, ok := h.waitUntil(id, time.Minute, finished)
		latency := p.clock.Now().Sub(requested)
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

func (p perf) retainedReportsAreBoundedByTheJobLimit(t *testing.T) {
	h := p.newHarness(t)
	heap := map[int]uint64{0: heapInUse()}
	for i := 1; i <= 20; i++ {
		id := h.start(p.medium, true)
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

func waitForOK(clock core.Clock, url string, timeout time.Duration) bool {
	deadline := clock.Now().Add(timeout)
	for clock.Now().Before(deadline) {
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

func (p perf) buildBinary(t *testing.T, goos, goarch string) string {
	t.Helper()
	dir := t.TempDir()
	name := "commitography"
	if goos == "windows" {
		name += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", filepath.Join(dir, name), "./cmd/commitography")
	build.Dir = p.checkout
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	return filepath.Join(dir, name)
}

func median(samples []time.Duration) time.Duration { return percentile(samples, 0.5) }

func (p perf) nativeStartupAndCLI(t *testing.T) {
	binary := p.buildBinary(t, runtime.GOOS, runtime.GOARCH)
	var startups []time.Duration
	for i := 0; i < 5; i++ {
		port := freePort(t)
		started := p.clock.Now()
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		cmd := exec.CommandContext(ctx, binary, "serve", "--listen", fmt.Sprintf("127.0.0.1:%d", port), "--allowed-root", p.repos)
		if err := cmd.Start(); err != nil {
			cancel()
			t.Fatal(err)
		}
		ok := waitForOK(p.clock, fmt.Sprintf("http://127.0.0.1:%d/", port), 30*time.Second)
		startups = append(startups, p.clock.Now().Sub(started))
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		cancel()
		if !ok {
			t.Fatal("the native server did not answer")
		}
	}
	t.Logf("native serve, start to first response: median %s, max %s", median(startups).Round(time.Millisecond), percentile(startups, 1).Round(time.Millisecond))

	for _, target := range []struct{ name, path string }{{"basic fixture", filepath.Join(p.checkout, "testdata", "fixtures", "basic")}, {"generated, 15,000 commits", p.large}} {
		for _, noBlame := range []bool{false, true} {
			args := []string{target.path, "-o", t.TempDir(), "-q"}
			if noBlame {
				args = append(args, "--no-blame")
			}
			started := p.clock.Now()
			ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			out, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("native CLI on %s: %v: %s", target.name, err, out)
			}
			t.Logf("native CLI %-26s no-blame=%-5v %s", target.name, noBlame, p.clock.Now().Sub(started).Round(time.Millisecond))
		}
	}
}

// commandTimeout bounds every process these tests start (ADR-0065 clause 4).
const commandTimeout = 10 * time.Minute

// docker runs the Docker CLI and returns its combined, trimmed output.
func docker(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (p perf) dockerStartupAndMountOverhead(t *testing.T) {
	arch, err := docker("version", "--format", "{{.Server.Arch}}")
	if err != nil {
		t.Skip("Docker is not available")
	}
	context := t.TempDir()
	linux := p.buildBinary(t, "linux", arch)
	data, err := os.ReadFile(linux)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(context, "commitography"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	dockerfile, err := os.ReadFile(filepath.Join(p.checkout, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(context, "Dockerfile"), dockerfile, 0o644); err != nil {
		t.Fatal(err)
	}
	// The tag only has to be unique among concurrent runs, so it is drawn from
	// randomness rather than from the clock.
	suffix := make([]byte, 8)
	if _, err := io.ReadFull(core.SystemRandom(), suffix); err != nil {
		t.Fatal(err)
	}
	image := fmt.Sprintf("commitography:perf-%x", suffix)
	if out, err := docker("build", "-q", "-t", image, context); err != nil {
		t.Fatalf("docker build: %v: %s", err, out)
	}
	t.Cleanup(func() { docker("image", "rm", "-f", image) })

	var empty []time.Duration
	for i := 0; i < 3; i++ {
		started := p.clock.Now()
		if out, err := docker("run", "--rm", "--entrypoint", "true", image); err != nil {
			t.Fatalf("docker run: %v: %s", err, out)
		}
		empty = append(empty, p.clock.Now().Sub(started))
	}
	t.Logf("docker run of an empty command: median %s", median(empty).Round(time.Millisecond))

	var startups []time.Duration
	for i := 0; i < 3; i++ {
		port := freePort(t)
		started := p.clock.Now()
		id, err := docker("run", "-d", "--publish", fmt.Sprintf("127.0.0.1:%d:8080", port),
			"--mount", "type=bind,source="+p.repos+",target=/repos,readonly",
			image, "serve", "--listen", "0.0.0.0:8080", "--allowed-root", "/repos")
		if err != nil {
			t.Fatalf("docker run serve: %v: %s", err, id)
		}
		ok := waitForOK(p.clock, fmt.Sprintf("http://127.0.0.1:%d/", port), 60*time.Second)
		startups = append(startups, p.clock.Now().Sub(started))
		docker("rm", "-f", id)
		if !ok {
			t.Fatal("the container did not answer")
		}
	}
	t.Logf("docker serve, docker run to first response: median %s, max %s", median(startups).Round(time.Millisecond), percentile(startups, 1).Round(time.Millisecond))

	targets := []struct{ name, path string }{{"basic fixture", filepath.Join(p.checkout, "testdata", "fixtures", "basic")}, {"generated, 15,000 commits", p.large}}
	if p.realRepo != "" {
		targets = append(targets, struct{ name, path string }{"COMMITOGRAPHY_PERF_REPO", p.realRepo})
	}
	for _, target := range targets {
		for _, noBlame := range []bool{false, true} {
			args := []string{"run", "--rm", "--mount", "type=bind,source=" + target.path + ",target=/repo,readonly", image, "/repo", "-o", "/tmp/out", "-q"}
			if noBlame {
				args = append(args, "--no-blame")
			}
			started := p.clock.Now()
			if out, err := docker(args...); err != nil {
				t.Fatalf("docker CLI on %s: %v: %s", target.name, err, out)
			}
			t.Logf("docker CLI %-26s no-blame=%-5v %s", target.name, noBlame, p.clock.Now().Sub(started).Round(time.Millisecond))
		}
	}
}

// newApp is the local application wired the way the server command wires it
// (cmd/commitography/compose.go), allowing the given roots.
func newApp(clock core.Clock, roots []string) (*server.App, error) {
	random, files := core.SystemRandom(), core.SystemFilesystem()
	collector := collect.New(clock, files)
	manager := server.NewManager(server.ManagerOptions{
		Clock:  clock,
		NewID:  server.RandomIDs(random),
		Runner: pipeline.New(collector, aggregate.New(clock, files), files).Run,
	})
	return server.NewApp(manager, collector, random, roots)
}
