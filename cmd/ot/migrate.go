package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("migrate", runMigrate) }

// runMigrate applies the migrations of a directory under the administrative DSN as ot_owner.
func runMigrate(a *app, args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", "migrations", "directory of the migration files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: ot migrate [--dir DIR]")
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("migrate needs a real profile; the demo profile opens no store")
	}
	dsn := a.getenv(store.AdminDSNEnv)
	if dsn == "" {
		return fmt.Errorf("%s is not set", store.AdminDSNEnv)
	}
	if !hasSQLFile(*dir) {
		return fmt.Errorf("migrate: --dir %q is not a directory with .sql files; pass --dir <repository>/migrations", *dir)
	}
	if err := a.deps.migrateUp(context.Background(), dsn, os.DirFS(*dir)); err != nil {
		return fmt.Errorf("migrate failed; the error text is withheld because it may hold %s", store.AdminDSNEnv)
	}
	a.log.Info("migrate "+*dir+" done", "component", "ot", "status", "ok")
	return nil
}

// hasSQLFile reports whether dir is a readable directory holding at least one *.sql file.
func hasSQLFile(dir string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			return true
		}
	}
	return false
}
