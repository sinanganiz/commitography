//go:build dockersmoke

package dockersmoke

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/server"
)

var (
	// image is the image under test.
	image string
	// fixtures is the generated fixture directory.
	fixtures string
	// skipReason explains why this environment cannot run the tests.
	skipReason string
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
	fixtures = filepath.Join(root, "testdata", "fixtures")
	if _, err := os.Stat(filepath.Join(fixtures, "basic", ".git")); err != nil {
		skipReason = "fixtures not built; run `make fixtures`"
		return m.Run()
	}
	if _, err := docker("version", "--format", "{{.Server.Arch}}"); err != nil {
		skipReason = "the Docker CLI or daemon is not available"
		return m.Run()
	}
	if name := os.Getenv("COMMITOGRAPHY_IMAGE"); name != "" {
		image = name
		return m.Run()
	}
	tag := fmt.Sprintf("commitography:smoke-%d", time.Now().UnixNano())
	if err := buildImage(root, tag); err != nil {
		fmt.Fprintf(os.Stderr, "building the image: %v\n", err)
		return 1
	}
	defer docker("image", "rm", "-f", tag)
	image = tag
	return m.Run()
}

// buildImage builds the image the way `make docker-image` does: a static Linux
// binary for the daemon's architecture next to the Dockerfile.
func buildImage(root, tag string) error {
	arch, err := docker("version", "--format", "{{.Server.Arch}}")
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "commitography-image-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	build := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(dir, "commitography"), "./cmd/commitography")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch)
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("go build: %v: %s", err, out)
	}
	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), dockerfile, 0o644); err != nil {
		return err
	}
	if out, err := docker("build", "-q", "-t", tag, dir); err != nil {
		return fmt.Errorf("docker build: %v: %s", err, out)
	}
	return nil
}

func TestImageKeepsTheCLIContract(t *testing.T) {
	requireEnvironment(t)
	out, err := docker("image", "inspect", "--format", "{{json .Config}}", image)
	must(t, err, out)
	var config struct {
		Entrypoint   []string
		Cmd          []string
		WorkingDir   string
		ExposedPorts map[string]struct{}
		Healthcheck  *struct{ Test []string }
	}
	if err := json.Unmarshal([]byte(out), &config); err != nil {
		t.Fatalf("image config: %v: %s", err, out)
	}
	if want := []string{"/sbin/tini", "--", "/usr/local/bin/commitography"}; !reflect.DeepEqual(config.Entrypoint, want) {
		t.Errorf("entrypoint = %v, want %v", config.Entrypoint, want)
	}
	if want := []string{"/repo", "-o", "/repo/out"}; !reflect.DeepEqual(config.Cmd, want) {
		t.Errorf("command = %v, want %v", config.Cmd, want)
	}
	if config.WorkingDir != "/repo" {
		t.Errorf("working directory = %q, want /repo", config.WorkingDir)
	}
	if len(config.ExposedPorts) != 0 {
		t.Errorf("image exposes %v; docker run -P would publish the server on every host interface", config.ExposedPorts)
	}
	if config.Healthcheck != nil {
		t.Errorf("image declares a health check %v, but the server has no health endpoint", config.Healthcheck.Test)
	}

	out, err = docker("run", "--rm", "--entrypoint", "git", image, "--version")
	must(t, err, out)
	if !strings.HasPrefix(out, "git version") {
		t.Errorf("git --version = %q", out)
	}
	out, err = docker("run", "--rm", "--entrypoint", "sh", image, "-c", "ls /usr/local/bin")
	must(t, err, out)
	if out != "commitography" {
		t.Errorf("/usr/local/bin holds %q, want only commitography", out)
	}
}

func TestCLIDefaultCommandWritesTheDashboard(t *testing.T) {
	requireEnvironment(t)
	repo := copyFixture(t, "basic", t.TempDir())
	args := append([]string{"run", "--rm"}, hostUser()...)
	out, err := docker(append(args, "--mount", mount(repo, "/repo", false), image)...)
	must(t, err, out)

	page, err := os.ReadFile(filepath.Join(repo, "out", "index.html"))
	if err != nil || !bytes.Contains(page, []byte(`id="commitography-root"`)) {
		t.Fatalf("out/index.html is missing or is not the dashboard: %v", err)
	}
	var report struct {
		SchemaVersion int `json:"schemaVersion"`
		Repository    struct {
			Name string `json:"name"`
		} `json:"repository"`
	}
	// The CLI names the repository after the path it analyzed, here /repo.
	data, err := os.ReadFile(filepath.Join(repo, "out", "report.json"))
	if err != nil || json.Unmarshal(data, &report) != nil || report.SchemaVersion != 1 || report.Repository.Name != "repo" {
		t.Fatalf("out/report.json = %+v (%v)", report, err)
	}
}

func TestCLIReadsReadOnlyRepositoriesAsAnyUser(t *testing.T) {
	requireEnvironment(t)
	repo := filepath.Join(fixtures, "basic")
	output := t.TempDir()
	if err := os.Chmod(output, 0o777); err != nil {
		t.Fatal(err)
	}
	out, err := docker("run", "--rm", "--user", nonRootUser(),
		"--mount", mount(repo, "/repo", true), "--mount", mount(output, "/out", false),
		image, "/repo", "-o", "/out", "-q")
	must(t, err, out)
	for _, name := range []string{"index.html", "report.json"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("%s was not written as a non-root user: %v", name, err)
		}
	}

	// The default output lies inside the read-only mount and must fail clearly.
	out, err = docker("run", "--rm", "--mount", mount(repo, "/repo", true), image)
	if exitCode(err) != 1 || !strings.Contains(out, "read-only file system") {
		t.Errorf("default output into a read-only mount: exit %d, output %q", exitCode(err), out)
	}
}

func TestServerModeMatchesTheNativeServer(t *testing.T) {
	requireEnvironment(t)
	base, _ := startServer(t, fixtures, true)
	client := newClient()

	if page := get(t, client, base+"/", http.StatusOK); !strings.Contains(page, "commitography-root") {
		t.Fatalf("the server did not serve the application shell: %.200s", page)
	}
	get(t, client, base+"/assets/app.js", http.StatusOK)
	get(t, client, base+"/assets/app.css", http.StatusOK)
	fromDocker := analyze(t, client, base, "/repos/basic")

	app, err := server.NewAppWithAllowedRoots(nil, []string{fixtures})
	if err != nil {
		t.Fatal(err)
	}
	native := httptest.NewServer(app.Handler())
	defer native.Close()
	fromNative := analyze(t, newClient(), native.URL, filepath.Join(fixtures, "basic"))

	// The generation time and tool version describe the run, not the repository.
	for _, report := range []map[string]any{fromDocker, fromNative} {
		delete(report, "generatedAt")
		delete(report, "toolVersion")
	}
	if diffs := difference("report", fromDocker, fromNative); len(diffs) > 0 {
		if len(diffs) > 20 {
			diffs = append(diffs[:20], fmt.Sprintf("... and %d more", len(diffs)-20))
		}
		t.Fatalf("the Docker report differs from the native report:\n%s", strings.Join(diffs, "\n"))
	}
}

func TestServerDoesNotWriteToMountedRepositories(t *testing.T) {
	requireEnvironment(t)
	parent := t.TempDir()
	repo := copyFixture(t, "basic", parent)
	before := snapshot(t, repo)

	base, _ := startServer(t, parent, false)
	analyze(t, newClient(), base, "/repos/basic")

	if after := snapshot(t, repo); !reflect.DeepEqual(before, after) {
		t.Fatalf("the analysis changed the mounted repository:\n%s", strings.Join(changes(before, after), "\n"))
	}
}

// changes lists the entries found in only one of two snapshots.
func changes(before, after []string) []string {
	count := map[string]int{}
	for _, entry := range before {
		count[entry]--
	}
	for _, entry := range after {
		count[entry]++
	}
	var out []string
	for entry, n := range count {
		switch {
		case n < 0:
			out = append(out, "- "+entry)
		case n > 0:
			out = append(out, "+ "+entry)
		}
	}
	sort.Strings(out)
	return out
}

func TestServerRefusesForeignHostsAndUndocumentedPaths(t *testing.T) {
	requireEnvironment(t)
	base, _ := startServer(t, fixtures, true)

	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/capabilities", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "attacker.example"
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden || res.Header.Get("Set-Cookie") != "" {
		t.Errorf("a foreign host received %d with cookie %q, want 403 without a cookie", res.StatusCode, res.Header.Get("Set-Cookie"))
	}

	client := newClient()
	get(t, client, base+"/api/v1/capabilities", http.StatusOK)
	for _, path := range []string{"/healthz", "/health", "/metrics", "/debug/pprof/", "/api/v1/admin", "/assets/"} {
		get(t, client, base+path, http.StatusNotFound)
	}
}

func TestStopSignalReachesTheServerThroughTini(t *testing.T) {
	requireEnvironment(t)
	_, id := startServer(t, fixtures, true)

	cmdline, err := docker("exec", id, "cat", "/proc/1/cmdline")
	must(t, err, cmdline)
	if !strings.HasPrefix(cmdline, "/sbin/tini") {
		t.Errorf("PID 1 is %q, want /sbin/tini", cmdline)
	}

	started := time.Now()
	out, err := docker("stop", "-t", "10", id)
	must(t, err, out)
	elapsed := time.Since(started)
	code, err := docker("inspect", "--format", "{{.State.ExitCode}}", id)
	must(t, err, code)
	// A signal that never reached the server would end in Docker's kill after
	// the ten-second grace period, with exit code 137.
	if elapsed > 5*time.Second || code != "0" {
		t.Errorf("docker stop took %s with exit code %s, want a graceful exit", elapsed, code)
	}
}

func TestMissingMountsExplainThemselves(t *testing.T) {
	requireEnvironment(t)

	t.Run("repository", func(t *testing.T) {
		out, err := docker("run", "--rm", image)
		if exitCode(err) != 2 {
			t.Errorf("exit code = %d, want 2: %s", exitCode(err), out)
		}
		for _, want := range []string{"/repo is not a git repository", "--mount type=bind,source=<repository>,target=/repo,readonly"} {
			if !strings.Contains(out, want) {
				t.Errorf("output does not contain %q:\n%s", want, out)
			}
		}
	})

	t.Run("allowed root", func(t *testing.T) {
		out, err := docker("run", "--rm", image, "serve", "--listen", "0.0.0.0:8080", "--allowed-root", "/repos")
		if exitCode(err) != 1 {
			t.Errorf("exit code = %d, want 1: %s", exitCode(err), out)
		}
		for _, want := range []string{`allowed root "/repos" does not exist`, "--mount type=bind,source=<folder>,target=/repos,readonly"} {
			if !strings.Contains(out, want) {
				t.Errorf("output does not contain %q:\n%s", want, out)
			}
		}
	})

	// Docker Engine refuses a missing bind source; Docker Desktop creates an
	// empty folder instead. Either way the user must learn what went wrong.
	t.Run("mistyped source", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "mistyped")
		out, err := docker("run", "--rm", "--mount", mount(missing, "/repo", true), image)
		if err == nil {
			t.Fatalf("a mistyped mount source succeeded:\n%s", out)
		}
		if !strings.Contains(out, "does not exist") && !strings.Contains(out, "--mount type=bind,source=<repository>") {
			t.Errorf("a mistyped mount source gave no useful error:\n%s", out)
		}
	})

	t.Run("empty allowed root", func(t *testing.T) {
		_, id := startServer(t, t.TempDir(), true)
		logs, err := docker("logs", id)
		must(t, err, logs)
		if !strings.Contains(logs, `allowed root "/repos" is empty`) {
			t.Errorf("the server did not warn about its empty allowed root:\n%s", logs)
		}
	})
}

func requireEnvironment(t *testing.T) {
	t.Helper()
	if skipReason != "" {
		t.Skip(skipReason)
	}
}

// docker runs the Docker CLI and returns its combined, trimmed output.
func docker(args ...string) (string, error) {
	out, err := exec.Command("docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func must(t *testing.T, err error, out string) {
	t.Helper()
	if err != nil {
		t.Fatalf("docker: %v\n%s", err, out)
	}
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exitErr):
		return exitErr.ExitCode()
	default:
		return -1
	}
}

func mount(source, target string, readonly bool) string {
	spec := "type=bind,source=" + source + ",target=" + target
	if readonly {
		spec += ",readonly"
	}
	return spec
}

// hostUser runs a container as the current user on Unix hosts, so the files it
// writes into test directories can be removed afterwards. Docker Desktop on
// Windows maps ownership itself.
func hostUser() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	return []string{"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())}
}

// nonRootUser is a uid other than root, preferring the host's own.
func nonRootUser() string {
	if runtime.GOOS != "windows" && os.Getuid() > 0 {
		return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	}
	return "1000:1000"
}

// copyFixture copies a generated fixture, including its .git directory, so a
// test may let a container write to it.
func copyFixture(t *testing.T, name, parent string) string {
	t.Helper()
	src := filepath.Join(fixtures, name)
	dst := filepath.Join(parent, name)
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying fixture %s: %v", name, err)
	}
	return dst
}

// snapshot lists every path below root, with the size, modification time and
// mode of each file, so a created, removed or modified entry changes the
// result. Directories are listed by path only: their modification times drift
// by tens of milliseconds on a Windows host both right after copying and when
// Docker Desktop mounts the folder, with nothing written by the tool.
func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			entries = append(entries, filepath.ToSlash(rel)+"/")
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("%s %d %d %s", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano(), info.Mode()))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	return entries
}

// startServer runs the image in server mode with source mounted at /repos,
// published on a free host loopback port, and returns its URL and container ID.
func startServer(t *testing.T, source string, readonly bool) (string, string) {
	t.Helper()
	out, err := docker("run", "-d", "--publish", "127.0.0.1::8080", "--mount", mount(source, "/repos", readonly),
		image, "serve", "--listen", "0.0.0.0:8080", "--allowed-root", "/repos")
	must(t, err, out)
	lines := strings.Split(out, "\n")
	id := strings.TrimSpace(lines[len(lines)-1])
	t.Cleanup(func() { docker("rm", "-f", id) })

	port, err := docker("port", id, "8080/tcp")
	must(t, err, port)
	base := "http://" + strings.TrimSpace(strings.Split(port, "\n")[0])

	deadline := time.Now().Add(30 * time.Second)
	for {
		res, err := http.Get(base + "/")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return base, id
			}
		}
		if time.Now().After(deadline) {
			logs, _ := docker("logs", id)
			t.Fatalf("the server at %s did not answer: %v\n%s", base, err, logs)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 30 * time.Second}
}

func get(t *testing.T, client *http.Client, url string, want int) string {
	t.Helper()
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		t.Fatalf("GET %s = %d, want %d: %.300s", url, res.StatusCode, want, body)
	}
	return string(body)
}

// analyze runs one job through the documented API and returns its report.
func analyze(t *testing.T, client *http.Client, base, repoPath string) map[string]any {
	t.Helper()
	get(t, client, base+"/api/v1/capabilities", http.StatusOK)
	payload, err := json.Marshal(map[string]any{
		"repoPath": repoPath,
		"options": map[string]any{
			"noBlame": false, "perAuthor": true, "anonymize": false, "allowShallow": false,
			"countMerges": false, "since": "", "until": "",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.Post(base+"/api/v1/jobs", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var created struct {
		ID string `json:"id"`
	}
	if res.StatusCode != http.StatusAccepted || json.Unmarshal(body, &created) != nil {
		t.Fatalf("creating a job for %s = %d: %s", repoPath, res.StatusCode, body)
	}

	deadline := time.Now().Add(2 * time.Minute)
	for {
		var status struct {
			Status string          `json:"status"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(get(t, client, base+"/api/v1/jobs/"+created.ID, http.StatusOK)), &status); err != nil {
			t.Fatal(err)
		}
		if status.Status == "succeeded" {
			break
		}
		if status.Status != "queued" && status.Status != "running" {
			t.Fatalf("the job for %s ended %s: %s", repoPath, status.Status, status.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("the job for %s did not finish", repoPath)
		}
		time.Sleep(200 * time.Millisecond)
	}

	var report map[string]any
	if err := json.Unmarshal([]byte(get(t, client, base+"/api/v1/jobs/"+created.ID+"/report", http.StatusOK)), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

// difference lists the paths at which two decoded JSON values differ.
func difference(path string, a, b any) []string {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: %s != %s", path, compact(a), compact(b))}
		}
		keys := map[string]bool{}
		for key := range av {
			keys[key] = true
		}
		for key := range bv {
			keys[key] = true
		}
		sorted := make([]string, 0, len(keys))
		for key := range keys {
			sorted = append(sorted, key)
		}
		sort.Strings(sorted)
		var out []string
		for _, key := range sorted {
			out = append(out, difference(path+"."+key, av[key], bv[key])...)
		}
		return out
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return []string{fmt.Sprintf("%s: %s != %s", path, compact(a), compact(b))}
		}
		var out []string
		for i := range av {
			out = append(out, difference(fmt.Sprintf("%s[%d]", path, i), av[i], bv[i])...)
		}
		return out
	default:
		if !reflect.DeepEqual(a, b) {
			return []string{fmt.Sprintf("%s: %s != %s", path, compact(a), compact(b))}
		}
		return nil
	}
}

func compact(v any) string {
	data, _ := json.Marshal(v)
	if len(data) > 200 {
		return string(data[:200]) + "..."
	}
	return string(data)
}
