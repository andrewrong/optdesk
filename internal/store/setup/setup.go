// Package setup creates the database, the extension and the roles of week 1 on the owner's machine.
package setup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.SetupFunc = Setup

// Database is the name of the database `Setup` creates.
const Database = "optdesk"

// The roles. Their grants on tables are in migrations/0002_series.sql to 0005_session_day_loader.sql,
// which goose runs as ot_owner; later issues add the other roles with their tables.
const (
	roleOwner      = "ot_owner"
	roleRunner     = "ot_runner"
	roleSeriesFred = "ot_w_series_fred"
	roleRoPages    = "ot_ro_pages"
	roleLoader     = "ot_loader"

	roleOptionEod     = "ot_w_option_eod"
	roleQuoteSnapshot = "ot_w_quote_snapshot"
	roleMaster        = "ot_w_master"
	roleReadings      = "ot_w_readings"
	roleAlerts        = "ot_w_alerts" // INSERT and SELECT on alerts.sent only (migrations/0015_alerts.sql)
)

// roleSpec is one role and the .env line that holds its DSN; the owner has no line (goose only).
type roleSpec struct{ name, envKey string }

var roles = []roleSpec{
	{roleOwner, ""},
	{roleRunner, store.DSNEnvRunner},
	{roleSeriesFred, store.DSNEnvSeriesFred},
	{roleRoPages, store.DSNEnvPages},
	{roleLoader, store.DSNEnvLoader},
	{roleOptionEod, store.DSNEnvOptionEod},
	{roleQuoteSnapshot, store.DSNEnvQuoteSnapshot},
	{roleMaster, store.DSNEnvMaster},
	{roleReadings, store.DSNEnvReadings},
	{roleAlerts, store.DSNEnvAlerts}, // ot_w_alerts, line OT_PG_DSN_ALERTS
}

// extensions are created in the database by Setup.
var extensions = []string{"timescaledb"}

// Setup creates database optdesk, the Timescale extension and the ten roles on the server of cfg.AdminDSN,
// each role with a generated password, then writes the OT_PG_DSN_* lines of cfg.Home/.env.
// It is idempotent: a role whose DSN is already in .env keeps that password, and the owner is created once.
// The SQL runs first; .env is written only after it succeeded. No password is logged.
func Setup(ctx context.Context, cfg store.SetupConfig) error {
	return run(ctx, cfg, extensions, slog.Default())
}

func run(ctx context.Context, cfg store.SetupConfig, exts []string, log *slog.Logger) error {
	if cfg.AdminDSN == "" {
		return permanent(fmt.Errorf("setup: no admin DSN (environment variable %s)", store.AdminDSNEnv))
	}
	if cfg.Home == "" {
		return permanent(errors.New("setup: no home directory"))
	}
	envPath := filepath.Join(cfg.Home, ".env")
	existing, err := readEnv(envPath)
	if err != nil {
		return permanent(err)
	}
	passwords := map[string]string{}
	for _, r := range roles {
		pw := passwordOf(existing[r.envKey], r.name)
		if pw == "" {
			if pw, err = generatePassword(); err != nil {
				return permanent(err)
			}
		}
		passwords[r.name] = pw
	}

	if err := createServerObjects(ctx, cfg.AdminDSN, passwords, exts, log); err != nil {
		return retryable(err)
	}

	var lines []envLine
	for _, r := range roles {
		if r.envKey == "" {
			continue
		}
		dsn, err := roleDSN(cfg.AdminDSN, r.name, passwords[r.name])
		if err != nil {
			return permanent(err)
		}
		lines = append(lines, envLine{r.envKey, dsn})
	}
	if err := writeEnv(envPath, lines, log); err != nil {
		return permanent(err)
	}
	return nil
}

func createServerObjects(ctx context.Context, adminDSN string, passwords map[string]string, exts []string, log *slog.Logger) error {
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return fmt.Errorf("setup: connect as admin: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	for _, r := range roles {
		if err := ensureRole(ctx, conn, r.name, passwords[r.name], r.envKey != ""); err != nil {
			return err
		}
		log.Info("role ready", "role", r.name)
	}
	var have bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, Database).Scan(&have); err != nil {
		return fmt.Errorf("setup: look up database: %w", err)
	}
	if !have {
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+ident(Database)+" OWNER "+ident(roleOwner)); err != nil {
			return fmt.Errorf("setup: create database: %w", err)
		}
	}
	grant := "REVOKE ALL ON DATABASE " + ident(Database) + " FROM PUBLIC; GRANT CONNECT ON DATABASE " + ident(Database) +
		" TO " + ident(roleRunner) + ", " + ident(roleSeriesFred) + ", " + ident(roleRoPages) + ", " + ident(roleLoader) +
		", " + ident(roleOptionEod) + ", " + ident(roleQuoteSnapshot) + ", " + ident(roleMaster) + ", " + ident(roleReadings) + ", " + ident(roleAlerts)
	if _, err := conn.Exec(ctx, grant); err != nil {
		return fmt.Errorf("setup: grant connect: %w", err)
	}
	log.Info("database ready", "database", Database)

	dbDSN, err := withDatabase(adminDSN)
	if err != nil {
		return err
	}
	db, err := pgx.Connect(ctx, dbDSN)
	if err != nil {
		return fmt.Errorf("setup: connect to %s: %w", Database, err)
	}
	defer func() { _ = db.Close(ctx) }()
	for _, e := range exts {
		if _, err := db.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS "+ident(e)); err != nil {
			return fmt.Errorf("setup: create extension %s: %w", e, err)
		}
		log.Info("extension ready", "extension", e)
	}
	return nil
}

// ensureRole creates the role, or for a role with a stored DSN sets its password again to the stored one.
// The owner is created once and its password is not touched afterwards.
func ensureRole(ctx context.Context, conn *pgx.Conn, name, password string, resetPassword bool) error {
	var have bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, name).Scan(&have); err != nil {
		return fmt.Errorf("setup: look up role %s: %w", name, err)
	}
	// The statement carries a SCRAM-SHA-256 verifier, never the plain password: a server may log DDL text.
	verifier := scramVerifier(password, newSalt())
	var sql string
	switch {
	case !have:
		sql = "CREATE ROLE " + ident(name) + " LOGIN PASSWORD " + literal(verifier)
	case resetPassword:
		sql = "ALTER ROLE " + ident(name) + " PASSWORD " + literal(verifier)
	default:
		return nil
	}
	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("setup: role %s: %w", name, err)
	}
	return nil
}

// generatePassword returns 32 random bytes in hex.
func generatePassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("setup: generate password: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func ident(s string) string   { return pgx.Identifier{s}.Sanitize() }
func literal(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// withDatabase returns adminDSN pointing at the database Database.
func withDatabase(adminDSN string) (string, error) {
	u, err := parseURL(adminDSN)
	if err != nil {
		return "", err
	}
	u.Path = "/" + Database
	return u.String(), nil
}

// roleDSN builds the DSN of role in database Database on the server of adminDSN, keeping its sslmode.
func roleDSN(adminDSN, role, password string) (string, error) {
	u, err := parseURL(adminDSN)
	if err != nil {
		return "", err
	}
	out := url.URL{Scheme: u.Scheme, User: url.UserPassword(role, password), Host: u.Host, Path: "/" + Database}
	if m := u.Query().Get("sslmode"); m != "" {
		out.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return out.String(), nil
}

func parseURL(dsn string) (*url.URL, error) {
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return nil, permanent(fmt.Errorf("setup: the admin DSN must be a postgres:// URL (environment variable %s)", store.AdminDSNEnv))
	}
	return u, nil
}

// passwordOf returns the password of user in a stored DSN, or "" when there is none or it names another user.
func passwordOf(dsn, user string) string {
	if dsn == "" {
		return ""
	}
	u, err := url.Parse(dsn)
	if err != nil || u.User == nil || u.User.Username() != user {
		return ""
	}
	pw, _ := u.User.Password()
	return pw
}

// typedErr is a contracts.TypedError wrapping a cause.
type typedErr struct {
	kind  contracts.ErrorKind
	cause error
}

func (e *typedErr) Error() string             { return e.cause.Error() }
func (e *typedErr) Unwrap() error             { return e.cause }
func (e *typedErr) Kind() contracts.ErrorKind { return e.kind }

func retryable(err error) error { return &typedErr{kind: contracts.Retryable, cause: err} }
func permanent(err error) error { return &typedErr{kind: contracts.Permanent, cause: err} }
