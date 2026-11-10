package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

func importApp(t *testing.T, res jobs.ImportResult, got *map[string]string) *app {
	t.Helper()
	a, _ := testApp(t, ledgerDSNLine)
	st := &ledgerFakeStore{}
	a.deps.openOptionLedgerStore = func(context.Context, string) (store.OptionLedgerStore, error) { return st, nil }
	a.deps.importOptionTrader = func(_ context.Context, r io.Reader, _ store.OptionLedgerWriter, m map[string]string, _ time.Time) (jobs.ImportResult, error) {
		if _, err := io.ReadAll(r); err != nil {
			return jobs.ImportResult{}, err
		}
		*got = m
		return res, nil
	}
	return a
}

func TestLedgerImportCommandPrintsCounts(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "rows.jsonl")
	if err := os.WriteFile(file, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	a := importApp(t, jobs.ImportResult{Lines: 5, Inserted: 3, Same: 2}, &m)
	if err := ledgerImport(a, []string{file, "--map", "old=new", "--map", "x=y"}); err != nil {
		t.Fatal(err)
	}
	if got, want := a.stdout.(interface{ String() string }).String(), "ledger-import lines=5 inserted=3 same=2 conflict=0\n"; got != want {
		t.Errorf("output %q, want %q", got, want)
	}
	if want := map[string]string{"old": "new", "x": "y"}; !reflect.DeepEqual(m, want) {
		t.Errorf("alias map %v, want %v", m, want)
	}

	var m2 map[string]string
	b := importApp(t, jobs.ImportResult{Lines: 2, Conflict: 1, Inserted: 1}, &m2)
	err := ledgerImport(b, []string{"--map", "old=new", file})
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 {
		t.Fatalf("conflict gave %v, want exit code 1", err)
	}
	if got := b.stdout.(interface{ String() string }).String(); got != "ledger-import lines=2 inserted=1 same=0 conflict=1\n" {
		t.Errorf("output %q", got)
	}
	if err := ledgerImport(b, nil); err == nil {
		t.Error("no FILE accepted")
	}
}

func TestLedgerOpenErrorWithheld(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "rows.jsonl")
	if err := os.WriteFile(file, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func(*app) error{
		"ledger":        func(a *app) error { return ledgerCmd(a, nil) },
		"ledger-import": func(a *app) error { return ledgerImport(a, []string{file}) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, _ := testApp(t, ledgerDSNLine)
			a.deps.openOptionLedgerStore = func(context.Context, string) (store.OptionLedgerStore, error) {
				return nil, errors.New("dial led-dsn-secret failed")
			}
			err := run(a)
			if err == nil || strings.Contains(err.Error(), "led-dsn-secret") {
				t.Fatalf("error %v", err)
			}
		})
	}
}
