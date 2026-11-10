package setup

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
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
	if len(roles) != 19 {
		t.Fatalf("%d roles, want 19", len(roles))
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
		"ot_w_master":         store.DSNEnvMaster,
	} {
		if got, ok := keys[name]; !ok || got != want {
			t.Errorf("%s line = %q (present %v), want %q", name, got, ok, want)
		}
	}
	if roleOptionEod != "ot_w_option_eod" || roleQuoteSnapshot != "ot_w_quote_snapshot" || roleMaster != "ot_w_master" {
		t.Errorf("role names %q, %q, %q", roleOptionEod, roleQuoteSnapshot, roleMaster)
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

func TestRoleListHasReadings(t *testing.T) {
	t.Parallel()
	if roleReadings != "ot_w_readings" || store.DSNEnvReadings != "OT_PG_DSN_READINGS" {
		t.Fatalf("role %q, line %q", roleReadings, store.DSNEnvReadings)
	}
	for _, r := range roles {
		if r.name == roleReadings {
			if r.envKey != "OT_PG_DSN_READINGS" {
				t.Errorf("ot_w_readings line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_readings is not in the role list")
}

func TestAlertRoleInList(t *testing.T) {
	t.Parallel()
	if roleAlerts != "ot_w_alerts" || store.DSNEnvAlerts != "OT_PG_DSN_ALERTS" {
		t.Fatalf("role %q, line %q", roleAlerts, store.DSNEnvAlerts)
	}
	for _, r := range roles {
		if r.name == roleAlerts {
			if r.envKey != store.DSNEnvAlerts {
				t.Errorf("ot_w_alerts line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_alerts is not in the role list")
}

func TestFundamentalRoleInList(t *testing.T) {
	t.Parallel()
	if roleFundamental != "ot_w_fundamental" || store.DSNEnvFundamental != "OT_PG_DSN_FUNDAMENTAL" {
		t.Fatalf("role %q, line %q", roleFundamental, store.DSNEnvFundamental)
	}
	for _, r := range roles {
		if r.name == roleFundamental {
			if r.envKey != store.DSNEnvFundamental {
				t.Errorf("ot_w_fundamental line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_fundamental is not in the role list")
}

func TestEventsRoleInList(t *testing.T) {
	t.Parallel()
	if roleEvents != "ot_w_events" || store.DSNEnvEvents != "OT_PG_DSN_EVENTS" {
		t.Fatalf("role %q, line %q", roleEvents, store.DSNEnvEvents)
	}
	for _, r := range roles {
		if r.name == roleEvents {
			if r.envKey != store.DSNEnvEvents {
				t.Errorf("ot_w_events line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_events is not in the role list")
}

func TestCandidateRoleSpec(t *testing.T) {
	t.Parallel()
	if roleCandidate != "ot_w_candidate" || store.DSNEnvCandidate != "OT_PG_DSN_CANDIDATE" {
		t.Fatalf("role %q, line %q", roleCandidate, store.DSNEnvCandidate)
	}
	for _, r := range roles {
		if r.name == roleCandidate {
			if r.envKey != store.DSNEnvCandidate {
				t.Errorf("ot_w_candidate line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_candidate is not in the role list")
}

func TestAccountRoleInList(t *testing.T) {
	t.Parallel()
	if roleAccount != "ot_w_account" || store.DSNEnvAccount != "OT_PG_DSN_ACCOUNT" {
		t.Fatalf("role %q, line %q", roleAccount, store.DSNEnvAccount)
	}
	for _, r := range roles {
		if r.name == roleAccount {
			if r.envKey != store.DSNEnvAccount {
				t.Errorf("ot_w_account line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_account is not in the role list")
}

func TestBarRoleSpec(t *testing.T) {
	t.Parallel()
	if roleBar != "ot_w_bar" || bar.DSNEnvBar != "OT_PG_DSN_BAR" {
		t.Fatalf("role %q, line %q", roleBar, bar.DSNEnvBar)
	}
	for _, r := range roles {
		if r.name == roleBar {
			if r.envKey != "OT_PG_DSN_BAR" {
				t.Errorf("ot_w_bar line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_bar is not in the role list")
}

func TestOptionLedgerRoleInList(t *testing.T) {
	t.Parallel()
	if roleOptionLedger != "ot_w_optionledger" || store.DSNEnvOptionLedger != "OT_PG_DSN_OPTION_LEDGER" {
		t.Fatalf("role %q, line %q", roleOptionLedger, store.DSNEnvOptionLedger)
	}
	for _, r := range roles {
		if r.name == roleOptionLedger {
			if r.envKey != store.DSNEnvOptionLedger {
				t.Errorf("ot_w_optionledger line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_optionledger is not in the role list")
}

func TestSettingsRoleSpec(t *testing.T) {
	t.Parallel()
	if roleSettings != "ot_w_settings" || dsnEnvSettings != "OT_PG_DSN_SETTINGS" {
		t.Fatalf("role %q, line %q", roleSettings, dsnEnvSettings)
	}
	for _, r := range roles {
		if r.name == roleSettings {
			if r.envKey != "OT_PG_DSN_SETTINGS" {
				t.Errorf("ot_w_settings line = %q", r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_settings is not in the role list")
}

func TestMarkRoleSpec(t *testing.T) {
	t.Parallel()
	for _, r := range roles {
		if r.name == roleMark {
			if r.name != "ot_w_posmark" || r.envKey != "OT_PG_DSN_POSMARK" {
				t.Errorf("role %q line %q", r.name, r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_posmark is not in the role list")
}

func TestOptionOpenRoleSpec(t *testing.T) {
	t.Parallel()
	for _, r := range roles {
		if r.name == roleOptionOpen {
			if r.name != "ot_w_option_open" || r.envKey != "OT_PG_DSN_OPTION_OPEN" {
				t.Errorf("role %q line %q", r.name, r.envKey)
			}
			return
		}
	}
	t.Error("ot_w_option_open is not in the role list")
}
