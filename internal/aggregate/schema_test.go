package aggregate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/collect"
	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/filter"
	"github.com/sinanganiz/commitography/internal/identity"
)

// The report is a documented, stable artifact, so the committed schema is part
// of the contract rather than documentation. Validating against it here catches
// a struct change that would silently break an external consumer.
//
// No JSON Schema library is on the permitted dependency list, so the subset the
// schema actually uses is checked directly: $ref, oneOf, type, enum,
// properties, required, additionalProperties and items.

func loadSchema(t *testing.T) map[string]any {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "report-schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return schema
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture %q not built; run `make fixtures`", name)
	}
	return path
}

// buildFromFixture runs the whole pipeline so the validated report is the one
// users actually get, not a hand-written stand-in.
func buildFromFixture(t *testing.T, name string, cfg config.Config, mutate func(*Input)) *Report {
	t.Helper()
	repo := fixturePath(t, name)

	history, err := collect.Collect(collect.Options{RepoPath: repo, UseMailmap: cfg.UseMailmap})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	pf, err := filter.NewPathFilter(cfg, repo)
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	resolver := identity.NewResolver(cfg, history.Commits)

	in := Input{
		RepoPath:   repo,
		Repository: history.Repository,
		Config:     cfg,
		Filtered:   filter.Apply(history.Commits, cfg, resolver, pf),
		Resolver:   resolver,
		PathFilter: pf,
		NoBlame:    true,
	}
	if mutate != nil {
		mutate(&in)
	}

	report, err := Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return report
}

func TestReportValidatesAgainstCommittedSchema(t *testing.T) {
	schema := loadSchema(t)

	anonymized := config.Default()
	anonymized.Anonymize = true

	cases := []struct {
		name    string
		fixture string
		cfg     config.Config
		mutate  func(*Input)
	}{
		{"defaults", "basic", config.Default(), nil},
		{"per-author", "basic", config.Default(), func(in *Input) { in.PerAuthor = true }},
		{"anonymized", "basic", anonymized, func(in *Input) { in.PerAuthor = true }},
		{"with-blame", "basic", config.Default(), func(in *Input) { in.NoBlame = false }},
		{"single-commit", "single", config.Default(), nil},
		{"merges", "merges", config.Default(), nil},
		{"noise", "noise", config.Default(), nil},
		{"bots", "bots", config.Default(), nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			report := buildFromFixture(t, c.fixture, c.cfg, c.mutate)

			data, err := json.Marshal(report)
			if err != nil {
				t.Fatalf("marshalling report: %v", err)
			}
			var decoded any
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("re-decoding report: %v", err)
			}

			if problems := validate(schema, decoded, "$", schema); len(problems) > 0 {
				t.Errorf("report does not validate against docs/report-schema.json:\n  %s",
					strings.Join(problems, "\n  "))
			}
		})
	}
}

func TestSchemaRejectsAMalformedReport(t *testing.T) {
	schema := loadSchema(t)

	// A guard on the validator itself: a schema that accepts anything would
	// make the test above meaningless.
	cases := map[string]string{
		"missing required key": `{"schemaVersion":1}`,
		"wrong type":           `{"schemaVersion":"one"}`,
		"unknown key":          `{"schemaVersion":1,"somethingElse":true}`,
		"wrong enum value":     `{"schemaVersion":99}`,
	}
	for name, body := range cases {
		var decoded any
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatal(err)
		}
		if problems := validate(schema, decoded, "$", schema); len(problems) == 0 {
			t.Errorf("%s: validator accepted %s", name, body)
		}
	}
}

// --- a small JSON Schema subset validator ---------------------------------

func validate(schema map[string]any, value any, path string, root map[string]any) []string {
	if ref, ok := schema["$ref"].(string); ok {
		resolved, err := resolveRef(root, ref)
		if err != nil {
			return []string{fmt.Sprintf("%s: %v", path, err)}
		}
		return validate(resolved, value, path, root)
	}

	var problems []string

	if alternatives, ok := schema["oneOf"].([]any); ok {
		matched := false
		for _, alt := range alternatives {
			sub, ok := alt.(map[string]any)
			if !ok {
				continue
			}
			if len(validate(sub, value, path, root)) == 0 {
				matched = true
				break
			}
		}
		if !matched {
			problems = append(problems, fmt.Sprintf("%s: matches none of the permitted shapes", path))
		}
		return problems
	}

	if declared, ok := schema["type"]; ok && !matchesType(declared, value) {
		return []string{fmt.Sprintf("%s: has type %s, schema requires %v", path, jsonTypeOf(value), declared)}
	}

	if allowed, ok := schema["enum"].([]any); ok {
		found := false
		for _, candidate := range allowed {
			if fmt.Sprint(candidate) == fmt.Sprint(value) {
				found = true
				break
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s: value %v is not one of %v", path, value, allowed))
		}
	}

	switch typed := value.(type) {
	case map[string]any:
		problems = append(problems, validateObject(schema, typed, path, root)...)
	case []any:
		if items, ok := schema["items"].(map[string]any); ok {
			for i, element := range typed {
				problems = append(problems, validate(items, element, fmt.Sprintf("%s[%d]", path, i), root)...)
			}
		}
	}

	return problems
}

func validateObject(schema map[string]any, value map[string]any, path string, root map[string]any) []string {
	var problems []string

	if required, ok := schema["required"].([]any); ok {
		for _, key := range required {
			name, _ := key.(string)
			if _, present := value[name]; !present {
				problems = append(problems, fmt.Sprintf("%s: required key %q is missing", path, name))
			}
		}
	}

	properties, _ := schema["properties"].(map[string]any)
	for key, sub := range value {
		declared, known := properties[key]
		if !known {
			switch extra := schema["additionalProperties"].(type) {
			case bool:
				if !extra {
					problems = append(problems, fmt.Sprintf("%s: key %q is not permitted by the schema", path, key))
				}
			case map[string]any:
				problems = append(problems, validate(extra, sub, path+"."+key, root)...)
			}
			continue
		}
		declaredSchema, ok := declared.(map[string]any)
		if !ok {
			continue
		}
		problems = append(problems, validate(declaredSchema, sub, path+"."+key, root)...)
	}

	return problems
}

func resolveRef(root map[string]any, ref string) (map[string]any, error) {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, fmt.Errorf("unsupported $ref %q", ref)
	}
	defs, ok := root["$defs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema has no $defs")
	}
	target, ok := defs[strings.TrimPrefix(ref, prefix)].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("$ref %q does not resolve", ref)
	}
	return target, nil
}

func matchesType(declared any, value any) bool {
	switch t := declared.(type) {
	case string:
		return matchesSingleType(t, value)
	case []any:
		for _, alternative := range t {
			name, _ := alternative.(string)
			if matchesSingleType(name, value) {
				return true
			}
		}
	}
	return false
}

func matchesSingleType(name string, value any) bool {
	switch name {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		n, ok := value.(float64)
		return ok && n == math.Trunc(n)
	default:
		return false
	}
}

func jsonTypeOf(value any) string {
	switch n := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case float64:
		if n == math.Trunc(n) {
			return "integer"
		}
		return "number"
	default:
		return fmt.Sprintf("%T", value)
	}
}
