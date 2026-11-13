package toml_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/accounts"
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

func loadStatement(t *testing.T, tail string) ([]string, error) {
	t.Helper()
	body := "mine = \"" + id("0001") + "\"\nwife = \"" + id("0002") + "\"\ndaughter = \"" + id("0003") + "\"\nmain = \"mine\"\n" + tail
	bk, err := toml.Load(write(t, body))
	if err != nil {
		return nil, err
	}
	s, ok := bk.(accounts.Sourced)
	if !ok {
		t.Fatal("book is not Sourced")
	}
	return s.Statement(), nil
}

func TestStatementLineIsRead(t *testing.T) {
	t.Parallel()
	got, err := loadStatement(t, "statement = [\"wife\", \"daughter\"]\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "wife,daughter" {
		t.Fatalf("Statement = %v", got)
	}
	got[0] = "changed"
	bk, _ := toml.Load(write(t, "mine = \""+id("0001")+"\"\nwife = \""+id("0002")+"\"\nmain = \"mine\"\nstatement = [\"wife\"]\n"))
	st := bk.(accounts.Sourced).Statement()
	st[0] = "changed"
	if bk.(accounts.Sourced).Statement()[0] != "wife" {
		t.Fatal("changing the returned slice changed the book")
	}
}

func TestStatementAbsentIsEmpty(t *testing.T) {
	t.Parallel()
	got, err := loadStatement(t, "")
	if err != nil || len(got) != 0 {
		t.Fatalf("Statement = %v, %v", got, err)
	}
}

func statementRefused(t *testing.T, tail, line string) {
	t.Helper()
	_, err := loadStatement(t, tail)
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), line+":") {
		t.Fatalf("error %q lacks %q", err, line)
	}
}

func TestStatementUnknownAliasRefused(t *testing.T) {
	t.Parallel()
	statementRefused(t, "statement = [\"nobody\"]\n", "line 5")
}

func TestStatementRepeatedAliasRefused(t *testing.T) {
	t.Parallel()
	statementRefused(t, "statement = [\"wife\", \"wife\"]\n", "line 5")
}

func TestStatementMainRefused(t *testing.T) {
	t.Parallel()
	statementRefused(t, "statement = [\"mine\"]\n", "line 5")
}

func TestStatementNotAnArrayRefused(t *testing.T) {
	t.Parallel()
	statementRefused(t, "statement = \"wife\"\n", "line 5")
	statementRefused(t, "statement = [1]\n", "line 5")
}

func TestStatementTwiceRefused(t *testing.T) {
	t.Parallel()
	statementRefused(t, "statement = [\"wife\"]\nstatement = [\"daughter\"]\n", "line 6")
}

func TestStatementRefusalNamesNoId(t *testing.T) {
	t.Parallel()
	for _, tail := range []string{"statement = [\"nobody\"]\n", "statement = [\"mine\"]\n", "statement = [\"wife\",\"wife\"]\n", "statement = 1\n"} {
		_, err := loadStatement(t, tail)
		if err == nil {
			t.Fatal("want an error")
		}
		for _, n := range []string{"0001", "0002", "0003"} {
			if strings.Contains(err.Error(), id(n)) {
				t.Fatalf("error names an id: %q", err)
			}
		}
	}
}
