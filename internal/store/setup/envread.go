package setup

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.ReadEnvFunc = ReadEnv

// ReadEnv returns the KEY=VALUE pairs of the env file at path and does not touch the process environment.
// Blank lines, # lines and lines without '=' are skipped; key and value are trimmed, the first '=' splits and
// one pair of matching surrounding quotes is removed. The error of a missing or unreadable file names the path
// and wraps the OS reason; it never carries a byte of the file.
func ReadEnv(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is $OT_HOME/.env, chosen by the owner
	if err != nil {
		return nil, fmt.Errorf("env file %s cannot be read: %w", path, err)
	}
	out := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	for sc.Scan() {
		if k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "="); ok && !strings.HasPrefix(k, "#") {
			out[strings.TrimSpace(k)] = unquoteValue(strings.TrimSpace(v))
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("env file %s cannot be read", path)
	}
	return out, nil
}

// unquoteValue strips one matching pair of surrounding quotes from a trimmed value.
func unquoteValue(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}
