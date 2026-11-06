package main

import (
	"fmt"
	"os"
)

// fredBaseURL is the FRED host; the source appends /fred/series/observations itself.
const fredBaseURL = "https://api.stlouisfed.org"

// envFile reads $OT_HOME/.env through deps.readEnv, or defaultReadEnv when that field is nil.
func (a *app) envFile() (map[string]string, error) {
	home := a.getenv("OT_HOME")
	if home == "" {
		return nil, fmt.Errorf("OT_HOME is not set")
	}
	read := a.deps.readEnv
	if read == nil {
		read = defaultReadEnv
	}
	m, err := read(home + string(os.PathSeparator) + ".env")
	if err != nil {
		return nil, err
	}
	return m, nil
}
