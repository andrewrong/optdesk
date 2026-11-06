package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// envLine is one KEY=VALUE line of the env file.
type envLine struct{ key, value string }

// readEnv returns the KEY=VALUE pairs of the file at path through ReadEnv; a missing file gives an empty map.
func readEnv(path string) (map[string]string, error) {
	m, err := ReadEnv(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	return m, err
}

func splitLine(line string) (key, value string, ok bool) {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return "", "", false
	}
	k, v, found := strings.Cut(t, "=")
	if !found {
		return "", "", false
	}
	return strings.TrimSpace(k), v, true
}

// writeEnv sets each line in the file at path: an existing KEY line is replaced where it stands, a new one is
// appended. Every other line, comments and order included, is kept. The file is written only when its content
// changes, by a temporary file renamed over it, with mode 0600. Only key names are logged, never values.
func writeEnv(path string, set []envLine, log *slog.Logger) error {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is $OT_HOME/.env, chosen by the owner
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("setup: read %s: %w", path, err)
	}
	var lines []string
	if len(raw) > 0 {
		lines = strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	}
	for _, s := range set {
		replaced := false
		for i, l := range lines {
			if k, v, ok := splitLine(l); ok && k == s.key {
				replaced = true
				if unquoteValue(strings.TrimSpace(v)) != s.value {
					lines[i] = s.key + "=" + s.value
				}
			}
		}
		if !replaced {
			lines = append(lines, s.key+"="+s.value)
		}
	}
	next := strings.Join(lines, "\n") + "\n"
	info, statErr := os.Stat(path)
	if statErr == nil && next == string(raw) && info.Mode().Perm() == 0o600 {
		log.Info("env file unchanged", "path", path)
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".env-*")
	if err != nil {
		return fmt.Errorf("setup: write %s: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setup: chmod %s: %w", path, err)
	}
	if _, err := tmp.WriteString(next); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setup: write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("setup: write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("setup: replace %s: %w", path, err)
	}
	keys := make([]string, len(set))
	for i, s := range set {
		keys[i] = s.key
	}
	log.Info("env file written", "path", path, "keys", strings.Join(keys, ","))
	return nil
}
