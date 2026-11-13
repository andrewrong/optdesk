package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/underlyingresolve"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
	"github.com/andrewrong/optdesk/internal/store/stockreg"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/underlyingpg"
)

func init() { register("underlying", underlyingCmd) }

const underlyingUsage = "usage: ot underlying import [--check] <watch.toml>"

// underlyingCmd is `ot underlying import [--check] <watch.toml>`: the only sub-command is import. It seeds the
// stored underlying list (settings.underlying) from the file, or with --check compares the two. The store is
// opened with the line OT_PG_DSN_SETTINGS of the env file; the file is only read.
func underlyingCmd(a *app, args []string) error {
	if len(args) == 0 || args[0] != "import" {
		return fmt.Errorf("underlying: the only sub-command is import; %s", underlyingUsage)
	}
	var file string
	check := false
	for _, s := range args[1:] {
		switch {
		case s == "--check":
			check = true
		case strings.HasPrefix(s, "-") || file != "":
			return errors.New(underlyingUsage)
		default:
			file = s
		}
	}
	if file == "" {
		return errors.New(underlyingUsage)
	}
	data, err := os.ReadFile(file) //nolint:gosec // the path is an argument chosen by the owner
	if err != nil {
		return errors.New("underlying import: cannot read the file")
	}
	env, err := a.envFile()
	if err != nil {
		return err
	}
	ctx := context.Background()
	var reg stockreg.StockRegistrar
	if !check {
		var extra closerSet
		defer extra.closeAll()
		if reg, err = openStockRegistrar(ctx, a.deps.openInstrument, env[store.DSNEnvMaster], &extra); err != nil {
			return err
		}
	}
	return underlyingImport(ctx, underlyingpg.Open, env[ownerset.DSNEnvSettings], data, check, a.stdout, reg)
}

// underlyingImport does the work of the command over the opener open, the DSN and the bytes of the file. A non-nil
// reg is called for each added entry with a con_id after the write; nil writes no company data.
func underlyingImport(ctx context.Context, open underlying.OpenFunc, dsn string, data []byte, check bool, out io.Writer,
	reg stockreg.StockRegistrar) error {
	if dsn == "" {
		return errors.New("underlying import needs the line " + ownerset.DSNEnvSettings + " in the env file")
	}
	entries, err := parseWatch(data)
	if err != nil {
		return err
	}
	st, err := open(ctx, dsn)
	if err != nil {
		// The text of an open error may hold the DSN: it is withheld.
		return errors.New("open the settings store with " + ownerset.DSNEnvSettings + " failed")
	}
	defer func() { _ = st.Close() }()
	rows, err := st.Current(ctx)
	if err != nil {
		return errors.New("underlying import: cannot read the stored list")
	}
	have := make(map[string]bool, len(rows))
	for _, r := range rows {
		have[r.Key] = true
	}
	if check {
		return underlyingCheck(entries, have, out)
	}
	var change underlying.Change
	var pending []string
	existing := 0
	for _, e := range entries {
		key := watchKey(e)
		if have[key] {
			existing++
			continue
		}
		have[key] = true
		if e.Contract.ConID == 0 {
			pending = append(pending, key)
		}
		change.Adds = append(change.Adds, underlying.Add{
			Symbol: strings.ToUpper(e.Contract.Symbol), Market: e.Market, Kind: underlying.KindStock,
			Exchange: e.Contract.Exchange, Currency: e.Contract.Currency, Issuer: e.Issuer,
			ConID: e.Contract.ConID, Options: true, Fundamentals: true,
			BuyPrice: e.BuyPrice, SalePrice: e.SalePrice, ReportDateOverride: e.ReportDateOverride,
		})
	}
	added := 0
	if len(change.Adds) > 0 {
		added, err = st.Apply(ctx, change)
		if err != nil {
			var ie underlying.InvalidError
			if errors.As(err, &ie) {
				return fmt.Errorf("underlying import refused: %s: %s", ie.Field(), ie.Why())
			}
			return errors.New("underlying import: the store refused the write")
		}
	}
	_, _ = fmt.Fprintf(out, "underlying import added=%d existing=%d pending=%d\n", added, existing, len(pending))
	for _, k := range pending {
		_, _ = fmt.Fprintf(out, "pending %s\n", k)
	}
	if reg == nil {
		_, _ = fmt.Fprintln(out, "no company data is written: "+store.DSNEnvMaster+" is not set or its store cannot register stocks")
		return nil
	}
	return registerAdded(ctx, reg, change.Adds, out)
}

// registerAdded registers each added entry that has a con_id. A Permanent refusal is printed with its reason and
// leaves the exit status alone; any other error stops the loop and is returned, naming the entry.
func registerAdded(ctx context.Context, reg stockreg.StockRegistrar, adds []underlying.Add, out io.Writer) error {
	for _, a := range adds {
		if a.ConID == 0 {
			continue
		}
		key := a.Market + ":" + a.Symbol
		row := underlying.Row{
			Key: key, Symbol: a.Symbol, Market: a.Market, Kind: a.Kind, Exchange: a.Exchange, Currency: a.Currency,
			Name: a.Name, Issuer: a.Issuer, ConID: a.ConID,
		}
		if _, err := reg.RegisterStock(ctx, underlyingresolve.RegistrationOf(row)); err != nil {
			var te contracts.TypedError
			if errors.As(err, &te) && te.Kind() == contracts.Permanent {
				_, _ = fmt.Fprintf(out, "company registration refused %s: %s\n", key, err.Error())
				continue
			}
			return fmt.Errorf("underlying import: company registration of %s failed", key)
		}
	}
	return nil
}

// watchKey is the name_key the store gives an entry: market, a colon and the upper-case symbol.
func watchKey(e jobs.WatchEntry) string { return e.Market + ":" + strings.ToUpper(e.Contract.Symbol) }

// underlyingCheck prints the two numbers and fails unless they are equal and every file name is in the store.
func underlyingCheck(entries []jobs.WatchEntry, have map[string]bool, out io.Writer) error {
	found, missing := 0, 0
	for _, e := range entries {
		if have[watchKey(e)] {
			found++
		} else {
			missing++
		}
	}
	_, _ = fmt.Fprintf(out, "underlying import --check file=%d store=%d missing=%d\n", len(entries), found, missing)
	if found != len(entries) || missing > 0 {
		return &exitError{code: 1, err: errors.New("underlying import --check: the file and the store differ")}
	}
	return nil
}
