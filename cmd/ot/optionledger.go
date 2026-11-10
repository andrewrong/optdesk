package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/option-ledger.md#12 optionledger-wire.

func init() { register("ledger", ledgerCmd) }

const (
	jobOptionLedger = "option_ledger"

	// flexFileName is the file of $OT_HOME that holds the Flex logins and the account allow-list.
	flexFileName = "flex.toml"

	// reasonNoLedgerDSN and reasonNoFlexFile are the lines logged when option_ledger is left out; they name a
	// key or a file, never a value.
	reasonNoLedgerDSN = store.DSNEnvOptionLedger + " is not set"
	reasonNoFlexFile  = flexFileName + " is not configured"
	reasonBadFlexFile = flexFileName + " is not valid"

	// ledgerCalendar is the calendar whose session days age the marks.
	ledgerCalendar = "XNYS"
	// ledgerMarkAge is mark_max_age_sessions of job.toml.
	ledgerMarkAge = 5
	// ledgerQuoteDays is how many calendar days of option_eod rows are read for the marks.
	ledgerQuoteDays = 14
	// ledgerSessionDays is how many calendar days of session days are read for the replay.
	ledgerSessionDays = 60
	// ledgerBatch is the largest list of ids one reader call takes.
	ledgerBatch = 500
)

// waitCtx pauses for d and returns early with the context error; it is the Wait of the Flex source.
func waitCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// optionLedgerJob builds option_ledger: the store with the line OT_PG_DSN_OPTION_LEDGER, the logins and the
// allow-list of $OT_HOME/flex.toml, the tokens and account numbers from the variables the file names (read
// through the env file reader). It is nil, with one logged reason, for a deps without the constructor, an
// empty line or a missing or invalid file; the other jobs are unaffected. A login whose token variable is
// empty is passed with a nil Source and a logged reason naming the login and the variable; the job fails
// the result for a required one and records `degraded` for any other. What it opens goes to extra.
func (a *app) optionLedgerJob(ctx context.Context, cfg runOnceConfig, f jobFactory, extra *closerSet) (contracts.Job, error) {
	if f.d.newOptionLedgerJob == nil {
		return nil, nil
	}
	if cfg.ledgerDSN == "" {
		a.log.Warn("job left out", "component", "ot", "job", jobOptionLedger, "status", reasonNoLedgerDSN)
		return nil, nil
	}
	// deps.loadFlexFile is flex.Load; its sentinel ErrFlexNotConfigured (no file) leaves the job out.
	ff, err := f.d.loadFlexFile(filepath.Join(a.getenv("OT_HOME"), flexFileName))
	switch {
	case errors.Is(err, sources.ErrFlexNotConfigured):
		a.log.Warn("job left out", "component", "ot", "job", jobOptionLedger, "status", reasonNoFlexFile)
		return nil, nil
	case err != nil:
		// the text of a load error names the entry and the field, never a value
		a.log.Error("job left out", "component", "ot", "job", jobOptionLedger, "status", reasonBadFlexFile, "err", err.Error())
		return nil, nil
	}
	env, err := a.envFile()
	if err != nil {
		return nil, err
	}
	lc, err := a.flexConfig(ff, env, f)
	if err != nil {
		return nil, err
	}
	st, err := f.d.openOptionLedgerStore(ctx, cfg.ledgerDSN)
	if err != nil {
		return nil, fmt.Errorf("open option_ledger store with %s failed", store.DSNEnvOptionLedger)
	}
	extra.addCloser(st)
	j, err := f.d.newOptionLedgerJob(jobs.OptionLedgerStores{Ledger: st, Read: st}, lc, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build option_ledger job failed: %w", err)
	}
	return j, nil
}

// flexConfig maps the file to the config of the job. A login whose variable is empty gets a nil Source and a
// logged reason; an account whose variable is empty is left out of the allow-list with one.
func (a *app) flexConfig(ff sources.FlexFile, env map[string]string, f jobFactory) (jobs.OptionLedgerConfig, error) {
	lc := jobs.OptionLedgerConfig{Accounts: map[string]string{}}
	for _, l := range ff.Logins {
		run := jobs.FlexRun{Name: l.Name, QueryIDs: l.TradeQueries, Required: l.Required}
		token := env[l.TokenEnv]
		if token == "" {
			a.log.Warn("flex login left out", "component", "ot", "job", jobOptionLedger,
				"status", fmt.Sprintf("login %s: %s is not set", l.Name, l.TokenEnv))
		} else {
			// deps.newFlexSource is flex.New: one source per login, with the token of that login only.
			src, err := f.d.newFlexSource(sources.FlexConfig{Token: token, Wait: waitCtx, Clock: f.clk})
			if err != nil {
				return lc, fmt.Errorf("build Flex source of login %s failed", l.Name)
			}
			run.Source = src
		}
		lc.Logins = append(lc.Logins, run)
	}
	for _, acc := range ff.Accounts {
		num := env[acc.AccountEnv]
		if num == "" {
			a.log.Warn("flex account left out", "component", "ot", "job", jobOptionLedger,
				"status", fmt.Sprintf("account %s: %s is not set", acc.Alias, acc.AccountEnv))
			continue
		}
		lc.Accounts[num] = acc.Alias
	}
	return lc, nil
}

// ledgerOpts are the parsed arguments of ot ledger.
type ledgerOpts struct {
	alias  string
	status string
	cycles bool
}

// ledgerStatuses are the values of --status: the statuses of a position and the two lists apart.
var ledgerStatuses = []string{"open", "closed", "assigned", "expired", "exercised", "unresolved", "review"}

func parseLedger(args []string) (ledgerOpts, error) {
	var o ledgerOpts
	fs := flag.NewFlagSet("ledger", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.alias, "alias", "", "alias of the account (default: every stored alias)")
	fs.StringVar(&o.status, "status", "", "only this status: "+strings.Join(ledgerStatuses, ", "))
	fs.BoolVar(&o.cycles, "cycles", false, "also print one block per wheel cycle")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, errors.New("usage: ot ledger [--alias a] [--status S] [--cycles]")
	}
	if o.status != "" && !slices.Contains(ledgerStatuses, o.status) {
		return o, fmt.Errorf("--status %q is not one of %s", o.status, strings.Join(ledgerStatuses, ", "))
	}
	return o, nil
}

// ledgerCmd is `ot ledger [--alias a] [--status S] [--cycles]`: it reads the stored rows with the line
// OT_PG_DSN_OPTION_LEDGER, replays them through compute.OptionLedger with the marks of option_eod (the line
// OT_PG_DSN_PAGES) and the session days of the runner line, and prints ledgerText. It writes nothing.
func ledgerCmd(a *app, args []string) error {
	o, err := parseLedger(args)
	if err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return errors.New("ledger needs a real profile; the demo profile opens no store")
	}
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
		return fmt.Errorf("open option_ledger store with %s failed", store.DSNEnvOptionLedger)
	}
	defer func() { _ = st.Close() }()
	day, err := dayOrDefault(a.deps.newClock(), "")
	if err != nil {
		return err
	}
	aliases := []string{o.alias}
	if o.alias == "" {
		if aliases, err = st.StoredAliases(ctx); err != nil {
			return errors.New("read the stored aliases failed")
		}
	}
	sessions, closeSessions := a.ledgerSessions(ctx, env[store.DSNEnvRunner], day)
	defer closeSessions()
	rd, closeMarks := a.openMarkReaders(ctx, env[store.DSNEnvPages])
	defer closeMarks()
	rule := a.deps.newOptionLedger()
	for _, alias := range aliases {
		trades, err := ledgerTrades(ctx, st, alias)
		if err != nil {
			return err
		}
		in := compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: sessions, AsOf: day}
		res := rule.Replay(in, compute.LedgerParams{MarkMaxAgeSessions: ledgerMarkAge})
		if in.Marks, err = rd.marks(ctx, res.Positions, day); err != nil {
			return err
		}
		if len(in.Marks) > 0 {
			res = rule.Replay(in, compute.LedgerParams{MarkMaxAgeSessions: ledgerMarkAge})
		}
		ledgerText(a.stdout, alias, day, res, o)
	}
	return nil
}

// ledgerTrades reads the latest rows of alias and decodes their attributes.
func ledgerTrades(ctx context.Context, r store.OptionLedgerReader, alias string) ([]compute.LedgerTrade, error) {
	rows, err := r.FlexTradeRows(ctx, alias)
	if err != nil {
		return nil, errors.New("read the stored rows failed")
	}
	out := make([]compute.LedgerTrade, 0, len(rows))
	for _, row := range rows {
		attrs := map[string]string{}
		if err := json.Unmarshal([]byte(row.Attributes), &attrs); err != nil {
			return nil, fmt.Errorf("a stored row of transaction %s is not a text map", row.TransactionID)
		}
		out = append(out, compute.LedgerTrade{TransactionID: row.TransactionID, Revision: row.Revision, Attributes: attrs})
	}
	return out, nil
}

// ledgerSessions reads the XNYS session days of the last ledgerSessionDays days with the runner line; an empty
// line or a failed read gives none (the marks are then not aged) and one logged reason.
func (a *app) ledgerSessions(ctx context.Context, dsn string, day contracts.Day) ([]contracts.Day, func()) {
	if dsn == "" || a.deps.openSessionDay == nil {
		a.log.Warn("ledger without session days", "component", "ot", "status", store.DSNEnvRunner+" is not set")
		return nil, func() {}
	}
	sd, err := a.deps.openSessionDay(ctx, dsn)
	if err != nil {
		a.log.Warn("ledger without session days", "component", "ot", "status", "open session-day store with "+store.DSNEnvRunner+" failed")
		return nil, func() {}
	}
	from, ok := shiftLedgerDay(day, -ledgerSessionDays)
	if !ok {
		return nil, func() { _ = sd.Close() }
	}
	rows, err := sd.SessionDays(ctx, ledgerCalendar, from, day)
	if err != nil {
		a.log.Warn("ledger without session days", "component", "ot", "status", "read session days failed")
		return nil, func() { _ = sd.Close() }
	}
	var out []contracts.Day
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	return out, func() { _ = sd.Close() }
}

func shiftLedgerDay(day contracts.Day, days int) (contracts.Day, bool) {
	t, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return "", false
	}
	return contracts.Day(t.AddDate(0, 0, days).Format(dayLayout)), true
}

// markReaders are the readers of option_eod; all nil means no marks (the positions print `no_mark`).
type markReaders struct {
	catalog store.OptionEodCatalog
	eod     store.OptionEodReader
	terms   store.OptionTermsReader
	names   store.InstrumentNameReader
}

// openMarkReaders opens the four stores of the marks with the pages line. An empty line or a failed open gives
// no readers and one logged reason.
func (a *app) openMarkReaders(ctx context.Context, dsn string) (markReaders, func()) {
	var rd markReaders
	var closers []func() error
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}
	skip := func(why string) (markReaders, func()) {
		closeAll()
		a.log.Warn("ledger without marks", "component", "ot", "status", why)
		return markReaders{}, func() {}
	}
	d := a.deps
	if dsn == "" || d.openOptionCatalog == nil || d.openOptionEod == nil || d.openOptionTerms == nil || d.openInstrumentName == nil {
		return skip(pagesDSNName + " is not set")
	}
	fail := "open the mark stores with " + pagesDSNName + " failed"
	cat, err := d.openOptionCatalog(ctx, dsn)
	if err != nil {
		return skip(fail)
	}
	closers, rd.catalog = append(closers, cat.Close), cat
	eod, err := d.openOptionEod(ctx, dsn)
	if err != nil {
		return skip(fail)
	}
	closers, rd.eod = append(closers, eod.Close), eod
	terms, err := d.openOptionTerms(ctx, dsn)
	if err != nil {
		return skip(fail)
	}
	closers, rd.terms = append(closers, terms.Close), terms
	names, err := d.openInstrumentName(ctx, dsn)
	if err != nil {
		return skip(fail)
	}
	closers, rd.names = append(closers, names.Close), names
	return rd, closeAll
}

// markKey is the contract of an option position as option_eod names it.
type markKey struct{ underlying, expiry, strike, right string }

func markKeyOf(underlying, expiry, strike, right string) markKey {
	if strings.Contains(strike, ".") {
		strike = strings.TrimRight(strings.TrimRight(strike, "0"), ".")
	}
	right = strings.ToUpper(right)
	if strings.HasPrefix(right, "C") {
		right = "C"
	} else if strings.HasPrefix(right, "P") {
		right = "P"
	}
	return markKey{underlying, expiry, strike, right}
}

// marks returns the newest option_eod quote of every open position of positions, named by the contract key the
// rule matches (LedgerPosition.Contract); the mapping is by underlying name, expiry, strike and right. Nil
// readers or no open position give none.
func (rd markReaders) marks(ctx context.Context, positions []compute.LedgerPosition, day contracts.Day) ([]compute.LedgerMark, error) {
	if rd.catalog == nil {
		return nil, nil
	}
	symbols := map[string]bool{}
	wanted := map[markKey]string{}
	for _, p := range positions {
		if p.Status != "open" {
			continue
		}
		symbols[p.Underlying] = true
		wanted[markKeyOf(p.Underlying, p.Expiry, p.Strike, p.Right)] = p.Contract
	}
	if len(symbols) == 0 {
		return nil, nil
	}
	from, ok := shiftLedgerDay(day, -ledgerQuoteDays)
	if !ok {
		return nil, fmt.Errorf("bad day %q", day)
	}
	latest, err := rd.catalog.OptionEodLatest(ctx, from)
	if err != nil {
		return nil, errors.New("read the option_eod catalog failed")
	}
	var catalogIDs []int64
	for _, l := range latest {
		catalogIDs = append(catalogIDs, l.UnderlyingID)
	}
	nameOf := map[int64]string{}
	var ids []int64
	for lo := 0; lo < len(catalogIDs); lo += ledgerBatch {
		got, err := rd.names.InstrumentNames(ctx, catalogIDs[lo:min(lo+ledgerBatch, len(catalogIDs))])
		if err != nil {
			return nil, errors.New("read the instrument names failed")
		}
		for _, n := range got {
			if symbols[n.Name] {
				nameOf[n.InstrumentID] = n.Name
				ids = append(ids, n.InstrumentID)
			}
		}
	}
	var rows []contracts.OptionEod
	underOf := map[int]string{}
	for _, id := range ids {
		got, err := rd.eod.OptionEod(ctx, id, from, day)
		if err != nil {
			return nil, errors.New("read option_eod failed")
		}
		for _, r := range got {
			underOf[r.InstrumentID] = nameOf[id]
		}
		rows = append(rows, got...)
	}
	return rd.newest(ctx, rows, underOf, wanted)
}

// newest keeps, per wanted contract, the quote of the latest day; terms are read in batches.
func (rd markReaders) newest(ctx context.Context, rows []contracts.OptionEod, underOf map[int]string, wanted map[markKey]string) ([]compute.LedgerMark, error) {
	var instruments []int64
	seen := map[int]bool{}
	for _, r := range rows {
		if !seen[r.InstrumentID] {
			seen[r.InstrumentID] = true
			instruments = append(instruments, int64(r.InstrumentID))
		}
	}
	terms := map[int64]store.OptionTerms{}
	for lo := 0; lo < len(instruments); lo += ledgerBatch {
		got, err := rd.terms.OptionTerms(ctx, instruments[lo:min(lo+ledgerBatch, len(instruments))])
		if err != nil {
			return nil, errors.New("read the option terms failed")
		}
		for _, t := range got {
			terms[t.InstrumentID] = t
		}
	}
	best := map[string]compute.LedgerMark{}
	var order []string
	for _, r := range rows {
		t, ok := terms[int64(r.InstrumentID)]
		if !ok {
			continue
		}
		contract, ok := wanted[markKeyOf(underOf[r.InstrumentID], string(t.ExpirationDate), t.Strike, t.Right)]
		if !ok {
			continue
		}
		m := compute.LedgerMark{Contract: contract, Day: contracts.Day(r.SessionDay.Format(dayLayout)),
			Bid: decimalOf(r.Bid), Ask: decimalOf(r.Ask), UnderlyingClose: decimalOf(r.UnderlyingPrice)}
		old, had := best[contract]
		if !had {
			order = append(order, contract)
		}
		if !had || m.Day >= old.Day {
			best[contract] = m
		}
	}
	out := make([]compute.LedgerMark, 0, len(order))
	for _, c := range order {
		out = append(out, best[c])
	}
	return out, nil
}

// decimalOf is the one place a stored option_eod number becomes decimal text; nil, NaN and infinity are "".
func decimalOf(v *float64) string {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
