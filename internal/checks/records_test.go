package checks

import (
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const (
	decisionsDir   = "docs/decisions/"
	decisionsIndex = decisionsDir + "INDEX.md"
	recordTemplate = decisionsDir + "0000-template.md"
)

// record is one parsed file under docs/decisions/.
type record struct {
	number  int
	path    string
	title   string
	status  string
	content string
}

// TestRecordIntegrity enforces ADR-0001 and ADR-0004 over docs/decisions/: the
// template section set, a complete index, no schedule content in an accepted
// record, and resolvable ADR-NNNN references in every tracked file.
func TestRecordIntegrity(t *testing.T) {
	repo := openRepository(t)
	records := loadRecords(t, repo)

	t.Run("sections", func(t *testing.T) {
		want := headings(repo.read(t, 1, recordTemplate))
		if len(want) == 0 {
			fatal(t, 1, "%s declares no sections", recordTemplate)
		}
		for _, r := range records {
			if got := headings(r.content); strings.Join(got, "|") != strings.Join(want, "|") {
				report(t, 1, "%s has sections %q; the template requires %q in that order", r.path, got, want)
			}
		}
	})

	t.Run("index", func(t *testing.T) {
		checkIndex(t, repo, records)
	})

	t.Run("no schedule", func(t *testing.T) {
		for _, r := range records {
			if r.status != "Accepted" {
				continue
			}
			for i, line := range lines(r.content) {
				for _, p := range schedulePatterns() {
					if m := p.re.FindString(line); m != "" {
						report(t, 4, "%s:%d: accepted record contains %s %q", r.path, i+1, p.what, m)
					}
				}
			}
		}
	})

	t.Run("references resolve", func(t *testing.T) {
		known := map[int]bool{}
		for _, r := range records {
			known[r.number] = true
		}
		reference := regexp.MustCompile(`ADR-([0-9]{4})`)
		for _, file := range repo.tracked {
			content := repo.read(t, 1, file)
			if !isText(content) {
				continue
			}
			for i, line := range lines(content) {
				for _, m := range reference.FindAllStringSubmatch(line, -1) {
					n, _ := strconv.Atoi(m[1])
					if !known[n] {
						report(t, 1, "%s:%d: %s does not resolve to a record in %s", file, i+1, m[0], decisionsDir)
					}
				}
			}
		}
	})
}

func loadRecords(t *testing.T, repo repository) []record {
	t.Helper()
	fileName := regexp.MustCompile(`^([0-9]{4})-[a-z0-9]+(-[a-z0-9]+)*\.md$`)
	title := regexp.MustCompile(`^# ADR-([0-9]{4}): (.+)$`)
	status := regexp.MustCompile(`^\*\*Status:\*\* (Proposed|Accepted|Rejected|Superseded)\s*$`)

	var records []record
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, decisionsDir) || file == decisionsIndex || file == recordTemplate {
			continue
		}
		name := strings.TrimPrefix(file, decisionsDir)
		m := fileName.FindStringSubmatch(name)
		if m == nil {
			report(t, 1, "%s is not named NNNN-kebab-case-title.md", file)
			continue
		}
		r := record{path: file, content: repo.read(t, 1, file)}
		r.number, _ = strconv.Atoi(m[1])
		text := lines(r.content)
		if tm := title.FindStringSubmatch(text[0]); tm == nil || tm[1] != m[1] {
			report(t, 1, "%s: first line must be \"# ADR-%s: <title>\"", file, m[1])
		} else {
			r.title = tm[2]
		}
		for _, line := range text {
			if sm := status.FindStringSubmatch(line); sm != nil {
				r.status = sm[1]
				break
			}
		}
		if r.status == "" {
			report(t, 1, "%s has no **Status:** line with a template status", file)
		}
		records = append(records, r)
	}
	if len(records) == 0 {
		fatal(t, 1, "no records found under %s", decisionsDir)
	}
	return records
}

// headings returns the level-two section headings of a Markdown document.
func headings(content string) []string {
	var out []string
	for _, line := range lines(content) {
		if strings.HasPrefix(line, "## ") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
		}
	}
	return out
}

// checkIndex requires one index row per record, with matching file, title and
// status, and no row without a record (ADR-0001 clause 6).
func checkIndex(t *testing.T, repo repository, records []record) {
	t.Helper()
	row := regexp.MustCompile(`^\| \[([0-9]{4})\]\(([^)]+)\) \| (.+) \| ([A-Za-z]+) \|\s*$`)
	type entry struct{ file, title, status string }
	listed := map[int][]entry{}
	for i, line := range lines(repo.read(t, 1, decisionsIndex)) {
		m := row.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		listed[n] = append(listed[n], entry{file: m[2], title: m[3], status: m[4]})
		if !repo.isTracked(path.Join(decisionsDir, m[2])) {
			report(t, 1, "%s:%d: row %s links to %s, which is not a tracked record", decisionsIndex, i+1, m[1], m[2])
		}
	}
	byNumber := map[int]record{}
	for _, r := range records {
		byNumber[r.number] = r
		entries := listed[r.number]
		switch {
		case len(entries) == 0:
			report(t, 1, "%s does not list %s", decisionsIndex, r.path)
			continue
		case len(entries) > 1:
			report(t, 1, "%s lists ADR-%04d %d times", decisionsIndex, r.number, len(entries))
		}
		e := entries[0]
		if e.file != path.Base(r.path) {
			report(t, 1, "%s links ADR-%04d to %s, but the record is %s", decisionsIndex, r.number, e.file, path.Base(r.path))
		}
		if r.title != "" && e.title != r.title {
			report(t, 1, "%s titles ADR-%04d %q, but the record says %q", decisionsIndex, r.number, e.title, r.title)
		}
		if r.status != "" && e.status != r.status {
			report(t, 1, "%s gives ADR-%04d status %s, but the record says %s", decisionsIndex, r.number, e.status, r.status)
		}
	}
	for n := range listed {
		if _, ok := byNumber[n]; !ok {
			report(t, 1, "%s lists ADR-%04d, which has no record file", decisionsIndex, n)
		}
	}
}

type schedulePattern struct {
	what string
	re   *regexp.Regexp
}

// schedulePatterns are the forms of date, duration, milestone and phase name
// that ADR-0004 clause 2 and ADR-0001 clause 5 exclude from accepted records.
//
// Durations are matched in the units work is scheduled in. Seconds, minutes,
// hours, days and years are not matched, because records state technical
// quantities in those units (a 30-day recency window, a 5-minute gate budget)
// that are not statements about when work will happen. The words "phase" and
// "milestone" alone are not matched either, because records prohibit them by
// name; a phase or milestone name is the word followed by an identifier.
func schedulePatterns() []schedulePattern {
	months := `(January|February|March|April|May|June|July|August|September|October|November|December)`
	return []schedulePattern{
		{"a date", regexp.MustCompile(`\b[0-9]{4}-[0-9]{2}-[0-9]{2}\b`)},
		{"a date", regexp.MustCompile(`\b` + months + `( [0-9]{1,2},?)? (19|20)[0-9]{2}\b`)},
		{"a date", regexp.MustCompile(`\b[0-9]{1,2} ` + months + ` (19|20)[0-9]{2}\b`)},
		{"a date", regexp.MustCompile(`(?i)\b(in|by|until|before|after|since|during|from) (19|20)[0-9]{2}\b`)},
		{"a date", regexp.MustCompile(`\b(Q[1-4]|H[12]) ?(19|20)[0-9]{2}\b`)},
		{"a duration", regexp.MustCompile(`(?i)\b[0-9]+(\.[0-9]+)?[ -]?(weeks?|fortnights?|months?|quarters?|sprints?)\b`)},
		{"a phase name", regexp.MustCompile(`(?i)\bphase [0-9IVX]+(\.[0-9]+)?\b`)},
		{"a milestone name", regexp.MustCompile(`(?i)\bmilestone [0-9]+\b|\bM[0-9]+\b`)},
		{"a sprint name", regexp.MustCompile(`(?i)\bsprint [0-9]+\b`)},
		{"a phase name", regexp.MustCompile(`\bMVP\b`)},
	}
}
