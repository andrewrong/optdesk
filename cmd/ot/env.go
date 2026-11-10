package main

import (
	"fmt"
	"os"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
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

// envFile reads $CREW_SECRETS/$CREW_SERVICE.env when both are set and non-empty, else $OT_HOME/.env, through
// deps.readEnv, or defaultReadEnv when that field is nil. A missing file under CREW_SECRETS is an error of the
// reader, naming the path; there is no fallback to $OT_HOME/.env.
func (a *app) envFile() (map[string]string, error) {
	var path string
	secrets, service := a.getenv("CREW_SECRETS"), a.getenv("CREW_SERVICE")
	if secrets != "" && service != "" {
		path = secrets + string(os.PathSeparator) + service + ".env"
	} else {
		home := a.getenv("OT_HOME")
		if home == "" {
			return nil, fmt.Errorf("OT_HOME is not set")
		}
		path = home + string(os.PathSeparator) + ".env"
	}
	read := a.deps.readEnv
	if read == nil {
		read = defaultReadEnv
	}
	m, err := read(path)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// envFormKey names the line of .env that holds the MAC key of the settings form; its value is never printed.
const envFormKey = "OT_FORM_KEY"

// minFormKeyBytes is the shortest key the settings form accepts.
const minFormKeyBytes = 32

// reasonNoFormKey and reasonNoSettingsDSN are the texts logged when the route /owner-settings is left out.
const (
	reasonNoFormKey     = envFormKey + " is not set or under 32 bytes"
	reasonNoSettingsDSN = ownerset.DSNEnvSettings + " is not set"
)

// ownerEditSecrets reads the form key and the settings DSN from the env file only. reason is not empty when the
// route is to be left out (a line unset, or the key under minFormKeyBytes); it names the line, never a value.
func (a *app) ownerEditSecrets() (key []byte, dsn, reason string, err error) {
	env, err := a.envFile()
	if err != nil {
		return nil, "", "", err
	}
	if len(env[envFormKey]) < minFormKeyBytes {
		return nil, "", reasonNoFormKey, nil
	}
	if env[ownerset.DSNEnvSettings] == "" {
		return nil, "", reasonNoSettingsDSN, nil
	}
	return []byte(env[envFormKey]), env[ownerset.DSNEnvSettings], "", nil
}

// reasonNoBarDSN is the line logged when bars_refresh and fx_refresh (and the HV step of compute_figures) are left out.
const reasonNoBarDSN = bar.DSNEnvBar + " is not set"

// envSECUserAgent names the line of .env that holds the SEC user agent; a third-party name carries no OT_ prefix.
const envSECUserAgent = "SEC_" +
	"USER_AGENT"

// reasonNoSECUserAgent is the line logged when the fundamentals job is left out for want of a user agent.
const reasonNoSECUserAgent = envSECUserAgent + " is not set"

// reasonNoFactDSN is the line logged when the fundamentals job and the valuation step are left out.
const reasonNoFactDSN = store.DSNEnvFundamental + " is not set"
