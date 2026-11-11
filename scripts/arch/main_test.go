package main

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

func modulesText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "design", "modules.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runArch(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String() + errb.String()
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// fixtureModules is a small modules text of its own: three modules.
func fixtureModules(gammaDepends string) string {
	return "# Fixture\n\n## Modules\n\n" +
		"| module | paths | entry | tests | depends |\n" +
		"| --- | --- | --- | --- | --- |\n" +
		"| alpha | internal/alpha/** | api.go | internal/alpha/api_test.go | |\n" +
		"| beta | internal/beta/** | api.go | internal/beta/api_test.go | alpha |\n" +
		"| gamma | internal/gamma/** | api.go | internal/gamma/api_test.go | " + gammaDepends + " |\n\n" +
		""
}

func TestArchWritesBothFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	modules := filepath.Join(dir, "modules.md")
	write(t, modules, fixtureModules("alpha"))
	if code, out := runArch(t, "--modules", modules, "--root", dir); code != 0 {
		t.Fatalf("generate: exit %d: %s", code, out)
	}
	for _, name := range []string{tomlFile, ymlFile} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || len(b) == 0 {
			t.Fatalf("%s not written: %v", name, err)
		}
	}
}

// otImports returns the import list of component ot in the generated arch.toml.
func otImports(t *testing.T, text string) string {
	t.Helper()
	toml, _, err := generate(text, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return mayImportOf(t, string(toml), "ot")
}

func mayImportOf(t *testing.T, toml, name string) string {
	t.Helper()
	_, rest, ok := strings.Cut(toml, "[component."+`"`+name+`"`+"]\n")
	if !ok {
		t.Fatalf("no component %s in:\n%s", name, toml)
	}
	block, _, _ := strings.Cut(rest, "\n[")
	return block
}

func TestArchOTImportsTheRootsOfItsDependsCell(t *testing.T) {
	t.Parallel()
	all := otImports(t, modulesText(t))
	for _, root := range []string{"contracts", "clock", "sources", "store", "runner", "jobs", "pages", "profile"} {
		if !strings.Contains(all, `"`+root+`"`) {
			t.Errorf("ot lacks root component %s", root)
		}
	}
}

func TestArchProfileHasNoImports(t *testing.T) {
	t.Parallel()
	toml, yml, err := generate(modulesText(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toml), "[component.\"profile\"]\nin = \"internal/profile\"\nmay_import = []\n") {
		t.Fatalf("profile is not generated with an empty import list:\n%s", toml)
	}
	if strings.Contains(string(yml), "\n  profile:\n    mayDependOn") {
		t.Fatal("profile has a mayDependOn entry")
	}
}

// scratch writes a module tree of interface packages plus the given files.
func scratch(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/s\n\ngo 1.24\n")
	pkgs := map[string]string{
		"internal/contracts": "contracts", "internal/clock": "clock", "internal/sources": "sources",
		"internal/store": "store", "internal/runner": "runner", "internal/jobs": "jobs",
		"internal/pages": "pages", "internal/profile": "profile",
		"internal/store/migrate": "migrate", "internal/store/sessionday": "sessionday",
		"cmd/ot": "main",
	}
	for p, name := range pkgs {
		src := "package " + name + "\n"
		if name == "main" {
			src += "\nfunc main() {}\n"
		}
		write(t, filepath.Join(dir, p, "x.go"), src)
	}
	for p, src := range files {
		write(t, filepath.Join(dir, p), src)
	}
	_, yml, err := generate(modulesText(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, ymlFile), string(yml))
	return dir
}

func imports(pkg, path string) string {
	return "package " + pkg + "\n\nimport _ \"example.com/s/" + path + "\"\n"
}

// lint runs the pinned go-arch-lint on a scratch tree and reports whether it passed.
func lint(t *testing.T, dir string) (bool, string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", "tool", "go-arch-lint", "check", "--project-path", dir, "--output-color=false")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	return err == nil, string(out)
}

func TestArchScratchTree(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		file string
		src  string
		pass bool
	}{
		{"ot imports a discovered implementation package", "cmd/ot/main2.go", imports("main", "internal/store/migrate"), true},
		{"ot imports another discovered implementation package", "cmd/ot/main2.go", imports("main", "internal/store/sessionday"), true},
		{"ot imports a package of a module outside its depends cell", "cmd/ot/main2.go", imports("main", "internal/web"), false},
		{"a jobs sub-package imports a store sub-package", "internal/jobs/fred/j.go", imports("fred", "internal/store/sessionday"), false},
		{"a jobs sub-package imports the store root", "internal/jobs/fred/j.go", imports("fred", "internal/store"), true},
		{"profile imports another module", "internal/profile/p.go", imports("profile", "internal/store"), false},
		{"clock imports a module that its row does not list", "internal/clock/c.go", imports("clock", "internal/runner"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := scratch(t, map[string]string{tc.file: tc.src})
			ok, out := lint(t, dir)
			if ok != tc.pass {
				t.Fatalf("pass = %v, want %v:\n%s", ok, tc.pass, out)
			}
		})
	}
}

func TestArchDeriveContractsGenpb(t *testing.T) {
	t.Parallel()
	dir := scratch(t, map[string]string{"internal/contracts/genpb/session.pb.go": "package genpb\n"})
	toml, yml, err := generate(modulesText(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toml), "[component.\"contracts/genpb\"]\nin = \"internal/contracts/genpb\"\nmay_import = []\n") {
		t.Fatalf("contracts/genpb is not derived with an empty import list:\n%s", toml)
	}
	if !strings.Contains(string(yml), "  contracts/genpb:\n    in: internal/contracts/genpb\n") {
		t.Fatalf("no contracts/genpb component in:\n%s", yml)
	}
	_, depsPart, _ := strings.Cut(string(yml), "\ndeps:\n")
	if strings.Contains(depsPart, "  contracts/genpb:\n") {
		t.Fatal("contracts/genpb has a mayDependOn entry")
	}
	if ok, out := lint(t, dir); !ok {
		t.Fatalf("a package under internal/contracts/genpb fails the lint:\n%s", out)
	}
}

func TestArchDeriveContractsGen(t *testing.T) {
	t.Parallel()
	dir := scratch(t, map[string]string{"internal/contracts/gen/g.go": "package gen\n"})
	toml, _, err := generate(modulesText(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toml), "[component.\"contracts/gen\"]\nin = \"internal/contracts/gen\"\nmay_import = []\n") {
		t.Fatalf("contracts/gen is not derived with an empty import list:\n%s", toml)
	}
	_, rest, _ := strings.Cut(string(toml), "[component.\"contracts\"]\n")
	block, _, _ := strings.Cut(rest, "\n[")
	if !strings.Contains(block, "may_import = [\"contracts/gen\"]") {
		t.Fatalf("contracts does not import contracts/gen:\n%s", block)
	}
}

var nextKey = regexp.MustCompile(`\n  [^ ]`)

func TestArchDeriveEntryListedSubPackages(t *testing.T) {
	t.Parallel()
	modules := "# Fixture\n\n## Modules\n\n" +
		"| module | paths | entry | tests | depends |\n" +
		"| --- | --- | --- | --- | --- |\n" +
		"| contracts | internal/contracts/** | internal/contracts/api.go internal/contracts/genpb | internal/contracts/api_test.go | |\n" +
		"| broker | internal/broker/** | internal/broker/api.go | internal/broker/api_test.go | contracts |\n" +
		"| sources | internal/sources/** | internal/sources/api.go | internal/sources/api_test.go | |\n"
	dir := scratch(t, map[string]string{
		"internal/contracts/genpb/g.go":   "package genpb\n",
		"internal/broker/grpcbroker/d.go": "package grpcbroker\n",
		"internal/sources/fred/f.go":      "package fred\n",
		"internal/sources/s.go":           "package sources\n",
		"internal/broker/b.go":            "package broker\n",
	})
	_, yml, err := generate(modules, dir)
	if err != nil {
		t.Fatal(err)
	}
	_, deps, _ := strings.Cut(string(yml), "\ndeps:\n")
	block := func(name string) string {
		_, rest, ok := strings.Cut(deps, "  "+name+":\n")
		if !ok {
			return ""
		}
		return nextKey.Split(rest, 2)[0]
	}
	if b := block("broker/grpcbroker"); !strings.Contains(b, "- contracts/genpb") {
		t.Fatalf("broker/grpcbroker lacks contracts/genpb:\n%s", b)
	}
	if b := block("sources/fred"); strings.Contains(b, "genpb") {
		t.Fatalf("sources/fred imports genpb:\n%s", b)
	}
	if b := block("broker"); strings.Contains(b, "genpb") {
		t.Fatalf("the root broker imports genpb:\n%s", b)
	}
	if b := block("sources"); strings.Contains(b, "genpb") {
		t.Fatalf("the root sources imports genpb:\n%s", b)
	}
}

// declFixture is a modules table whose store row lists a declarations-only package
// (internal/store/bar), a test kit and a generated package in entry cells.
const declFixture = "# Fixture\n\n## Modules\n\n" +
	"| module | paths | entry | tests | depends |\n" +
	"| --- | --- | --- | --- | --- |\n" +
	"| contracts | internal/contracts/** | internal/contracts/api.go internal/contracts/genpb | internal/contracts/api_test.go | |\n" +
	"| compute | internal/compute/** | internal/compute/api.go internal/compute/contracttest " +
	"internal/compute/trackapi | internal/compute/api_test.go | contracts |\n" +
	"| store | internal/store/** | internal/store/api.go internal/store/bar | internal/store/api_test.go | contracts |\n" +
	"| jobs | internal/jobs/** | internal/jobs/api.go | internal/jobs/api_test.go | contracts compute store |\n"

var declTree = map[string]string{
	"internal/contracts/genpb/g.go":      "package genpb\n",
	"internal/compute/contracttest/c.go": "package contracttest\n\nimport _ \"testing\"\n",
	"internal/store/bar/api.go":          "package bar\n",
	"internal/store/barpg/s.go":          "package barpg\n",
	"internal/store/s.go":                "package store\n",
	"internal/jobs/j.go":                 "package jobs\n",
	"internal/jobs/computefigures/c.go":  "package computefigures\n",
	"internal/compute/c.go":              "package compute\n",
	"internal/compute/trackapi/api.go":   "package trackapi\n",
	"internal/contracts/c.go":            "package contracts\n",
}

// mayImport reads the import list of a component from arch.toml.
func mayImport(t *testing.T, toml, name string) string {
	t.Helper()
	_, rest, ok := strings.Cut(toml, "[component."+`"`+name+`"`+"]\n")
	if !ok {
		t.Fatalf("no component %s in:\n%s", name, toml)
	}
	block, _, _ := strings.Cut(rest, "\n[")
	return block
}

// hasDep reports whether a mayDependOn block lists the component.
func hasDep(block, name string) bool {
	for _, l := range strings.Split(block, "\n") {
		if strings.TrimSpace(l) == "- "+name {
			return true
		}
	}
	return false
}

func TestRootImportsListedInterfacePackage(t *testing.T) {
	t.Parallel()
	dir := scratch(t, declTree)
	toml, yml, err := generate(declFixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"jobs", "jobs/computefigures"} {
		if b := mayImport(t, string(toml), name); !strings.Contains(b, `"store/bar"`) {
			t.Fatalf("%s lacks store/bar in arch.toml:\n%s", name, b)
		}
	}
	if b := mayImport(t, string(toml), "store"); strings.Contains(b, "store/bar") {
		t.Fatalf("the root store imports its own declarations package:\n%s", b)
	}
	if b := mayImport(t, string(toml), "compute"); strings.Contains(b, "store/bar") {
		t.Fatalf("compute does not depend on store but imports store/bar:\n%s", b)
	}
	_, deps, _ := strings.Cut(string(yml), "\ndeps:\n")
	_, jobs, _ := strings.Cut(deps, "  jobs:\n")
	if !hasDep(nextKey.Split(jobs, 2)[0], "store/bar") {
		t.Fatalf("the root jobs lacks store/bar in the lint file:\n%s", yml)
	}
}

func TestGeneratedSubPackageStaysSubOnly(t *testing.T) {
	t.Parallel()
	dir := scratch(t, declTree)
	toml, yml, err := generate(declFixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"jobs", "compute", "store", "contracts"} {
		if b := mayImport(t, string(toml), name); strings.Contains(b, "genpb") || strings.Contains(b, "contracttest") {
			t.Fatalf("root %s imports a kit or generated package in arch.toml:\n%s", name, b)
		}
	}
	_, deps, _ := strings.Cut(string(yml), "\ndeps:\n")
	for _, name := range []string{"jobs", "compute", "store", "contracts"} {
		_, rest, ok := strings.Cut(deps, "\n  "+name+":\n")
		if !ok {
			continue
		}
		if b := nextKey.Split(rest, 2)[0]; strings.Contains(b, "genpb") || strings.Contains(b, "contracttest") {
			t.Fatalf("root %s imports a kit or generated package in the lint file:\n%s", name, b)
		}
	}
	if b := mayImport(t, string(toml), "contracts/genpb"); strings.Contains(b, "store/bar") {
		t.Fatalf("a generated package imports store/bar:\n%s", b)
	}
}

func TestSubPackageImportsOwnRowDeclarations(t *testing.T) {
	t.Parallel()
	dir := scratch(t, declTree)
	toml, yml, err := generate(declFixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b := mayImport(t, string(toml), "store/barpg"); !strings.Contains(b, `"store/bar"`) {
		t.Fatalf("store/barpg lacks store/bar in arch.toml:\n%s", b)
	}
	if b := mayImport(t, string(toml), "store/bar"); strings.Contains(b, `"store/bar"`) {
		t.Fatalf("store/bar imports itself:\n%s", b)
	}
	_, deps, _ := strings.Cut(string(yml), "\ndeps:\n")
	_, rest, _ := strings.Cut(deps, "  store/barpg:\n")
	if !hasDep(nextKey.Split(rest, 2)[0], "store/bar") {
		t.Fatalf("store/barpg lacks store/bar in the lint file:\n%s", yml)
	}
	_, rest, _ = strings.Cut(deps, "  store/bar:\n")
	if hasDep(nextKey.Split(rest, 2)[0], "store/bar") {
		t.Fatalf("store/bar imports itself in the lint file:\n%s", yml)
	}
}

// blockFixture is a modules table whose compute row lists internal/compute/blockapi.
const blockFixture = "# Fixture\n\n## Modules\n\n" +
	"| module | paths | entry | tests | depends |\n" +
	"| --- | --- | --- | --- | --- |\n" +
	"| contracts | internal/contracts/** | internal/contracts/api.go | internal/contracts/api_test.go | |\n" +
	"| compute | internal/compute/** | internal/compute/api.go internal/compute/blockapi | internal/compute/api_test.go | contracts |\n" +
	"| pages | internal/pages/** | internal/pages/api.go | internal/pages/api_test.go | contracts compute |\n"

func TestComputeBlockapiIsDeclarationsOnly(t *testing.T) {
	t.Parallel()
	dir := scratch(t, map[string]string{
		"internal/contracts/c.go":          "package contracts\n",
		"internal/compute/c.go":            "package compute\n",
		"internal/compute/blockapi/api.go": "package blockapi\n",
		"internal/compute/pagefig/p.go":    "package pagefig\n",
		"internal/pages/p.go":              "package pages\n",
	})
	toml, _, err := generate(blockFixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pages", "compute/pagefig"} {
		if b := mayImport(t, string(toml), name); !strings.Contains(b, `"compute/blockapi"`) {
			t.Fatalf("%s lacks compute/blockapi in arch.toml:\n%s", name, b)
		}
	}
	if b := mayImport(t, string(toml), "compute"); strings.Contains(b, "compute/blockapi") {
		t.Fatalf("the root compute imports its own declarations package:\n%s", b)
	}
}

// markFixture is a modules table whose store row lists internal/store/mark, the
// declarations-only package of the position marks.
const markFixture = "# Fixture\n\n## Modules\n\n" +
	"| module | paths | entry | tests | depends |\n" +
	"| --- | --- | --- | --- | --- |\n" +
	"| contracts | internal/contracts/** | internal/contracts/api.go | internal/contracts/api_test.go | |\n" +
	"| store | internal/store/** | internal/store/api.go internal/store/mark | internal/store/api_test.go | contracts |\n" +
	"| jobs | internal/jobs/** | internal/jobs/api.go | internal/jobs/api_test.go | contracts store |\n"

func TestRootAndSubPackageImportStoreMark(t *testing.T) {
	t.Parallel()
	dir := scratch(t, map[string]string{
		"internal/contracts/c.go":    "package contracts\n",
		"internal/store/mark/api.go": "package mark\n",
		"internal/store/markpg/s.go": "package markpg\n",
		"internal/store/s.go":        "package store\n",
		"internal/jobs/j.go":         "package jobs\n",
	})
	toml, yml, err := generate(markFixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"jobs", "store/markpg"} {
		if b := mayImport(t, string(toml), name); !strings.Contains(b, `"store/mark"`) {
			t.Fatalf("%s lacks store/mark in arch.toml:\n%s", name, b)
		}
	}
	_, deps, _ := strings.Cut(string(yml), "\ndeps:\n")
	_, jobs, _ := strings.Cut(deps, "  jobs:\n")
	if !hasDep(nextKey.Split(jobs, 2)[0], "store/mark") {
		t.Fatalf("the root jobs lacks store/mark in the lint file:\n%s", yml)
	}
	if b := mayImport(t, string(toml), "store"); strings.Contains(b, "store/mark") {
		t.Fatalf("the root store imports its own declarations package:\n%s", b)
	}
}

func TestComputeTrackapiIsDeclarationsOnlyPackage(t *testing.T) {
	t.Parallel()
	dir := scratch(t, declTree)
	toml, yml, err := generate(declFixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b := mayImport(t, string(toml), "jobs"); !strings.Contains(b, `"compute/trackapi"`) {
		t.Fatalf("jobs depends on compute and lacks compute/trackapi:\n%s", b)
	}
	if b := mayImport(t, string(toml), "contracts"); strings.Contains(b, "compute/trackapi") {
		t.Fatalf("contracts does not depend on compute but imports compute/trackapi:\n%s", b)
	}
	_, deps, _ := strings.Cut(string(yml), "\ndeps:\n")
	_, rest, _ := strings.Cut(deps, "  compute/trackapi:\n")
	if b := nextKey.Split(rest, 2)[0]; !hasDep(b, "compute") || !hasDep(b, "contracts") {
		t.Fatalf("compute/trackapi lacks compute or contracts in the lint file:\n%s", yml)
	}
}

func TestRootImportsOwnersetDeclarations(t *testing.T) {
	t.Parallel()
	fixture := strings.Replace(declFixture, "internal/store/api.go internal/store/bar", "internal/store/api.go internal/store/ownerset", 1)
	tree := map[string]string{}
	for k, v := range declTree {
		tree[k] = v
	}
	tree["internal/store/ownerset/api.go"] = "package ownerset\n"
	dir := scratch(t, tree)
	toml, _, err := generate(fixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b := mayImport(t, string(toml), "jobs"); !strings.Contains(b, `"store/ownerset"`) {
		t.Fatalf("jobs lacks store/ownerset in arch.toml:\n%s", b)
	}
	if b := mayImport(t, string(toml), "store"); strings.Contains(b, "store/ownerset") {
		t.Fatalf("the root store imports its own declarations package:\n%s", b)
	}
}

func TestRootImportsSettingsapiDeclarations(t *testing.T) {
	t.Parallel()
	fixture := strings.Replace(declFixture, "internal/compute/api.go internal/compute/contracttest", "internal/compute/api.go internal/compute/settingsapi", 1)
	tree := map[string]string{}
	for k, v := range declTree {
		tree[k] = v
	}
	tree["internal/compute/settingsapi/api.go"] = "package settingsapi\n"
	dir := scratch(t, tree)
	toml, _, err := generate(fixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b := mayImport(t, string(toml), "jobs"); !strings.Contains(b, `"compute/settingsapi"`) {
		t.Fatalf("jobs lacks compute/settingsapi in arch.toml:\n%s", b)
	}
	if b := mayImport(t, string(toml), "compute"); strings.Contains(b, "compute/settingsapi") {
		t.Fatalf("the root compute imports its own declarations package:\n%s", b)
	}
}

func TestRootImportsDividendDeclarations(t *testing.T) {
	t.Parallel()
	fixture := strings.Replace(declFixture, "internal/store/api.go internal/store/bar", "internal/store/api.go internal/store/dividend", 1)
	tree := map[string]string{}
	for k, v := range declTree {
		tree[k] = v
	}
	tree["internal/store/dividend/api.go"] = "package dividend\n"
	dir := scratch(t, tree)
	toml, _, err := generate(fixture, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b := mayImport(t, string(toml), "jobs"); !strings.Contains(b, `"store/dividend"`) {
		t.Fatalf("jobs lacks store/dividend in arch.toml:\n%s", b)
	}
}

// treeModules is the table of the tree tests: b depends on a and contracts, ot on a and b but not on c.
const treeModules = "# Fixture\n\n## Modules\n\n" +
	"| module | paths | entry | tests | depends |\n" +
	"| --- | --- | --- | --- | --- |\n" +
	"| contracts | internal/contracts/** | | | |\n" +
	"| a | internal/a/** | | | |\n" +
	"| b | internal/b/** | | | a contracts |\n" +
	"| c | internal/c/** | | | |\n" +
	"| ot | cmd/** | | | a b |\n"

// treeFiles holds one package of each kind: decl (api.go alone), kit (imports "testing"),
// impl (api.go and x.go), the generated pair, and an implementation package of the row c.
var treeFiles = map[string]string{
	"internal/contracts/api.go":     "package contracts\n",
	"internal/contracts/gen/g.go":   "package gen\n",
	"internal/contracts/genpb/g.go": "package genpb\n",
	"internal/a/api.go":             "package a\n",
	"internal/a/decl/api.go":        "package decl\n",
	"internal/a/kit/k.go":           "package kit\n\nimport _ \"testing\"\n",
	"internal/a/impl/api.go":        "package impl\n",
	"internal/a/impl/x.go":          "package impl\n",
	"internal/a/sub/s.go":           "package sub\n",
	"internal/b/api.go":             "package b\n",
	"internal/b/sub/s.go":           "package sub\n",
	"internal/c/api.go":             "package c\n",
	"internal/c/impl/x.go":          "package impl\n",
	"cmd/ot/main.go":                "package main\n\nfunc main() {}\n",
}

// lintDeps returns the mayDependOn list of a component in the generated lint file.
func lintDeps(t *testing.T, files map[string]string, component string) []string {
	t.Helper()
	dir := t.TempDir()
	for p, src := range files {
		write(t, filepath.Join(dir, p), src)
	}
	_, yml, err := generate(treeModules, dir)
	if err != nil {
		t.Fatal(err)
	}
	_, deps, _ := strings.Cut(string(yml), "\ndeps:\n")
	_, rest, ok := strings.Cut(deps, "\n  "+component+":\n")
	if !ok {
		_, rest, ok = strings.Cut(deps, "  "+component+":\n")
	}
	if !ok {
		return nil
	}
	var out []string
	for _, l := range strings.Split(nextKey.Split(rest, 2)[0], "\n") {
		if d, ok := strings.CutPrefix(strings.TrimSpace(l), "- "); ok {
			out = append(out, d)
		}
	}
	return out
}

type edge struct {
	from string
	to   string
	want bool
}

func checkEdges(t *testing.T, edges []edge) {
	t.Helper()
	for _, e := range edges {
		got := slices.Contains(lintDeps(t, treeFiles, e.from), e.to)
		if got != e.want {
			t.Errorf("%s imports %s = %v, want %v", e.from, e.to, got, e.want)
		}
	}
}

func TestTreeDeclarationsOnlyPackageIsImportable(t *testing.T) {
	t.Parallel()
	checkEdges(t, []edge{
		{"b", "a/decl", true},
		{"b/sub", "a/decl", true},
		{"a/sub", "a/decl", true},
		{"ot", "a/decl", true},
		{"c", "a/decl", false},
		{"a/decl", "a/decl", false},
	})
}

func TestTreeTestKitIsImportedBySubPackagesOfDependentsOnly(t *testing.T) {
	t.Parallel()
	checkEdges(t, []edge{
		{"b/sub", "a/kit", true},
		{"b", "a/kit", false},
		{"a/sub", "a/kit", false},
		{"ot", "a/kit", false},
	})
}

func TestTreeImplementationPackageIsImportedByOTOnly(t *testing.T) {
	t.Parallel()
	checkEdges(t, []edge{
		{"ot", "a/impl", true},
		{"b", "a/impl", false},
		{"b/sub", "a/impl", false},
		{"a/sub", "a/impl", false},
	})
}

func TestTreeImplementationPackageOfARowOutsideTheDependsCellOfOT(t *testing.T) {
	t.Parallel()
	checkEdges(t, []edge{
		{"ot", "c/impl", false},
		{"ot", "b/sub", true},
		{"ot", "profile", false},
	})
}

func TestTreeGenpbIsImportedBySubPackagesOfDependentsOfContracts(t *testing.T) {
	t.Parallel()
	checkEdges(t, []edge{
		{"b/sub", "contracts/genpb", true},
		{"b", "contracts/genpb", false},
		{"a/sub", "contracts/genpb", false},
		{"ot", "contracts/genpb", false},
	})
}

func TestTreeGenIsImportedByTheRootContractsOnly(t *testing.T) {
	t.Parallel()
	checkEdges(t, []edge{
		{"contracts", "contracts/gen", true},
		{"b", "contracts/gen", false},
		{"b/sub", "contracts/gen", false},
		{"ot", "contracts/gen", false},
	})
}

func TestTreeClassifiesByFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for p, src := range map[string]string{
		"internal/a/withdoc/api.go":     "package withdoc\n",
		"internal/a/withdoc/doc.go":     "package withdoc\n",
		"internal/a/apikit/api.go":      "package apikit\n\nimport _ \"testing\"\n",
		"internal/a/onlytest/x_test.go": "package onlytest\n",
		"internal/a/noapi/x.go":         "package noapi\n",
		"internal/contracts/genpb/g.go": "package genpb\n\nimport _ \"testing\"\n",
	} {
		write(t, filepath.Join(dir, p), src)
	}
	rows, err := parseModules(treeModules)
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
	got := map[string]kind{}
	for _, p := range pkgs {
		got[p.dir] = p.kind
	}
	want := map[string]kind{
		"internal/a/withdoc":       kindDecl,
		"internal/a/apikit":        kindKit,
		"internal/a/noapi":         kindImpl,
		"internal/contracts/genpb": kindGenerated,
	}
	if !maps.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
}

func TestOwnerRowIsTheFirstMatchingRow(t *testing.T) {
	t.Parallel()
	rows := []row{
		{name: "first", paths: []string{"internal/x/special/**"}},
		{name: "second", paths: []string{"internal/x/**"}},
	}
	for dir, want := range map[string]string{
		"internal/x/special": "first", "internal/x/other": "second", "internal/y/z": "",
	} {
		if got := ownerRow(rows, dir); got != want {
			t.Errorf("ownerRow(%s) = %q, want %q", dir, got, want)
		}
	}
}

// sharedFiles is treeFiles plus the sub-package b/shared and a sibling b/other.
func sharedFiles() map[string]string {
	files := maps.Clone(treeFiles)
	files["internal/b/shared/s.go"] = "package shared\n"
	files["internal/b/other/o.go"] = "package other\n"
	return files
}

func TestTreeSharedIsImportedByTheOtherSubPackagesOfItsRow(t *testing.T) {
	t.Parallel()
	for _, e := range []edge{
		{"b/sub", "b/shared", true},
		{"b/other", "b/shared", true},
		{"b/sub", "b/other", false},
		{"b/other", "b/sub", false},
		{"b/shared", "b/sub", false},
		{"b/shared", "b/other", false},
		{"b/shared", "b/shared", false},
		{"b/shared", "b", true},
		{"b/shared", "a", true},
		{"b", "b/shared", false},
		{"a/sub", "b/shared", false},
		{"ot", "b/shared", false},
	} {
		if got := slices.Contains(lintDeps(t, sharedFiles(), e.from), e.to); got != e.want {
			t.Errorf("%s imports %s = %v, want %v", e.from, e.to, got, e.want)
		}
	}
}

func TestTreeSharedEdgeIsInTheTomlToo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for p, src := range sharedFiles() {
		write(t, filepath.Join(dir, p), src)
	}
	toml, _, err := generate(treeModules, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b := mayImportOf(t, string(toml), "b/sub"); !strings.Contains(b, `"b/shared"`) {
		t.Errorf("b/sub lacks b/shared:\n%s", b)
	}
	if b := mayImportOf(t, string(toml), "b/shared"); strings.Contains(b, `"b/sub"`) || strings.Contains(b, `"b/shared"`) {
		t.Errorf("b/shared imports a sibling:\n%s", b)
	}
}

func TestTreeWithoutSharedAddsNoEdge(t *testing.T) {
	t.Parallel()
	checkEdges(t, []edge{
		{"b/sub", "b/shared", false},
		{"a/sub", "a/shared", false},
	})
}
