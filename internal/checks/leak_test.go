package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

// The leak scan (ADR-0063 table 2), applying ADR-0067 clause 6: one scan, two
// rules, chosen by where the text is going.
//
//   - An artifact that can leave the machine — the report, an API response —
//     carries no path, no address and no hostname. Clause 2 admits no
//     exception.
//   - An interactive diagnostic carries no address and no hostname either, and
//     may name a path only in the form the operator supplied in this
//     invocation. A resolved form is a leak; so is a path that arrived in a
//     request.
//
// A single pattern over both destinations would be wrong in one direction: too
// strict for a diagnostic, which must be able to say which path was rejected,
// or too loose for an artifact, which must say nothing at all.
//
// What is covered is the report, every API response and every warning under
// the artifact rule, and under the diagnostic rule the refusals the command
// prints and the log: the progress and warning lines the injected logger
// writes (TestLeakScanLog). The command's stage names are its own words and
// carry no value from the run; the lines it adds that do are the output path,
// which the log test drives in the form the operator types.
//
// Beside the generic shapes, every scan of an analysis forbids the exact
// addresses the analysed history records, before and after .mailmap
// (fixtureAuthors). An address the email pattern does not recognise, such as
// one without a dotted domain, is still an address the artifact must not
// carry. The report and the API are scanned under anonymised output too, where
// the author names the history records are forbidden as well (ADR-0033
// clause 5). Exported images do not exist yet; WP-0057 brings them under this
// scan.

// machineValues are the strings that identify this machine: the roots the run
// works under, the operator's home, the temporary directory, and the host's
// own name. A resolved path leaking into an artifact contains one of them,
// which is what makes the rule checkable without guessing whether a string in
// repository content happens to look like a path.
func machineValues(t *testing.T, repo repository, extra ...string) []string {
	t.Helper()
	var out []string
	add := func(value string) {
		if len(value) < 3 {
			return
		}
		for _, spelling := range []string{
			value,
			filepath.ToSlash(value),
			strings.ReplaceAll(value, `\`, `\\`),
		} {
			out = append(out, spelling)
		}
	}
	add(repo.root)
	add(filepath.Join(repo.root, "testdata", "fixtures"))
	if home, err := os.UserHomeDir(); err == nil {
		add(home)
	}
	add(os.TempDir())
	if host, err := os.Hostname(); err == nil && len(host) > 3 {
		out = append(out, host)
		if short, _, found := strings.Cut(host, "."); found && len(short) > 3 {
			out = append(out, short)
		}
	}
	for _, value := range extra {
		add(value)
	}
	return out
}

// emailPattern and machinePathPattern are the generic shapes ADR-0033's
// acceptance criteria name. The path pattern covers a drive letter, a UNC name
// and the conventional absolute roots of a user's machine; a repository's own
// paths are relative and match none of them.
func emailPattern() *regexp.Regexp {
	return regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
}

// A drive letter has to be a letter on its own. Without the leading class, a
// remedy ending "Fix it with:" followed by a newline reads as one once the
// newline is JSON-escaped.
func machinePathPattern() *regexp.Regexp {
	return regexp.MustCompile(
		`(?i)((^|[^A-Za-z0-9_])[A-Za-z]:[\\/])|(\\\\[A-Za-z0-9._-]+\\)|(/(home|users|root|tmp|var|private)/)`)
}

// scanArtifact applies ADR-0067 clause 2: nothing that names this machine.
func scanArtifact(t *testing.T, what, content string, forbidden []string) {
	t.Helper()
	for _, value := range forbidden {
		if strings.Contains(content, value) {
			report(t, 67, "%s contains %q; an artifact that can leave the machine carries no path, "+
				"address or hostname (clause 2)", what, value)
		}
	}
	if m := emailPattern().FindString(content); m != "" {
		report(t, 33, "%s contains the address %q; no exported artifact carries a raw email address", what, m)
	}
	if m := machinePathPattern().FindString(content); m != "" {
		report(t, 67, "%s contains %q, which is part of an absolute local path; an artifact carries none "+
			"(clause 2)", what, m)
	}
}

// scanDiagnostic applies ADR-0067 clauses 3 and 4: no address and no hostname
// at all, and a path only in the form the operator supplied.
func scanDiagnostic(t *testing.T, what, content string, supplied []string, forbidden []string) {
	t.Helper()
	// A supplied path is permitted, so it is removed before the rest is judged.
	// Removing it is also what makes the check strict: whatever absolute path
	// remains is one the operator did not type.
	residue := content
	for _, path := range supplied {
		residue = strings.ReplaceAll(residue, path, "<supplied>")
	}
	for _, value := range forbidden {
		if strings.Contains(residue, value) {
			report(t, 67, "%s names %q, which the operator did not supply in this invocation; "+
				"a diagnostic repeats only the form that was typed (clauses 3 and 5)", what, value)
		}
	}
	if m := emailPattern().FindString(residue); m != "" {
		report(t, 67, "%s contains the address %q; no diagnostic carries an address or a hostname "+
			"under any circumstance (clause 4)", what, m)
	}
	if m := machinePathPattern().FindString(residue); m != "" {
		report(t, 67, "%s contains %q, part of an absolute path the operator did not supply (clause 3)",
			what, m)
	}
}

// fixtureAuthors returns what a fixture's history records about its authors:
// every address, before and after .mailmap, and every name. These are the raw
// identities of ADR-0033 clause 1, which no artifact carries; the names are
// forbidden only where anonymised output was requested. Values shorter than
// four characters are left out, because they would match unrelated text. A
// fixture whose history cannot be read, such as one with no commit, records
// nothing, and a caller that needs a value checks for one.
func fixtureAuthors(t *testing.T, dir string) (addresses, names []string) {
	t.Helper()
	history, err := newCollector().Collect(collect.Options{RepoPath: dir, UseMailmap: true})
	if err != nil {
		return nil, nil
	}
	seen := map[string]bool{}
	add := func(list *[]string, value string) {
		for _, form := range []string{strings.TrimSpace(value), strings.ToLower(strings.TrimSpace(value))} {
			if len(form) >= 4 && !seen[form] {
				seen[form] = true
				*list = append(*list, form)
			}
		}
	}
	for _, c := range history.Commits {
		add(&addresses, c.AuthorEmail)
		add(&addresses, c.AuthorSourceEmail)
		add(&names, c.AuthorName)
	}
	return addresses, names
}

// TestLeakScanReport applies the artifact rule to the report of every fixture,
// which is the artifact ADR-0021 makes the product's contract, with and
// without anonymised output.
func TestLeakScanReport(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	forbidden := machineValues(t, repo)
	fixtures := generatedFixtures(t, repo)
	if len(fixtures) == 0 {
		fatal(t, 64, "no fixture was generated; the gates generate them with `make fixtures`")
	}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			content, refused := produce(t, repo, fixture)
			if refused {
				// A refusal is a diagnostic, not an artifact; the test below
				// judges it under the other rule.
				return
			}
			addresses, names := fixtureAuthors(t, filepath.Join(repo.root, "testdata", "fixtures", fixture))
			if len(addresses) == 0 {
				fatal(t, 64, "the history of fixture %s records no address, so the scan for one proves nothing", fixture)
			}
			withAddresses := append(append([]string(nil), forbidden...), addresses...)
			scanArtifact(t, "the report of fixture "+fixture, content, withAddresses)

			anonymised, _ := produceWith(t, repo, fixture, true)
			scanArtifact(t, "the anonymised report of fixture "+fixture, anonymised,
				append(append([]string(nil), withAddresses...), names...))
		})
	}
}

// TestLeakScanWarnings applies the artifact rule to every warning an analysis
// raises.
//
// A warning is prose written by whichever stage raised it, and it travels
// further than the terminal it was written for: in server mode every warning
// enters the job status response. That makes a warning an artifact whatever raised it, and the
// likeliest place for a resolved path to reach one. The configuration warning
// did exactly that until this package, naming the file it had resolved.
func TestLeakScanWarnings(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	forbidden := machineValues(t, repo)

	// An unknown key is the configuration warning reachable without a malformed
	// repository. The file is given by an absolute path, which the command line
	// may carry and an artifact may not.
	config := filepath.Join(t.TempDir(), "explicit.yml")
	if err := os.WriteFile(config, []byte("unknown_key: true\ntheme: default\n"), 0o600); err != nil {
		fatal(t, 67, "writing a configuration file: %v", err)
	}
	var raised []string
	result, err := newAnalyzer().Run(context.Background(), pipeline.Options{
		RepoPath:         filepath.Join(repo.root, "testdata", "fixtures", "basic"),
		ConfigPath:       config,
		OperatorSupplied: true,
		OnWarning:        func(message string) { raised = append(raised, message) },
	}, nil)
	if err != nil {
		fatal(t, 67, "analysing the basic fixture with an explicit configuration: %v", err)
	}
	if len(raised) == 0 {
		fatal(t, 64, "the unknown configuration key raised no warning, so there is nothing to scan")
	}
	forbidden = append(forbidden, config, filepath.ToSlash(config))
	for _, warning := range append(raised, result.Warnings...) {
		scanArtifact(t, "the warning "+warning, warning, forbidden)
	}
}

// TestLeakScanDiagnostics applies the diagnostic rule to the refusals the
// command prints. The supplied path is what the operator typed, and the scan
// removes it before judging the rest, so a resolved form still fails.
func TestLeakScanDiagnostics(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	root := filepath.Join(repo.root, "testdata", "fixtures")
	forbidden := machineValues(t, repo)

	for _, fixture := range []string{"shallow", "empty"} {
		t.Run(fixture, func(t *testing.T) {
			dir := filepath.Join(root, fixture)
			if _, err := os.Stat(dir); err != nil {
				fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
			}
			// The analysis is driven with a relative path, as an operator
			// working in a shell would. That is what gives the rule something
			// to catch: the supplied form contains nothing machine-specific, so
			// any absolute path in the diagnostic is one the product resolved.
			// Driving it with an absolute path would make the check vacuous,
			// because the leak and the permitted value would be the same string.
			supplied := filepath.Join("..", "..", "testdata", "fixtures", fixture)
			_, err := newAnalyzer().Run(context.Background(),
				pipeline.Options{RepoPath: supplied, OperatorSupplied: true}, nil)
			if err == nil {
				fatal(t, 67, "fixture %s was not refused, so it produces no diagnostic to scan", fixture)
			}
			scanDiagnostic(t, "the refusal of fixture "+fixture, err.Error(),
				[]string{supplied, filepath.ToSlash(supplied)}, forbidden)
		})
	}
}

// TestLeakScanLog applies the diagnostic rule to log output: everything the
// injected logger writes during an analysis of each fixture, which is the
// progress and warning output the command prints on standard error.
//
// The analysis is wired to the logger the way composeRun and Run wire it:
// every progress event becomes a stage line carrying its detail, every warning
// a warning line, and the run closes with the output path the operator gave.
// The command also names each stage in words of its own, which carry no value
// from the run; the checker writes the stage identifier in their place.
//
// The repository, the output directory and the configuration file are given
// in the forms an operator types, so what the rule permits is exactly those,
// and any other path in the log is one the product resolved or discovered.
func TestLeakScanLog(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	forbidden := machineValues(t, repo)
	fixtures := generatedFixtures(t, repo)
	if len(fixtures) == 0 {
		fatal(t, 64, "no fixture was generated; the gates generate them with `make fixtures`")
	}

	// An unknown key raises a configuration warning on every fixture, so each
	// log carries a warning line as well as progress.
	config := filepath.Join(t.TempDir(), "explicit.yml")
	if err := os.WriteFile(config, []byte("unknown_key: true\ntheme: default\n"), 0o600); err != nil {
		fatal(t, 67, "writing a configuration file: %v", err)
	}
	output := filepath.Join("out", "report.json")

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			supplied := filepath.Join("..", "..", "testdata", "fixtures", fixture)
			addresses, _ := fixtureAuthors(t, filepath.Join(repo.root, "testdata", "fixtures", fixture))
			forbidden := append(append([]string(nil), forbidden...), addresses...)
			var log bytes.Buffer
			logger := core.NewLogger(&log, core.FixedClock(checkTime()), core.LoggerOptions{Verbose: true})
			_, err := newAnalyzer().Run(context.Background(), pipeline.Options{
				RepoPath:         supplied,
				ConfigPath:       config,
				OperatorSupplied: true,
				NoBlame:          true,
				OnWarning:        func(message string) { logger.Warn("%s", message) },
			}, func(event pipeline.ProgressEvent) { logger.Stage(event.Stage, event.Detail) })
			if err == nil {
				logger.Stage("Rendering", output)
				logger.Done(output)
			}
			if log.Len() == 0 {
				fatal(t, 64, "the analysis of fixture %s wrote nothing to the log, so there is nothing to scan", fixture)
			}
			scanDiagnostic(t, "the log of fixture "+fixture, log.String(),
				[]string{supplied, filepath.ToSlash(supplied), config, filepath.ToSlash(config), output}, forbidden)
		})
	}
}

// TestLeakScanAPIResponses applies the artifact rule to every response the
// local API produces, including each refusal, because an API response is an
// artifact whatever its status (ADR-0067 clause 2).
func TestLeakScanAPIResponses(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	root := filepath.Join(repo.root, "testdata", "fixtures")
	if _, err := os.Stat(filepath.Join(root, "basic")); err != nil {
		fatal(t, 64, "the basic fixture is missing; the gates generate it with `make fixtures`")
	}
	outside := t.TempDir()
	addresses, names := fixtureAuthors(t, filepath.Join(root, "basic"))
	if len(addresses) == 0 || len(names) == 0 {
		fatal(t, 64, "the history of the basic fixture records no author, so the scan for one proves nothing")
	}
	forbidden := append(machineValues(t, repo, outside), addresses...)

	app, err := newApp([]string{root})
	if err != nil {
		fatal(t, 67, "building the local application: %v", err)
	}
	handler := app.Handler()

	// The capabilities route issues the session cookie, so every later request
	// is authorised without reaching into the package.
	cookie := ""
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		var reader *strings.Reader
		if body != "" {
			reader = strings.NewReader(body)
		} else {
			reader = strings.NewReader("")
		}
		request := httptest.NewRequest(method, path, reader)
		request.Host = "127.0.0.1:8080"
		if cookie != "" {
			request.Header.Set("Cookie", cookie)
		}
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if set := response.Header().Get("Set-Cookie"); set != "" {
			cookie, _, _ = strings.Cut(set, ";")
		}
		scanArtifact(t, fmt.Sprintf("the response to %s %s", method, path), response.Body.String(), forbidden)
		return response
	}

	call(http.MethodGet, "/api/v1/capabilities", "")
	call(http.MethodGet, "/api/v1/jobs", "")
	call(http.MethodGet, "/", "")
	call(http.MethodGet, "/api/v1/jobs/unknown", "")
	call(http.MethodGet, "/api/v1/jobs/unknown/report", "")
	call(http.MethodPost, "/api/v1/jobs", "not json")
	call(http.MethodPost, "/api/v1/jobs", `{"repoPath":""}`)

	// Each refusal below is given an absolute path, which the response must not
	// repeat: it arrived in a request, so no destination may name it.
	for _, repoPath := range []string{
		outside,
		filepath.Join(outside, "absent"),
		filepath.Join(root, "shallow"),
		filepath.Join(root, "empty"),
	} {
		body, err := json.Marshal(map[string]any{"repoPath": repoPath})
		if err != nil {
			fatal(t, 67, "encoding a request: %v", err)
		}
		call(http.MethodPost, "/api/v1/jobs", string(body))
	}

	// A successful analysis, its status polled to a terminal state, and its
	// report: the largest responses and the ones carrying analysis output.
	// It returns the job's path once the analysis has succeeded.
	analyse := func(request map[string]any) string {
		t.Helper()
		body, err := json.Marshal(request)
		if err != nil {
			fatal(t, 67, "encoding a request: %v", err)
		}
		created := call(http.MethodPost, "/api/v1/jobs", string(body))
		if created.Code != http.StatusAccepted {
			fatal(t, 67, "starting an analysis of the basic fixture: %d %s", created.Code, created.Body.String())
		}
		var start struct{ ID string }
		if err := json.Unmarshal(created.Body.Bytes(), &start); err != nil || start.ID == "" {
			fatal(t, 67, "reading the created job's identifier: %v", err)
		}

		// The poll is bounded by a count rather than by a deadline, so that
		// the checker reads no clock (ADR-0042) and needs no exclusion for
		// doing so.
		const (
			pollInterval = 50 * time.Millisecond
			maxPolls     = 1200
		)
		statusPath := "/api/v1/jobs/" + start.ID
		for poll := 0; ; poll++ {
			response := call(http.MethodGet, statusPath, "")
			var status struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
				fatal(t, 67, "reading the job status: %v", err)
			}
			if status.Status != "queued" && status.Status != "running" {
				if status.Status != "succeeded" {
					fatal(t, 67, "the analysis of the basic fixture ended %s: %s", status.Status, response.Body.String())
				}
				return statusPath
			}
			if poll == maxPolls {
				fatal(t, 67, "the analysis of the basic fixture did not finish within %d polls", maxPolls)
			}
			time.Sleep(pollInterval)
		}
	}

	statusPath := analyse(map[string]any{"repoPath": filepath.Join(root, "basic")})
	call(http.MethodGet, statusPath+"/report", "")
	call(http.MethodPost, statusPath+"/cancel", "")
	call(http.MethodDelete, statusPath, "")

	// The same analysis with anonymised output, whose report must not carry
	// the names the history records either (ADR-0033 clause 5).
	anonymisedPath := analyse(map[string]any{
		"repoPath": filepath.Join(root, "basic"),
		"options":  map[string]any{"anonymize": true},
	})
	anonymised := call(http.MethodGet, anonymisedPath+"/report", "")
	if anonymised.Code != http.StatusOK {
		fatal(t, 33, "fetching the anonymised report: %d %s", anonymised.Code, anonymised.Body.String())
	}
	scanArtifact(t, "the anonymised report response", anonymised.Body.String(), names)
	call(http.MethodDelete, anonymisedPath, "")
}

// TestLeakScanRejectsALeakedPath is the failure demonstration ADR-0064
// clause 6 requires, kept as a test rather than performed once by hand: it
// feeds each rule a leak of the kind it exists to catch and requires the rule
// to name it.
func TestLeakScanRejectsALeakedPath(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	forbidden := machineValues(t, repo)

	for _, tc := range []struct {
		name    string
		content string
	}{
		{"a resolved repository path", `{"repository":{"name":"` + filepath.ToSlash(repo.root) + `"}}`},
		{"a home directory", `{"warnings":["could not read /home/someone/.gitconfig"]}`},
		{"a drive letter", `{"warnings":["could not read C:\\Users\\someone\\.gitconfig"]}`},
		{"a raw email address", `{"authors":[{"email":"someone@example.com"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := &testing.T{}
			scanArtifact(inner, "a deliberately leaked artifact", tc.content, forbidden)
			if !inner.Failed() {
				report(t, 64, "the artifact rule accepted %s, so it cannot catch one", tc.name)
			}
		})
	}

	// The diagnostic rule permits the supplied path and nothing else, which is
	// the distinction ADR-0067 clause 5 turns on.
	supplied := []string{filepath.Join("testdata", "fixtures", "shallow")}
	permitted := &testing.T{}
	scanDiagnostic(permitted, "a diagnostic naming the supplied path",
		"the repository at "+supplied[0]+" is a shallow clone", supplied, forbidden)
	if permitted.Failed() {
		report(t, 67, "the diagnostic rule refused the path the operator supplied, which clause 3 permits")
	}

	refused := &testing.T{}
	scanDiagnostic(refused, "a diagnostic naming the resolved path",
		"the repository at "+filepath.ToSlash(filepath.Join(repo.root, supplied[0]))+" is a shallow clone",
		supplied, forbidden)
	if !refused.Failed() {
		report(t, 64, "the diagnostic rule accepted a resolved path, so it cannot catch one")
	}

	// The log is judged by the diagnostic rule too, so a warning line that
	// names a resolved path, or an address, fails it.
	for _, warning := range []string{
		"could not read " + filepath.Join(repo.root, supplied[0], ".gitattributes"),
		"skipping the commit by someone@example.com",
	} {
		var log bytes.Buffer
		core.NewLogger(&log, core.FixedClock(checkTime()), core.LoggerOptions{}).Warn("%s", warning)
		leaked := &testing.T{}
		scanDiagnostic(leaked, "a deliberately leaked log", log.String(), supplied, forbidden)
		if !leaked.Failed() {
			report(t, 64, "the diagnostic rule accepted the log line %q, so it cannot catch one", log.String())
		}
	}
}
