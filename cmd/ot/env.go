package main

import (
	"fmt"
	"os"
	"strings"
)

// fredBaseURL is the FRED host; the source appends /fred/series/observations itself.
const fredBaseURL = "https://api.stlouisfed.org"

// readEnvFile reads KEY=VALUE lines of path into a map and does not touch the process
// environment. Blank lines and # lines are skipped; one pair of surrounding quotes is removed.
// The error names the path and never carries a value.
func readEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("env file %s cannot be read", path)
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		out[strings.TrimSpace(k)] = v
	}
	return out, nil
}

// envFile reads $OT_HOME/.env.
func (a *app) envFile() (map[string]string, error) {
	home := a.getenv("OT_HOME")
	if home == "" {
		return nil, fmt.Errorf("OT_HOME is not set")
	}
	return readEnvFile(home + string(os.PathSeparator) + ".env")
}
