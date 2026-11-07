package contracts_test

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemasDir = "../../contracts/schemas"

// compileCarried compiles one schema file of contracts/schemas. The keywords
// x-decimals, x-timezone and x-rounding are unknown to the compiler and ignored.
func compileCarried(t *testing.T, file string) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open(filepath.Join(schemasDir, file))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(file, doc); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	s, err := c.Compile(file)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return s
}

// readManifest returns file name -> expected sha256 from carryover.sha256.
func readManifest(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join(schemasDir, "carryover.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		sum, path, ok := strings.Cut(line, "  ")
		if !ok || len(sum) != 64 || !strings.HasPrefix(path, "contracts/schemas/") {
			t.Fatalf("bad manifest line %q", line)
		}
		name := strings.TrimPrefix(path, "contracts/schemas/")
		if _, dup := m[name]; dup {
			t.Fatalf("manifest lists %s twice", name)
		}
		m[name] = sum
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return m
}

// pageFileNames returns the names of the page.*.json files of dir, sorted.
func pageFileNames(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "page.*.json"))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(paths))
	for _, p := range paths {
		names = append(names, filepath.Base(p))
	}
	sort.Strings(names)
	return names
}

// classifyPages sorts the page files of dir: carried (a manifest line, outside the v2 and v3 lines), new (no
// manifest line, not of the v2 or v3 line, not page.macro_rates.v1.json, and an $id of this repository) and
// errs (a file that is neither: no manifest line and the wrong $id). Nothing is listed by hand.
func classifyPages(t *testing.T, dir string, manifest map[string]string) (carried, fresh, errs []string) {
	t.Helper()
	for _, n := range pageFileNames(t, dir) {
		if n == "page.macro_rates.v1.json" || slices.Contains(pageV2Files, n) || slices.Contains(pageV3Files, n) {
			continue
		}
		if _, ok := manifest[n]; ok {
			carried = append(carried, n)
			continue
		}
		if id, _ := readRaw(t, dir, n)["$id"].(string); id != schemaHost+n {
			errs = append(errs, n+": not in carryover.sha256 and $id is "+id+", want "+schemaHost+n)
			continue
		}
		fresh = append(fresh, n)
	}
	return carried, fresh, errs
}

func carriedFiles(t *testing.T) []string {
	t.Helper()
	carried, _, _ := classifyPages(t, schemasDir, readManifest(t))
	return carried
}

// A page file that is neither carried nor new fails; the new ones are derived from the directory.
func TestNewSchemasAreOurs(t *testing.T) {
	t.Parallel()
	_, _, errs := classifyPages(t, schemasDir, readManifest(t))
	for _, e := range errs {
		t.Error(e)
	}
}

// A carried copy with its manifest line removed is rejected: its $id is not ours.
func TestCarriedCopyWithoutManifestLineIsRejected(t *testing.T) {
	t.Parallel()
	manifest := readManifest(t)
	name := carriedFiles(t)[0]
	dir := t.TempDir()
	b, err := os.ReadFile(filepath.Join(schemasDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, errs := classifyPages(t, dir, manifest); len(errs) != 0 {
		t.Fatalf("with its manifest line the copy is rejected: %q", errs)
	}
	delete(manifest, name)
	carried, fresh, errs := classifyPages(t, dir, manifest)
	if len(errs) != 1 || len(carried) != 0 || len(fresh) != 0 {
		t.Errorf("without its manifest line: carried %q new %q errs %q, want one error", carried, fresh, errs)
	}
}

func TestCarryoverManifest(t *testing.T) {
	t.Parallel()
	manifest := readManifest(t)
	files, _, errs := classifyPages(t, schemasDir, manifest)
	for _, e := range errs {
		t.Error(e)
	}
	onDisk := map[string]bool{}
	for _, n := range files {
		onDisk[n] = true
	}
	if _, ok := manifest["page.macro_rates.v1.json"]; ok {
		t.Error("page.macro_rates.v1.json must not be in the manifest")
	}
	for name := range manifest {
		if !onDisk[name] {
			t.Errorf("manifest line without a file: %s", name)
		}
	}
	for _, name := range files {
		want := manifest[name]
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join(schemasDir, name))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(b)
			if got := hex.EncodeToString(sum[:]); got != want {
				t.Errorf("sha256 = %s, manifest has %s", got, want)
			}
			compileCarried(t, name)
		})
	}
	if len(files) != 29 {
		t.Errorf("carried schemas = %d, want 29", len(files))
	}
}

var timezoneValue = regexp.MustCompile(`"x-timezone"\s*:\s*("(?:[^"\\]|\\.)*"|[^,}\s]+)`)

// Every x-timezone value of the carried copies is the literal string "owner".
func TestCarryoverTimezoneIsOwner(t *testing.T) {
	t.Parallel()
	for _, name := range carriedFiles(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(filepath.Join(schemasDir, name))
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range timezoneValue.FindAllSubmatch(b, -1) {
				if string(m[1]) != `"owner"` {
					t.Errorf("x-timezone value is not the literal \"owner\"")
				}
			}
		})
	}
}
