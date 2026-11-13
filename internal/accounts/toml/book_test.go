package toml_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/accounts/toml"
)

// Invented ids are built at run time so that no committed file holds an id-shaped string.
func id(n string) string { return "U" + "000" + n }

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "accounts.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadReadsTheBook(t *testing.T) {
	t.Parallel()
	a, b := id("0001"), id("0002")
	p := write(t, "# owner\nmine = \""+a+"\"\nwife = \""+b+"\"\nmain = \"mine\"\n")
	bk, err := toml.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := bk.Alias(b); !ok || got != "wife" {
		t.Fatalf("Alias = %q, %v", got, ok)
	}
	if bk.Main() != "mine" {
		t.Fatalf("Main = %q", bk.Main())
	}
	if got := strings.Join(bk.Aliases(), ","); got != "mine,wife" {
		t.Fatalf("Aliases = %q", got)
	}
}

func TestIDsInFileOrderAsACopy(t *testing.T) {
	t.Parallel()
	a, b := id("0001"), id("0002")
	bk, err := toml.Load(write(t, "wife = \""+b+"\"\nmine = \""+a+"\"\nmain = \"mine\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := bk.IDs()
	if len(got) != 2 || got[0] != b || got[1] != a {
		t.Fatalf("IDs order wrong: %d ids", len(got))
	}
	got[0] = "changed"
	if again := bk.IDs(); again[0] != b {
		t.Fatal("changing the returned slice changed the book")
	}
}

func TestAliasOfUnknownIDIsFalse(t *testing.T) {
	t.Parallel()
	bk, err := toml.Load(write(t, "mine = \""+id("0001")+"\"\nmain = \"mine\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := bk.Alias(id("0009")); ok || got != "" {
		t.Fatalf("Alias = %q, %v", got, ok)
	}
}

func TestLoadRefuses(t *testing.T) {
	t.Parallel()
	a := id("0001")
	cases := []struct {
		name, body, line string
	}{
		{"missing main", "mine = \"" + a + "\"\n", "line 1"},
		{"main names no alias", "mine = \"" + a + "\"\nmain = \"other\"\n", "line 2"},
		{"bad alias", "mine = \"" + a + "\"\nBad = \"" + id("0002") + "\"\nmain = \"mine\"\n", "line 2"},
		{"key main given twice, the second as an alias", "main = \"mine\"\nmain = \"" + a + "\"\nmine = \"" + id("0002") + "\"\n", "line 2"},
		{"repeated id", "mine = \"" + a + "\"\nwife = \"" + a + "\"\nmain = \"mine\"\n", "line 2"},
		{"bad id shape", "\nmine = \"" + a + "x\"\nmain = \"mine\"\n", "line 2"},
		{"not toml", "mine = \n", "line 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := toml.Load(write(t, c.body))
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), c.line+":") {
				t.Fatalf("error %q lacks %q", err, c.line)
			}
			if strings.Contains(err.Error(), a) || strings.Contains(err.Error(), id("0002")) {
				t.Fatalf("error names an id: %q", err)
			}
		})
	}
}

func TestLoadRefusesMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := toml.Load(filepath.Join(t.TempDir(), "accounts.toml")); err == nil {
		t.Fatal("want an error")
	}
}

func TestLoadRefusesMissingFileNamesALine(t *testing.T) {
	t.Parallel()
	_, err := toml.Load(filepath.Join(t.TempDir(), "accounts.toml"))
	if err == nil || !strings.Contains(err.Error(), "line ") {
		t.Fatalf("missing-file error %v names no line number", err)
	}
}

func TestLoadRefusesATableAtItsLine(t *testing.T) {
	t.Parallel()
	_, err := toml.Load(write(t, "mine = \""+id("0001")+"\"\n\n[extra]\nmain = \"mine\"\n"))
	if err == nil || !strings.Contains(err.Error(), "line 3:") {
		t.Fatalf("error %v does not name line 3", err)
	}
}
