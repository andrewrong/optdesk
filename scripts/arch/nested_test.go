package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// nestedModules: y depends on nothing, ot depends on x; w is a row ot does not list.
const nestedModules = "# Fixture\n\n## Modules\n\n" +
	"| module | paths | entry | tests | depends |\n" +
	"| --- | --- | --- | --- | --- |\n" +
	"| x | internal/x/** | | | |\n" +
	"| v | internal/v/** | | | x |\n" +
	"| w | internal/w/** | | | |\n" +
	"| ot | cmd/** | | | x |\n"

func TestTreeNestedPackageIsImportableByOT(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"internal/x/api.go":               "package x\n",
		"internal/x/shared/s.go":          "package shared\n",
		"internal/x/y/z/z.go":             "package z\n",
		"internal/x/y/decl/api.go":        "package decl\n",
		"internal/x/y/kit/k.go":           "package kit\n\nimport _ \"testing\"\n",
		"internal/x/y/onlytest/a_test.go": "package onlytest\n",
		"internal/x/y/nogo/readme.txt":    "x\n",
		"internal/x/y/sub/s.go":           "package sub\n",
		"internal/v/api.go":               "package v\n",
		"internal/v/q/r/r.go":             "package r\n",
		"internal/w/api.go":               "package w\n",
		"internal/w/p/q/q.go":             "package q\n",
		"cmd/ot/main.go":                  "package main\n\nfunc main() {}\n",
	}
	dir := t.TempDir()
	for p, src := range files {
		write(t, filepath.Join(dir, p), src)
	}
	rows, err := parseModules(nestedModules)
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := goDirs(dir)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := discover(dir, dirs, rows)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]pkg{}
	for _, p := range pkgs {
		got[p.dir] = p
	}
	for d, want := range map[string]pkg{
		"internal/x/y/z":    {"internal/x/y/z", kindImpl, "x"},
		"internal/x/y/decl": {"internal/x/y/decl", kindDecl, "x"},
		"internal/x/y/kit":  {"internal/x/y/kit", kindKit, "x"},
		"internal/v/q/r":    {"internal/v/q/r", kindImpl, "v"},
		"internal/w/p/q":    {"internal/w/p/q", kindImpl, "w"},
	} {
		if got[d] != want {
			t.Errorf("discover %s = %v, want %v", d, got[d], want)
		}
	}
	for _, d := range []string{"internal/x/y/onlytest", "internal/x/y/nogo", "internal/x/y"} {
		if _, ok := got[d]; ok {
			t.Errorf("discover returned %s, which has no non-test .go file", d)
		}
	}

	toml, yml, err := generate(nestedModules, dir)
	if err != nil {
		t.Fatal(err)
	}
	lint := func(c string) []string {
		return depsOfYAML(string(yml), c)
	}
	tomlHas := func(from, to string) bool {
		return strings.Contains(mayImportOf(t, string(toml), from), `"`+to+`"`)
	}
	for _, e := range []edge{
		{"ot", "x/y/z", true},    // item 2
		{"ot", "w/p/q", false},   // row outside the depends cell
		{"ot", "x/y/decl", true}, // item 3
		{"v/q/r", "x/y/decl", true},
		{"x/y/sub", "x/y/decl", true}, // item 4
		{"x/y/z", "x/y/decl", true},
		{"x/y/z", "x", true},
		{"x/shared", "x/y/z", false},
		{"v/q/r", "x/y/kit", true},
		{"ot", "x/y/kit", false},
		{"x/y/sub", "x/y/kit", false},
		{"v/q/r", "x/y/z", false},
		{"x/y/sub", "x/y/z", false},
		{"v/q/r", "x", true},
		{"v/q/r", "v", true},
		{"v/q/r", "w", false},
	} {
		if got := slices.Contains(lint(e.from), e.to); got != e.want {
			t.Errorf("yml: %s imports %s = %v, want %v", e.from, e.to, got, e.want)
		}
		// arch.toml lists no test kit: the tree adds those edges to the lint file only.
		if e.from == "ot" && e.to != "x/y/kit" && tomlHas(e.from, e.to) != e.want {
			t.Errorf("toml: %s imports %s = %v, want %v", e.from, e.to, !e.want, e.want)
		}
	}

	// item 5: one component per exact directory, no directory twice.
	ins := map[string]string{}
	for _, c := range componentsOfYAML(string(yml)) {
		if prev, dup := ins[c.dir]; dup {
			t.Errorf("directory %s is the in of %s and %s", c.dir, prev, c.name)
		}
		ins[c.dir] = c.name
	}
	if ins["internal/x/y/z"] != "x/y/z" {
		t.Errorf("in of internal/x/y/z is component %q, want x/y/z", ins["internal/x/y/z"])
	}
}

// componentsOfYAML reads the name and in of each component of a generated lint file.
func componentsOfYAML(yml string) []component {
	_, body, _ := strings.Cut(yml, "\ncomponents:\n")
	body, _, _ = strings.Cut(body, "\ndeps:\n")
	var out []component
	for _, l := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(l, "    in: "):
			out[len(out)-1].dir = strings.TrimPrefix(l, "    in: ")
		case strings.HasPrefix(l, "  ") && strings.HasSuffix(l, ":"):
			out = append(out, component{name: strings.TrimSuffix(strings.TrimSpace(l), ":")})
		}
	}
	return out
}

// depsOfYAML returns the mayDependOn list of a component of a generated lint file.
func depsOfYAML(yml, name string) []string {
	_, deps, _ := strings.Cut(yml, "\ndeps:\n")
	var out []string
	in := false
	for _, l := range strings.Split(deps, "\n") {
		switch {
		case strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "   "):
			in = strings.TrimSuffix(strings.TrimSpace(l), ":") == name
		case in && strings.HasPrefix(strings.TrimSpace(l), "- "):
			out = append(out, strings.TrimPrefix(strings.TrimSpace(l), "- "))
		}
	}
	return out
}

// A nested package only adds: every component of the tree without it keeps its whole
// import list, and ot gains the nested component and nothing else.
func TestTreeNestedPackageKeepsEveryExistingAllowance(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"internal/x/api.go":      "package x\n",
		"internal/x/shared/s.go": "package shared\n",
		"internal/x/m/m.go":      "package m\n",
		"internal/v/api.go":      "package v\n",
		"cmd/ot/main.go":         "package main\n\nfunc main() {}\n",
	}
	build := func(nested bool) []byte {
		dir := t.TempDir()
		for p, src := range base {
			write(t, filepath.Join(dir, p), src)
		}
		if nested {
			write(t, filepath.Join(dir, "internal/x/y/z/z.go"), "package z\n")
		}
		_, yml, err := generate(nestedModules, dir)
		if err != nil {
			t.Fatal(err)
		}
		return yml
	}
	before, after := string(build(false)), string(build(true))
	for _, c := range componentsOfYAML(before) {
		old, now := depsOfYAML(before, c.name), depsOfYAML(after, c.name)
		for _, d := range old {
			if !slices.Contains(now, d) {
				t.Errorf("%s lost the allowance %s", c.name, d)
			}
		}
		for _, d := range now {
			if !slices.Contains(old, d) && (c.name != "ot" || d != "x/y/z") {
				t.Errorf("%s gained the allowance %s", c.name, d)
			}
		}
	}
	if !slices.Contains(depsOfYAML(after, "ot"), "x/y/z") {
		t.Error("ot does not import x/y/z")
	}
}

// On the real tree the nested packages strategy/risk/{bsm,fit,stress} are imported by ot, are
// components of the lint file only, and import what they imported before they were discovered.
func TestTreeRealNestedPackagesChangeOnlyOT(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	toml, yml, err := generate(modulesText(t), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"strategy/risk/bsm", "strategy/risk/fit", "strategy/risk/stress"} {
		if !slices.Contains(depsOfYAML(string(yml), "ot"), n) {
			t.Errorf("ot does not import %s in the lint file", n)
		}
		if !strings.Contains(string(toml), `"`+n+`"`) {
			t.Errorf("arch.toml does not list %s for ot", n)
		}
		if strings.Contains(string(toml), "[component.\""+n+"\"]") {
			t.Errorf("arch.toml has a new component %s", n)
		}
		if slices.Contains(depsOfYAML(string(yml), n), "strategy/shared") {
			t.Errorf("%s gained strategy/shared", n)
		}
	}
}
