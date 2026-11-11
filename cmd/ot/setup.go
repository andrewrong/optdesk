package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("setup", runSetup) }

func runSetup(a *app, args []string) error {
	if len(args) != 1 || args[0] != "pg" {
		return fmt.Errorf("usage: ot setup pg")
	}
	cfg := store.SetupConfig{AdminDSN: a.getenv(store.AdminDSNEnv), Home: a.getenv("OT_HOME")}
	if cfg.AdminDSN == "" {
		return fmt.Errorf("%s is not set", store.AdminDSNEnv)
	}
	if cfg.Home == "" {
		return fmt.Errorf("OT_HOME is not set")
	}
	if err := a.deps.setup(context.Background(), cfg); err != nil {
		return fmt.Errorf("setup pg failed; the error text is withheld because it may hold %s", store.AdminDSNEnv)
	}
	if a.getenv("CREW_SECRETS") != "" && a.getenv("CREW_SERVICE") != "" {
		reportServiceEnv(a, cfg.Home)
	}
	a.log.Info("setup pg done", "component", "ot", "status", "ok")
	return nil
}

// pgDSNPrefix starts the keys of the generated lines of $OT_HOME/.env.
const pgDSNPrefix = "OT_PG_DSN_"

// reportServiceEnv prints one line to stdout naming the service env file and the OT_PG_DSN_* keys of $OT_HOME/.env
// that the file lacks or holds with another value. It does not go through the logger (its whitelist drops these
// attributes), never prints a value, writes nothing to either file, and a read error is printed as a line with
// the path and the error text, never failing the command (the roles are already set up).
func reportServiceEnv(a *app, home string) {
	read := a.deps.readEnv
	if read == nil {
		read = defaultReadEnv
	}
	svcPath := a.getenv("CREW_SECRETS") + string(os.PathSeparator) + a.getenv("CREW_SERVICE") + ".env"
	homePath := home + string(os.PathSeparator) + ".env"
	homeEnv, err := read(homePath)
	if err != nil {
		fmt.Fprintf(a.stdout, "setup pg: cannot read %s: %v\n", homePath, err)
		return
	}
	svcEnv, err := read(svcPath)
	if err != nil {
		fmt.Fprintf(a.stdout, "setup pg: cannot read %s: %v\n", svcPath, err)
		return
	}
	var keys []string
	for k, v := range homeEnv {
		if !strings.HasPrefix(k, pgDSNPrefix) {
			continue
		}
		if sv, ok := svcEnv[k]; !ok || sv != v {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return
	}
	sort.Strings(keys)
	fmt.Fprintf(a.stdout, "setup pg: the service env file %s lacks or holds differently: %s\n", svcPath, strings.Join(keys, ", "))
}
