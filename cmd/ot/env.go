package main

import (
	"fmt"
	"os"
)

// fredBaseURL is the FRED host; the source appends /fred/series/observations itself.
const fredBaseURL = "https://api.stlouisfed.org"

// alphaVantageBaseURL is the Alpha Vantage host; the source appends /query itself.
const alphaVantageBaseURL = "https://www.alphavantage.co"

// envAlphaKey names the line of .env that holds the Alpha Vantage key; its value is never printed.
const envAlphaKey = "ALPHA_VANTAGE_API_KEY"

// jquantsBaseURL is the J-Quants host; the source appends /v2/fins/earnings-date itself.
const jquantsBaseURL = "https://api.jquants.com"

// envJQuantsKey names the line of .env that holds the J-Quants key; its value is never printed.
const envJQuantsKey = "JQUANTS_API_KEY"

// reasonNoJQuantsKey is the line logged when the J-Quants source is left out: JP entries then get unsupported rows.
const reasonNoJQuantsKey = envJQuantsKey + " is not set"

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
