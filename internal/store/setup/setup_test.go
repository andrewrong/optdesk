package setup

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestEnvfile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".env")
	const secret1, secret2 = "s3cr3tpassw0rdone", "s3cr3tpassw0rdtwo"
	orig := "# keys of the owner\nFRED_API_KEY=abc\n\nIB_FLEX_TOKEN=xyz\n" + store.DSNEnvRunner + "=old\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil { //nolint:gosec // a loose mode on purpose
		t.Fatal(err)
	}
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	set := []envLine{
		{store.DSNEnvRunner, "postgres://ot_runner:" + secret1 + "@h:5432/optdesk"},
		{store.DSNEnvPages, "postgres://ot_ro_pages:" + secret2 + "@h:5432/optdesk"},
	}
	if err := writeEnv(path, set, log); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}
	first, _ := os.ReadFile(path)
	want := "# keys of the owner\nFRED_API_KEY=abc\n\nIB_FLEX_TOKEN=xyz\n" +
		store.DSNEnvRunner + "=" + set[0].value + "\n" + store.DSNEnvPages + "=" + set[1].value + "\n"
	if string(first) != want {
		t.Errorf("file =\n%s\nwant\n%s", first, want)
	}

	// A second run with the same lines changes nothing.
	if err := writeEnv(path, set, log); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(second) != string(first) {
		t.Errorf("second run changed the file:\n%s", second)
	}
	info2, _ := os.Stat(path)
	if !info2.ModTime().Equal(info.ModTime()) {
		t.Error("second run rewrote the file")
	}

	got, err := readEnv(path)
	if err != nil || got["FRED_API_KEY"] != "abc" || got[store.DSNEnvPages] != set[1].value {
		t.Errorf("readEnv = %v, %v", got, err)
	}
	for _, s := range []string{secret1, secret2} {
		if strings.Contains(logs.String(), s) {
			t.Errorf("a password is in the log: %s", logs.String())
		}
	}
	if logs.Len() == 0 {
		t.Error("nothing was logged")
	}
}

func TestEnvfileMissingFileIsCreated(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".env")
	m, err := readEnv(path)
	if err != nil || len(m) != 0 {
		t.Fatalf("readEnv = %v, %v", m, err)
	}
	if err := writeEnv(path, []envLine{{"A", "1"}}, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "A=1\n" {
		t.Errorf("file = %q", raw)
	}
}

func TestDSNFromAdmin(t *testing.T) {
	t.Parallel()
	got, err := roleDSN("postgres://postgres:adminpw@dbhost:5433/postgres?sslmode=require&application_name=x", "ot_runner", "pw1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "postgres://ot_runner:pw1@dbhost:5433/optdesk?sslmode=require"; got != want {
		t.Errorf("dsn = %q, want %q", got, want)
	}
	if _, err := roleDSN("host=db user=x", "ot_runner", "pw1"); err == nil {
		t.Error("a key/value DSN was accepted")
	}
}

func TestPasswordFromDSN(t *testing.T) {
	t.Parallel()
	if pw := passwordOf("postgres://ot_runner:pw1@dbhost:5433/optdesk", "ot_runner"); pw != "pw1" {
		t.Errorf("password = %q", pw)
	}
	if pw := passwordOf("postgres://someone_else:pw1@dbhost/optdesk", "ot_runner"); pw != "" {
		t.Errorf("password of another user = %q", pw)
	}
	if pw := passwordOf("garbage", "ot_runner"); pw != "" {
		t.Errorf("password = %q", pw)
	}
}

func TestRolesTable(t *testing.T) {
	t.Parallel()
	if len(roles) != 7 {
		t.Fatalf("%d roles, want 7", len(roles))
	}
	keys := map[string]string{}
	for _, r := range roles {
		if r.envKey == "" && r.name != roleOwner {
			t.Errorf("role %s has no line key", r.name)
		}
		keys[r.name] = r.envKey
	}
	if keys[roleOwner] != "" {
		t.Errorf("ot_owner line = %q, want none", keys[roleOwner])
	}
	for name, want := range map[string]string{
		"ot_loader":           store.DSNEnvLoader,
		"ot_w_option_eod":     store.DSNEnvOptionEod,
		"ot_w_quote_snapshot": store.DSNEnvQuoteSnapshot,
	} {
		if got, ok := keys[name]; !ok || got != want {
			t.Errorf("%s line = %q (present %v), want %q", name, got, ok, want)
		}
	}
	if roleOptionEod != "ot_w_option_eod" || roleQuoteSnapshot != "ot_w_quote_snapshot" {
		t.Errorf("role names %q, %q", roleOptionEod, roleQuoteSnapshot)
	}
}

func TestEnvfileQuotedValue(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".env")
	orig := "# c\n" + store.DSNEnvRunner + "=\"postgres://u:p@h:1/d\"\nB=2\n"
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := readEnv(path)
	if err != nil || m[store.DSNEnvRunner] != "postgres://u:p@h:1/d" {
		t.Fatalf("readEnv = %v, %v", m, err)
	}
	log := slog.New(slog.DiscardHandler)
	if err := writeEnv(path, []envLine{{store.DSNEnvRunner, "postgres://u:p@h:1/d"}}, log); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != orig {
		t.Errorf("file changed: %q", got)
	}
	if err := writeEnv(path, []envLine{{store.DSNEnvRunner, "postgres://u:q@h:1/d"}}, log); err != nil {
		t.Fatal(err)
	}
	want := "# c\n" + store.DSNEnvRunner + "=postgres://u:q@h:1/d\nB=2\n"
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}
