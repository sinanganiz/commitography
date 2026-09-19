package checks

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// The identity checkers enforce ADR-0033 in the internal working layer:
//
//   - clause 3, structurally: a raw address is held only as an
//     identity.Address, in an unexported field, and has no marshalling path
//     (TestIdentityRawAddressUnserialisable);
//   - clause 4: the same address has the same digest in two different
//     repositories (TestIdentityDigestAcrossFixtures);
//   - clause 5: anonymised output replaces every display name with the
//     identity's id, its stable pseudonym, and changes nothing else
//     (TestIdentityAnonymisedChangesOnlyDisplayNames).
//
// The artifacts themselves are scanned by the leak scan in leak_test.go.
//
// Narrowing: ADR-0033 clause 6 is not implemented or checked here. It governs
// public mode, which does not exist until WP-0045 introduces the mode switch
// and the capability matrix; that package implements clause 6 and adds its
// check. Clauses 1 to 5 and 7 are what WP-0009 implements, and clause 6 is not
// satisfied by anything in this file.
//
// The collect stage's commit record, model.Commit, carries the author's
// address as a string with a JSON name. It is the internal layer's own
// persisted history (ADR-0017), which clause 1 lets hold raw addresses, and it
// never reaches an exported artifact; the identity rule below covers the
// identity layer and every field of type identity.Address anywhere in the
// tree, not the commit record.

const identityPackage = "internal/core/identity/"

// addressFieldName is a field name that says it holds an address.
func addressFieldName() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(e-?mail|address)`)
}

// rawAddressFields returns every field in a parsed file that holds a raw
// address and has a path to an encoder: an exported field, or one carrying a
// struct tag. A field holds a raw address when its type is identity.Address,
// or, inside the identity package, when its type is Address or its name says
// it holds one. The Address type's own fields are held to the same rule.
func rawAddressFields(fset *token.FileSet, parsed *ast.File, inIdentity bool) []string {
	var out []string
	name := addressFieldName()
	ast.Inspect(parsed, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		isAddressType := inIdentity && spec.Name.Name == "Address"
		for _, field := range st.Fields.List {
			holds := isAddressType || mentionsAddress(field.Type, inIdentity)
			names := field.Names
			if len(names) == 0 {
				// An embedded field is named after its type, and is exported
				// when the type is.
				names = []*ast.Ident{ast.NewIdent(embeddedName(field.Type))}
			}
			for _, ident := range names {
				if !holds && !(inIdentity && name.MatchString(ident.Name)) {
					continue
				}
				where := fmt.Sprintf("%s:%d %s.%s", fset.Position(field.Pos()).Filename,
					fset.Position(field.Pos()).Line, spec.Name.Name, ident.Name)
				if ast.IsExported(ident.Name) {
					out = append(out, where+" is exported, so an encoder writes it")
				}
				if field.Tag != nil {
					out = append(out, where+" carries the tag "+field.Tag.Value+", a declared marshalling path")
				}
			}
		}
		return true
	})
	return out
}

// mentionsAddress reports whether a type expression involves the Address
// type: directly, or as the element of a slice, array, pointer or map.
func mentionsAddress(expr ast.Expr, inIdentity bool) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := x.X.(*ast.Ident); ok && pkg.Name == "identity" && x.Sel.Name == "Address" {
				found = true
			}
			return false
		case *ast.Ident:
			if inIdentity && x.Name == "Address" {
				found = true
			}
		}
		return true
	})
	return found
}

func embeddedName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.StarExpr:
		return embeddedName(x.X)
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.Ident:
		return x.Name
	}
	return "_"
}

// serialisationLeaks passes each value through every encoder in the standard
// library and through fmt, and returns every form in which the probe address
// survives. An encoder refusing the value is not a leak.
func serialisationLeaks(probe string, values map[string]any) []string {
	var out []string
	for what, value := range values {
		forms := map[string]func() ([]byte, error){
			"encoding/json": func() ([]byte, error) { return json.Marshal(value) },
			"encoding/xml":  func() ([]byte, error) { return xml.Marshal(value) },
			"encoding/gob": func() ([]byte, error) {
				var buf bytes.Buffer
				err := gob.NewEncoder(&buf).Encode(value)
				return buf.Bytes(), err
			},
		}
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			forms["fmt "+verb] = func() ([]byte, error) { return []byte(fmt.Sprintf(verb, value)), nil }
		}
		for form, encode := range forms {
			// A panic inside an encoder is a refusal too; it is recovered so
			// one value cannot hide the rest.
			data, err := func() (data []byte, err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panicked: %v", r)
					}
				}()
				return encode()
			}()
			if err == nil && bytes.Contains(bytes.ToLower(data), []byte(probe)) {
				out = append(out, fmt.Sprintf("%s writes the address of %s", form, what))
			}
		}
	}
	return out
}

// identityValues returns every value the identity layer hands out, built over
// commits whose author carries probe, together with what the pipeline derives
// from them.
func identityValues(probe string) map[string]any {
	commits := []model.Commit{{
		Hash: "a", AuthorName: "Someone", AuthorEmail: probe,
		AuthorDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}}
	cfg := config.Default()
	cfg.Identities = []config.Identity{{Name: "Someone", Emails: []string{probe, "other." + probe}}}
	resolver := identity.NewResolver(cfg, commits)
	all := resolver.Identities()
	return map[string]any{
		"an address":                       identity.ParseAddress(probe),
		"an address in an exported field":  struct{ A identity.Address }{identity.ParseAddress(probe)},
		"a resolved identity":              all[0],
		"the resolved identities":          all,
		"the resolver":                     resolver,
		"the resolver by value":            *resolver,
		"the input the families read from": core.Input{Resolver: resolver},
	}
}

// TestIdentityRawAddressUnserialisable enforces ADR-0033 clause 3 in the
// internal layer, where the address exists: no field holding a raw address is
// exported or tagged, and no value the identity layer hands out writes the
// address through any encoder or through fmt.
func TestIdentityRawAddressUnserialisable(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fset := token.NewFileSet()
	scanned, identityFiles := 0, 0
	for _, file := range repo.tracked {
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") ||
			!(strings.HasPrefix(file, "internal/") || strings.HasPrefix(file, "cmd/")) {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 33, file), parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 33, "parsing %s: %v", file, err)
		}
		inIdentity := strings.HasPrefix(file, identityPackage)
		if inIdentity {
			identityFiles++
		}
		scanned++
		for _, finding := range rawAddressFields(fset, parsed, inIdentity) {
			report(t, 33, "%s; a raw address stays in the internal layer with no marshalling path (clause 3)",
				finding)
		}
	}
	if identityFiles == 0 || scanned == 0 {
		fatal(t, 64, "no file of %s was scanned; the checker would pass vacuously", identityPackage)
	}

	const probe = "probe.address@example.com"
	for _, leak := range serialisationLeaks(probe, identityValues(probe)) {
		report(t, 33, "%s; a raw address has no serialised form (clause 3)", leak)
	}
}

// TestIdentityRawAddressRejectsAMarshallingPath is the failure demonstration
// ADR-0064 clause 6 requires: a raw address field that gains a marshalling
// path, in each of the ways one can, fails the checker.
func TestIdentityRawAddressRejectsAMarshallingPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		inIdentity   bool
	}{
		{"an exported address field", "package identity\n\ntype Identity struct{ Email string }\n", true},
		{"a tagged address field", "package identity\n\ntype Identity struct{ email string `json:\"email\"` }\n", true},
		{"an exported Address", "package identity\n\ntype Identity struct{ Canonical Address }\n", true},
		{"an exported field in Address", "package identity\n\ntype Address struct{ Value string }\n", true},
		{"an exported identity.Address elsewhere", "package server\n\ntype Body struct{ From []identity.Address }\n", false},
		{"an embedded identity.Address elsewhere", "package server\n\ntype Body struct{ identity.Address }\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, "violation.go", tc.source, 0)
			if err != nil {
				fatal(t, 64, "parsing the violation: %v", err)
			}
			if len(rawAddressFields(fset, parsed, tc.inIdentity)) == 0 {
				report(t, 64, "the checker accepted %s, so it cannot catch one", tc.name)
			}
		})
	}

	permitted := "package identity\n\ntype Identity struct{ Digest string; addresses []Address }\n"
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "permitted.go", permitted, 0)
	if err != nil {
		fatal(t, 64, "parsing the permitted form: %v", err)
	}
	if found := rawAddressFields(fset, parsed, true); len(found) != 0 {
		report(t, 33, "the checker refused the permitted form: %v", found)
	}

	// The runtime half: a value whose address is reachable fails.
	const probe = "probe.address@example.com"
	leaky := struct{ Email string }{probe}
	if len(serialisationLeaks(probe, map[string]any{"a leaky value": leaky})) == 0 {
		report(t, 64, "the runtime probe accepted a value that serialises its address, so it cannot catch one")
	}
	if got := reflect.TypeOf(identity.Address{}).NumField(); got == 0 {
		fatal(t, 64, "identity.Address holds nothing, so the probe proves nothing about it")
	}
}

// TestIdentityDigestAcrossFixtures enforces ADR-0033 clause 4: an address
// shared by two different fixture repositories has the same id in both
// reports, and it is the digest docs/metrics.md section 14 defines.
func TestIdentityDigestAcrossFixtures(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	// Ada commits as ada@example.com in both, in histories that are otherwise
	// different repositories with different commits.
	const shared = "ada@example.com"
	want := identity.Digest(shared)
	if want != core.IdentityDigest(shared) {
		fatal(t, 33, "the report's digest and the identity layer's digest differ for one address")
	}

	seen := map[string]string{}
	for _, fixture := range []string{"basic", "merges"} {
		content, refused := produce(t, repo, fixture)
		if refused {
			fatal(t, 64, "fixture %s was refused, so it has no identities to compare", fixture)
		}
		var document struct {
			Repository struct {
				Commit string `json:"commit"`
			} `json:"repository"`
			Identities []core.IdentityEntry `json:"identities"`
		}
		if err := json.Unmarshal([]byte(content), &document); err != nil {
			fatal(t, 33, "fixture %s: reading the report: %v", fixture, err)
		}
		found := false
		for _, entry := range document.Identities {
			if entry.ID == want {
				found = true
			}
		}
		if !found {
			report(t, 33, "fixture %s has no identity with the digest %s of the address it shares with the "+
				"other fixture; the same address must have the same digest in every repository", fixture, want)
		}
		seen[fixture] = document.Repository.Commit
	}
	if seen["basic"] == seen["merges"] {
		fatal(t, 64, "the two fixtures analysed the same commit, so they are not two repositories")
	}
}

// anonymisationDifferences compares a report with its anonymised counterpart
// under ADR-0033 clause 5: every individual identity's display name is
// replaced by its id, the stable pseudonym, and nothing else in the document
// differs, the aggregate entry's name included.
func anonymisationDifferences(plain, anonymised string) []string {
	var a, b map[string]any
	if err := json.Unmarshal([]byte(plain), &a); err != nil {
		return []string{"the report is not JSON: " + err.Error()}
	}
	if err := json.Unmarshal([]byte(anonymised), &b); err != nil {
		return []string{"the anonymised report is not JSON: " + err.Error()}
	}
	var out []string
	plainIdentities, _ := a["identities"].([]any)
	anonymisedIdentities, _ := b["identities"].([]any)
	if len(plainIdentities) != len(anonymisedIdentities) {
		return []string{fmt.Sprintf("anonymising changed the number of identities from %d to %d",
			len(plainIdentities), len(anonymisedIdentities))}
	}
	for i := range plainIdentities {
		p, _ := plainIdentities[i].(map[string]any)
		q, _ := anonymisedIdentities[i].(map[string]any)
		if p == nil || q == nil {
			return []string{fmt.Sprintf("identity %d is not an object", i)}
		}
		if q["aggregate"] != true {
			if q["display_name"] != q["id"] {
				out = append(out, fmt.Sprintf("identity %d is named %v under anonymised output, not its id %v",
					i, q["display_name"], q["id"]))
			}
			// The names are compared through the pseudonym, so what remains is
			// everything else.
			p["display_name"], q["display_name"] = nil, nil
		}
	}
	delete(a, "metadata")
	delete(b, "metadata")
	if !reflect.DeepEqual(a, b) {
		for key := range a {
			if !reflect.DeepEqual(a[key], b[key]) {
				out = append(out, "anonymising changed "+key+", which carries no display name")
			}
		}
		if len(out) == 0 {
			out = append(out, "anonymising changed the document beyond the display names")
		}
	}
	return out
}

// TestIdentityAnonymisedChangesOnlyDisplayNames applies anonymisationDifferences
// to every fixture's report.
func TestIdentityAnonymisedChangesOnlyDisplayNames(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fixtures := generatedFixtures(t, repo)
	compared := 0
	for _, fixture := range fixtures {
		plain, refused := produce(t, repo, fixture)
		if refused {
			continue
		}
		anonymised, _ := produceWith(t, repo, fixture, true)
		compared++
		for _, difference := range anonymisationDifferences(plain, anonymised) {
			report(t, 33, "fixture %s: %s; anonymised output replaces display names with stable pseudonyms "+
				"and changes nothing else (clause 5)", fixture, difference)
		}
		if plain == anonymised && strings.Contains(plain, `"display_name"`) {
			report(t, 33, "fixture %s: anonymised output is identical to the plain report, so no name was replaced",
				fixture)
		}
	}
	if compared == 0 {
		fatal(t, 64, "no fixture produced a report, so anonymised output was not compared")
	}
}

// TestIdentityAnonymisedRejectsAChange is the failure demonstration for the
// check above: a name left in place, and a value other than a name changed,
// each fail it.
func TestIdentityAnonymisedRejectsAChange(t *testing.T) {
	t.Parallel()
	const plain = `{"metadata":{"generated_at":"a"},"identities":[` +
		`{"id":"0123456789abcdef","display_name":"Ada","commit_count":2},` +
		`{"display_name":"3 other identities","commit_count":4,"aggregate":true}],"families":{}}`
	for name, anonymised := range map[string]string{
		"a name left in place": `{"metadata":{"generated_at":"b"},"identities":[` +
			`{"id":"0123456789abcdef","display_name":"Ada","commit_count":2},` +
			`{"display_name":"3 other identities","commit_count":4,"aggregate":true}],"families":{}}`,
		"a count changed": `{"metadata":{"generated_at":"b"},"identities":[` +
			`{"id":"0123456789abcdef","display_name":"0123456789abcdef","commit_count":3},` +
			`{"display_name":"3 other identities","commit_count":4,"aggregate":true}],"families":{}}`,
		"a family changed": `{"metadata":{"generated_at":"b"},"identities":[` +
			`{"id":"0123456789abcdef","display_name":"0123456789abcdef","commit_count":2},` +
			`{"display_name":"3 other identities","commit_count":4,"aggregate":true}],"families":{"x":1}}`,
	} {
		if len(anonymisationDifferences(plain, anonymised)) == 0 {
			report(t, 64, "the anonymisation check accepted %s, so it cannot catch one", name)
		}
	}
	permitted := `{"metadata":{"generated_at":"b"},"identities":[` +
		`{"id":"0123456789abcdef","display_name":"0123456789abcdef","commit_count":2},` +
		`{"display_name":"3 other identities","commit_count":4,"aggregate":true}],"families":{}}`
	if found := anonymisationDifferences(plain, permitted); len(found) != 0 {
		report(t, 33, "the anonymisation check refused a correct anonymised report: %v", found)
	}
}
