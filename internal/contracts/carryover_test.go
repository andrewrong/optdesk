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

// pageNewFiles names the schemas written in this repository after the carried copies; they are not in carryover.sha256.
var pageNewFiles = []string{
	"page.option_data_page.v1.json",
	"page.macro_rates.v2.json",
}

func carriedFiles(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(schemasDir, "page.*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range paths {
		n := filepath.Base(p)
		if n == "page.macro_rates.v1.json" || slices.Contains(pageV2Files, n) || slices.Contains(pageV3Files, n) || slices.Contains(pageNewFiles, n) {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func TestCarryoverManifest(t *testing.T) {
	t.Parallel()
	manifest := readManifest(t)
	files := carriedFiles(t)
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
		want, ok := manifest[name]
		if !ok {
			t.Errorf("file missing from the manifest: %s", name)
			continue
		}
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
