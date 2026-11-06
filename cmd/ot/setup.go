package main

import (
	"context"
	"fmt"

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
	a.log.Info("setup pg done", "component", "ot", "status", "ok")
	return nil
}
