package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("ledger-import", ledgerImport) }

// ledgerImport is `ot ledger-import FILE [--map from=to ...]`: it imports the JSON Lines export of option-trader's
// stored Flex rows through deps.importOptionTrader into the store opened with the line OT_PG_DSN_OPTION_LEDGER,
// and prints counts only: `ledger-import lines=N inserted=N same=N conflict=N`. Exit code 1 when a row conflicts
// (it is counted and not written). The file is read by the import, which names a bad line by number.
func ledgerImport(a *app, args []string) error {
	file, aliasMap, err := parseLedgerImport(args)
	if err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return errors.New("ledger-import needs a real profile; the demo profile opens no store")
	}
	if err := a.checkImportMap(aliasMap); err != nil {
		return err
	}
	f, err := os.Open(file) //nolint:gosec // the path is an argument chosen by the owner
	if err != nil {
		return errors.New("cannot read FILE")
	}
	defer func() { _ = f.Close() }()
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn := env[store.DSNEnvOptionLedger]
	if dsn == "" {
		return errors.New(reasonNoLedgerDSN)
	}
	ctx := context.Background()
	st, err := a.deps.openOptionLedgerStore(ctx, dsn)
	if err != nil {
		// The text of an open error may hold the DSN: it is withheld.
		return fmt.Errorf("open option_ledger store with %s failed", store.DSNEnvOptionLedger)
	}
	defer func() { _ = st.Close() }()
	res, err := a.deps.importOptionTrader(ctx, f, st, aliasMap, a.deps.newClock().Now())
	if err != nil {
		return fmt.Errorf("ledger-import: %w", err)
	}
	_, _ = fmt.Fprintf(a.stdout, "ledger-import lines=%d inserted=%d same=%d conflict=%d\n", res.Lines, res.Inserted, res.Same, res.Conflict)
	if res.Conflict > 0 {
		return &exitError{code: 1, err: errors.New("ledger-import: rows conflict with the stored rows (see the counts)")}
	}
	return nil
}

// parseLedgerImport reads FILE (first or last argument) and the --map flags.
func parseLedgerImport(args []string) (string, map[string]string, error) {
	var file string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		file, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("ledger-import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var maps stringList
	fs.Var(&maps, "map", "alias map from=to (repeatable)")
	if err := fs.Parse(args); err != nil {
		return "", nil, err
	}
	if file == "" && fs.NArg() == 1 {
		file = fs.Arg(0)
	} else if fs.NArg() != 0 {
		return "", nil, errors.New("usage: ot ledger-import FILE [--map from=to ...]")
	}
	if file == "" {
		return "", nil, errors.New("ledger-import needs FILE")
	}
	aliasMap, err := parseAliasMaps(maps)
	return file, aliasMap, err
}
