package setup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func writeEnvBytes(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestEnvContractReal checks the real side: ReadEnv behind store.ReadEnvFunc.
func TestEnvContractReal(t *testing.T) {
	t.Parallel()
	var read store.ReadEnvFunc = ReadEnv
	p := writeEnvBytes(t, store.DSNEnvLoader+"=postgres://ot_loader@h/d\n")
	m, err := read(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := m[store.DSNEnvLoader]; got != "postgres://ot_loader@h/d" {
		t.Fatalf("loader dsn %q", got)
	}
}

func TestReadEnv(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"comments and blanks", "# c\n\nA=1\n  # d\nB=two words\n", map[string]string{"A": "1", "B": "two words"}},
		{"line without equals", "junk\nA=1\n", map[string]string{"A": "1"}},
		{"spaces", "  A  =  v  \n", map[string]string{"A": "v"}},
		{"double quotes", "A=\"x y\"\n", map[string]string{"A": "x y"}},
		{"single quotes", "B='z'\n", map[string]string{"B": "z"}},
		{"doubled quotes", "C=\"\"q\"\"\n", map[string]string{"C": "\"q\""}},
		{"open quote", "D=\"open\n", map[string]string{"D": "\"open"}},
		{"value with equals", "E=a=b=c\n", map[string]string{"E": "a=b=c"}},
		{"empty value", "F=\n", map[string]string{"F": ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := ReadEnv(writeEnvBytes(t, c.in))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("map %v, want %v", got, c.want)
			}
			for k, v := range c.want {
				if g, ok := got[k]; !ok || g != v {
					t.Fatalf("map %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestReadEnvMissing(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), ".env")
	_, err := ReadEnv(p)
	if err == nil || !strings.Contains(err.Error(), p) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error %v", err)
	}
}

func TestReadEnvUnreadable(t *testing.T) {
	t.Parallel()
	const sentinel = "S3NTINEL-pw"
	t.Run("directory", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(t.TempDir(), ".env")
		if err := os.Mkdir(p, 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := ReadEnv(p)
		if err == nil || !strings.Contains(err.Error(), p) || strings.Contains(err.Error(), sentinel) {
			t.Fatalf("error %v", err)
		}
	})
	t.Run("mode 0000", func(t *testing.T) {
		t.Parallel()
		p := writeEnvBytes(t, "OT_PG_DSN_X=postgres://u:"+sentinel+"@h/d\n")
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		if f, err := os.Open(p); err == nil {
			_ = f.Close()
			t.Skip("mode 0000 is readable here")
		}
		_, err := ReadEnv(p)
		if err == nil || !strings.Contains(err.Error(), p) || strings.Contains(err.Error(), sentinel) {
			t.Fatalf("error %v", err)
		}
	})
}
