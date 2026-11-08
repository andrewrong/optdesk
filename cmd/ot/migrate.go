package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/pressly/goose/v3"
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
		return errors.New(migrateErrorText(err))
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

// sqlStater is what a *pgconn.PgError offers; cmd/ot may not import pgx, so the error is found by
// this shape.
type sqlStater interface {
	error
	SQLState() string
}

// migrateErrorText turns the error of the migration run into a text that never holds the text of a
// wrapper of the chain, because that may hold the administrative DSN. The message is the text of the
// SQL error itself (a *pgconn.PgError says severity, message and SQLSTATE, never the DSN).
func migrateErrorText(err error) string {
	// a fixed-text stage error of setup (parse, connect, role, schema): it holds no DSN
	if errors.Unwrap(err) == nil && strings.HasPrefix(err.Error(), "setup: migrate: ") {
		return err.Error()
	}
	var pe *goose.PartialError
	var sq sqlStater
	isPartial := errors.As(err, &pe)
	isSQL := errors.As(err, &sq)
	if !isPartial && !isSQL {
		return "migrate failed; the error text is withheld because it may hold " + store.AdminDSNEnv
	}
	text := "migrate failed"
	if isPartial && pe.Failed != nil && pe.Failed.Source != nil {
		text += " at migration version " + strconv.FormatInt(pe.Failed.Source.Version, 10)
	}
	if isSQL {
		code := sq.SQLState()
		msg := strings.TrimSuffix(sq.Error(), " (SQLSTATE "+code+")")
		text += ": SQLSTATE " + code + ": " + msg
		if code == "42704" && strings.Contains(msg, "role ") {
			text += "; run ot setup pg first"
		}
	}
	return text
}
