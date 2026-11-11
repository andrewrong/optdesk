package flex

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/sources"
)

var (
	loginNameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	envNameRE   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	queryIDRE   = regexp.MustCompile(`^[0-9]+$`)
	aliasRE     = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
)

var _ sources.LoadFlexFileFunc = Load

func bad(format string, a ...any) error {
	return fmt.Errorf("%w: %s", sources.ErrFlexConfig, fmt.Sprintf(format, a...))
}

// Load reads and validates the flex.toml at path. Errors wrap sources.ErrFlexNotConfigured (no file)
// or sources.ErrFlexConfig; their texts name the entry and the field and never a value.
func Load(path string) (sources.FlexFile, error) {
	var zero sources.FlexFile
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return zero, fmt.Errorf("%w: no file", sources.ErrFlexNotConfigured)
		}
		return zero, bad("cannot open the file")
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return zero, bad("not a regular file")
	}
	if st.Mode().Perm()&0o077 != 0 {
		return zero, bad("file mode allows group or other access; use 0600")
	}
	if inGitWorkTree(path) {
		return zero, bad("file is inside a Git work tree")
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return zero, bad("cannot read the file")
	}
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return zero, bad("invalid TOML at line %d column %d", row, col)
		}
		return zero, bad("invalid TOML")
	}
	return validate(doc)
}

// inGitWorkTree reports whether any parent directory of path holds .git.
func inGitWorkTree(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}

func validate(doc map[string]any) (sources.FlexFile, error) {
	var out sources.FlexFile
	for k := range doc {
		if k != "login" && k != "account" {
			return out, bad("unknown table %q", k)
		}
	}
	logins, err := entries(doc, "login")
	if err != nil {
		return out, err
	}
	names := map[string]bool{}
	required := 0
	for i, e := range logins {
		l, err := login(i, e)
		if err != nil {
			return out, err
		}
		if names[l.Name] {
			return out, bad("login[%d] name: duplicate", i)
		}
		names[l.Name] = true
		if l.Required {
			required++
		}
		out.Logins = append(out.Logins, l)
	}
	if required != 1 {
		return out, bad("login: exactly one entry must have required = true, found %d", required)
	}
	accts, err := entries(doc, "account")
	if err != nil {
		return out, err
	}
	aliases := map[string]bool{}
	for i, e := range accts {
		a, err := account(i, e)
		if err != nil {
			return out, err
		}
		if aliases[a.Alias] {
			return out, bad("account[%d] alias: duplicate", i)
		}
		aliases[a.Alias] = true
		out.Accounts = append(out.Accounts, a)
	}
	return out, nil
}

func entries(doc map[string]any, key string) ([]map[string]any, error) {
	v, ok := doc[key]
	if !ok {
		return nil, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, bad("%s: must be an array of tables", key)
	}
	out := make([]map[string]any, len(arr))
	for i, x := range arr {
		m, ok := x.(map[string]any)
		if !ok {
			return nil, bad("%s[%d]: must be a table", key, i)
		}
		out[i] = m
	}
	return out, nil
}

func checkKeys(entry string, i int, m map[string]any, allowed ...string) error {
	for k := range m {
		found := false
		for _, a := range allowed {
			found = found || k == a
		}
		if !found {
			return bad("%s[%d]: unknown field", entry, i)
		}
	}
	return nil
}

func str(entry string, i int, m map[string]any, field string, re *regexp.Regexp) (string, error) {
	s, ok := m[field].(string)
	if !ok || !re.MatchString(s) {
		return "", bad("%s[%d] %s: missing or invalid", entry, i, field)
	}
	return s, nil
}

func login(i int, m map[string]any) (sources.FlexLogin, error) {
	var l sources.FlexLogin
	if err := checkKeys("login", i, m, "name", "token_env", "trade_queries", "required"); err != nil {
		return l, err
	}
	var err error
	if l.Name, err = str("login", i, m, "name", loginNameRE); err != nil {
		return l, err
	}
	if l.TokenEnv, err = str("login", i, m, "token_env", envNameRE); err != nil {
		return l, err
	}
	qs, ok := m["trade_queries"].([]any)
	if !ok || len(qs) == 0 {
		return l, bad("login[%d] trade_queries: must be a non-empty list", i)
	}
	seen := map[string]bool{}
	for _, q := range qs {
		s, ok := q.(string)
		if !ok || !queryIDRE.MatchString(s) || seen[s] {
			return l, bad("login[%d] trade_queries: entry invalid or duplicate", i)
		}
		seen[s] = true
		l.TradeQueries = append(l.TradeQueries, s)
	}
	if r, ok := m["required"]; ok {
		b, isBool := r.(bool)
		if !isBool {
			return l, bad("login[%d] required: must be a boolean", i)
		}
		l.Required = b
	}
	return l, nil
}

func account(i int, m map[string]any) (sources.FlexAccount, error) {
	var a sources.FlexAccount
	if err := checkKeys("account", i, m, "alias", "account_env"); err != nil {
		return a, err
	}
	var err error
	if a.Alias, err = str("account", i, m, "alias", aliasRE); err != nil {
		return a, err
	}
	a.AccountEnv, err = str("account", i, m, "account_env", envNameRE)
	return a, err
}
