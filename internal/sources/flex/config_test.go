package flex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/sources"
)

func writeConf(t *testing.T, dir, src string, mode os.FileMode) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", src))
	if err != nil {
		t.Fatal(err)
	}
	return writeBytes(t, dir, string(b), mode)
}

func writeBytes(t *testing.T, dir, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, "flex.toml")
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigLoads(t *testing.T) {
	t.Parallel()
	f, err := Load(writeConf(t, t.TempDir(), "flex_ok.toml", 0o600))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Logins) != 2 || !f.Logins[0].Required || f.Logins[1].Required {
		t.Fatalf("logins %+v", f.Logins)
	}
	if f.Logins[1].TradeQueries[1] != "3333333" || f.Logins[0].TokenEnv != "OPTDESK_FLEX_TOKEN_MAIN" {
		t.Fatalf("logins %+v", f.Logins)
	}
}

func TestConfigRefusesOpenMode(t *testing.T) {
	t.Parallel()
	_, err := Load(writeConf(t, t.TempDir(), "flex_ok.toml", 0o644))
	if !errors.Is(err, sources.ErrFlexConfig) {
		t.Fatalf("err %v", err)
	}
}

func TestConfigRefusesGitWorkTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := Load(writeConf(t, sub, "flex_ok.toml", 0o600))
	if !errors.Is(err, sources.ErrFlexConfig) || !strings.Contains(err.Error(), "Git work tree") {
		t.Fatalf("err %v", err)
	}
}

func TestConfigMissingFileNotConfigured(t *testing.T) {
	t.Parallel()
	_, err := Load(filepath.Join(t.TempDir(), "flex.toml"))
	if !errors.Is(err, sources.ErrFlexNotConfigured) {
		t.Fatalf("err %v", err)
	}
}

func TestConfigRefusesNoRequiredLogin(t *testing.T) {
	t.Parallel()
	src := "[[login]]\nname = \"main\"\ntoken_env = \"A_TOKEN\"\ntrade_queries = [\"1\"]\n"
	_, err := Load(writeBytes(t, t.TempDir(), src, 0o600))
	if !errors.Is(err, sources.ErrFlexConfig) || !strings.Contains(err.Error(), "required") {
		t.Fatalf("err %v", err)
	}
}

func TestConfigRefusesTwoRequiredLogins(t *testing.T) {
	t.Parallel()
	one := func(n string) string {
		return "[[login]]\nname = \"" + n + "\"\ntoken_env = \"A_TOKEN\"\ntrade_queries = [\"1\"]\nrequired = true\n"
	}
	_, err := Load(writeBytes(t, t.TempDir(), one("main")+one("other"), 0o600))
	if !errors.Is(err, sources.ErrFlexConfig) || !strings.Contains(err.Error(), "required") {
		t.Fatalf("err %v", err)
	}
}

func TestConfigRefusesBadEntryNamesFieldNotValue(t *testing.T) {
	t.Parallel()
	_, err := Load(writeConf(t, t.TempDir(), "flex_bad_entry.toml", 0o600))
	if !errors.Is(err, sources.ErrFlexConfig) {
		t.Fatalf("err %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "login[1]") || !strings.Contains(msg, "name") {
		t.Fatalf("no entry and field: %s", msg)
	}
	if strings.Contains(msg, "SECRET-LOOKING-VALUE") {
		t.Fatalf("value in error: %s", msg)
	}
}

func TestConfigHoldsNoTokenOrAccountValue(t *testing.T) {
	t.Parallel()
	f, err := Load(writeConf(t, t.TempDir(), "flex_ok.toml", 0o600))
	if err != nil {
		t.Fatal(err)
	}
	// The loaded value carries the NAMES of the variables, exactly as the file gives them; the
	// loader has no field for a token or an account number.
	if f.Logins[0].TokenEnv != "OPTDESK_FLEX_TOKEN_MAIN" || f.Logins[1].TokenEnv != "OPTDESK_FLEX_TOKEN_SECOND" {
		t.Fatalf("file %+v", f)
	}
}

func TestConfigRefusesAccountTable(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("testdata", "flex_ok.toml"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b) + "\n[[account]]\nalias = \"a\"\naccount_env = \"OPTDESK_FLEX_ACCOUNT_A\"\n"
	_, err = Load(writeBytes(t, t.TempDir(), src, 0o600))
	if !errors.Is(err, sources.ErrFlexConfig) || !strings.Contains(err.Error(), "unknown table") {
		t.Fatalf("err = %v", err)
	}
}

func TestConfigLoadsLoginsOnlyFiles(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"flex_alias_c.toml", "flex_alias_empty.toml", "flex_alias_upper.toml"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			f, err := Load(writeConf(t, t.TempDir(), file, 0o600))
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Logins) != 2 {
				t.Fatalf("logins %+v", f.Logins)
			}
		})
	}
}
