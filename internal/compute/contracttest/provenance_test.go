package contracttest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// moduleRoot walks up from the test's directory to the one holding go.mod.
func moduleRoot(t *testing.T) string {
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

type provenance struct {
	Generator   string            `json:"generator"`
	Libraries   map[string]string `json:"libraries"`
	GeneratedAt string            `json:"generated_at"`
	GenSHA256   string            `json:"gen_sha256"`
	LockSHA256  string            `json:"lock_sha256"`
}

// checkProvenance judges a provenance object against the generator's gen.py and uv.lock in dir.
func checkProvenance(raw json.RawMessage, dir string) error {
	var p provenance
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if p.Generator != "gen.py" {
		return fmt.Errorf("generator = %q, want the file name gen.py", p.Generator)
	}
	if len(p.Libraries) == 0 || p.GeneratedAt == "" {
		return errors.New("libraries or generated_at is empty")
	}
	for field, want := range map[string]struct{ got, file string }{
		"gen_sha256":  {p.GenSHA256, "gen.py"},
		"lock_sha256": {p.LockSHA256, "uv.lock"},
	} {
		b, err := os.ReadFile(filepath.Join(dir, want.file))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		if got := hex.EncodeToString(sum[:]); got != want.got {
			return fmt.Errorf("%s = %s, but %s hashes to %s", field, want.got, want.file, got)
		}
	}
	return nil
}

var generatedFiles = []string{"indicators.json", "iv30_independent.json"}

func TestProvenanceHashes(t *testing.T) {
	t.Parallel()
	gen := filepath.Join(moduleRoot(t), "tools", "compute-golden")
	for _, name := range generatedFiles {
		f, err := Load(name)
		if err != nil {
			t.Fatalf("Load(%s): %v", name, err)
		}
		if f.Provenance == nil {
			t.Fatalf("%s has no provenance", name)
		}
		t.Run(name, func(t *testing.T) {
			if err := checkProvenance(f.Provenance, gen); err != nil {
				t.Fatal(err)
			}
		})
		t.Run(name+" edited gen.py", func(t *testing.T) {
			dir := t.TempDir()
			for _, file := range []string{"gen.py", "uv.lock"} {
				b, err := os.ReadFile(filepath.Join(gen, file))
				if err != nil {
					t.Fatal(err)
				}
				if file == "gen.py" {
					b = append(b, []byte("# edited\n")...)
				}
				if err := os.WriteFile(filepath.Join(dir, file), b, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if checkProvenance(f.Provenance, dir) == nil {
				t.Fatal("an edited gen.py passed the hash check")
			}
		})
	}
}

func TestGoldenCoversIndicators(t *testing.T) {
	t.Parallel()
	f, err := Load("indicators.json")
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, c := range f.Cases {
		count[c.Group]++
	}
	for _, fn := range []string{"SMA", "EMA", "WMA", "RSI", "ROC", "MACD", "BBands", "ATR"} {
		if count[fn] < 3 {
			t.Errorf("%s has %d cases, want three parameter sets", fn, count[fn])
		}
	}
	if len(count) != 8 {
		t.Errorf("groups = %v, want exactly the eight functions", count)
	}
}

// checkLengths fails when an expected array of a case is not as long as its input series.
func checkLengths(f File) error {
	for _, c := range f.Cases {
		var in map[string]json.RawMessage
		if err := json.Unmarshal(c.Input, &in); err != nil {
			return err
		}
		key := "values"
		if c.Group == "ATR" {
			key = "close"
		}
		var series []float64
		if err := json.Unmarshal(in[key], &series); err != nil {
			return fmt.Errorf("%s: input %s: %w", c.Name, key, err)
		}
		var out map[string][]*float64
		if err := json.Unmarshal(c.Expected, &out); err != nil || len(out) == 0 {
			return fmt.Errorf("%s: expected is not a map of arrays: %v", c.Name, err)
		}
		for k, a := range out {
			if len(a) != len(series) {
				return fmt.Errorf("%s: expected %s has %d entries, input %d", c.Name, k, len(a), len(series))
			}
		}
	}
	return nil
}

func TestFixtureLengths(t *testing.T) {
	t.Parallel()
	f, err := Load("indicators.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkLengths(f); err != nil {
		t.Fatal(err)
	}
	t.Run("mutated copy fails", func(t *testing.T) {
		c := f.Cases[0]
		var out map[string][]*float64
		if err := json.Unmarshal(c.Expected, &out); err != nil {
			t.Fatal(err)
		}
		for k := range out {
			out[k] = out[k][1:]
		}
		short := f
		short.Cases = append([]Case{}, f.Cases...)
		short.Cases[0].Expected, _ = json.Marshal(out)
		if checkLengths(short) == nil {
			t.Fatal("a shortened expected passed")
		}
		if bytes.Equal(short.Cases[0].Expected, f.Cases[0].Expected) {
			t.Fatal("mutation did not change the case")
		}
	})
}
