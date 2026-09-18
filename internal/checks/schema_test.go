package checks

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

// The report schema checker (WP-0008 clause 8): every golden report validates
// against docs/report-schema.json, the published description of the document
// (ADR-0021 clause 2, ADR-0031). The golden files are what the golden checker
// proves the analysis produces, so validating them validates the output of
// every fixture. It runs in the checks step, which both gates run.
//
// No JSON Schema library is on the dependency allow list (ADR-0049 clause 2),
// so the subset the schema uses is checked directly: $ref, oneOf, type, enum,
// properties, required, additionalProperties and items.

const reportSchema = "docs/report-schema.json"

func loadReportSchema(t *testing.T, repo repository) map[string]any {
	t.Helper()
	if !repo.isTracked(reportSchema) {
		fatal(t, 21, "%s is not tracked", reportSchema)
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(repo.read(t, 21, reportSchema)), &schema); err != nil {
		fatal(t, 21, "%s is not valid JSON: %v", reportSchema, err)
	}
	return schema
}

// TestReportSchemaGolden validates every tracked golden report.
func TestReportSchemaGolden(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	schema := loadReportSchema(t, repo)
	validated := 0
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, goldenDir+"/") || !strings.HasSuffix(file, ".json") {
			continue
		}
		var document any
		if err := json.Unmarshal([]byte(repo.read(t, 21, file)), &document); err != nil {
			report(t, 21, "%s is not valid JSON: %v", file, err)
			continue
		}
		validated++
		if problems := validate(schema, document, "$", schema); len(problems) > 0 {
			report(t, 21, "%s does not validate against %s:\n  %s", file, reportSchema, strings.Join(problems, "\n  "))
		}
	}
	if validated == 0 {
		fatal(t, 64, "no golden report is tracked under %s, so there is nothing to validate", goldenDir)
	}
}

// TestReportSchemaRejectsMalformedReports is the failure demonstration
// ADR-0064 clause 6 requires: each case breaks one rule the document carries,
// and the schema must refuse it.
func TestReportSchemaRejectsMalformedReports(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	schema := loadReportSchema(t, repo)
	valid := repo.read(t, 21, goldenDir+"/basic.json")

	var base map[string]any
	if err := json.Unmarshal([]byte(valid), &base); err != nil {
		fatal(t, 21, "%s/basic.json is not valid JSON: %v", goldenDir, err)
	}
	if problems := validate(schema, base, "$", schema); len(problems) > 0 {
		fatal(t, 21, "the unmodified golden report is refused, so the cases below prove nothing:\n  %s",
			strings.Join(problems, "\n  "))
	}

	families := func(d map[string]any) map[string]any { return d["families"].(map[string]any) }
	family := func(d map[string]any, name string) map[string]any { return families(d)[name].(map[string]any) }
	cases := map[string]func(d map[string]any){
		"the configuration section is absent": func(d map[string]any) { delete(d, "configuration") },
		"a family is absent":                  func(d map[string]any) { delete(families(d), "static-analysis") },
		"a top-level key is invented":         func(d map[string]any) { d["warnings"] = []any{} },
		"a skipped family carries a metric": func(d map[string]any) {
			family(d, "ownership")["metrics"] = map[string]any{"bus_factor": 0.0}
		},
		"a skipped family has no reason": func(d map[string]any) { delete(family(d, "worktype"), "reasons") },
		"a reason code is invented": func(d map[string]any) {
			family(d, "worktype")["reasons"] = []any{"not_measured"}
		},
		"a degraded family has no confidence": func(d map[string]any) {
			f := family(d, "temporal")
			f["status"], f["reasons"] = "degraded", []any{"shallow_clone"}
		},
		"an ok family carries a reason": func(d map[string]any) {
			family(d, "temporal")["reasons"] = []any{"shallow_clone"}
		},
		"a metric is invented": func(d map[string]any) {
			family(d, "temporal")["metrics"].(map[string]any)["hour_weekday_grid"] = []any{}
		},
		"the family version is absent": func(d map[string]any) { delete(family(d, "files"), "version") },
	}
	for name, breakIt := range cases {
		var document map[string]any
		if err := json.Unmarshal([]byte(valid), &document); err != nil {
			fatal(t, 21, "decoding the golden report: %v", err)
		}
		breakIt(document)
		if len(validate(schema, document, "$", schema)) == 0 {
			report(t, 64, "%s accepts a report in which %s", reportSchema, name)
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
