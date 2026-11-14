package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/replay"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
	"github.com/andrewrong/optdesk/internal/strategy/shortput"
)

// Design: docs/design/strategy-replay.md#3 ot-strategy-replay.

func init() { register("strategy-replay", strategyReplayCmd) }

const (
	lotsShortPuts = "short_puts"
	lotsShortLots = "short_lots"
	// replayMinLots is the trial minimum of lots of a line (Decision 4).
	replayMinLots = 20
)

// replayOpts are the flags of `ot strategy-replay`.
type replayOpts struct {
	lots, alias, asOf, earlyKeep, earlyDTE string
	minLots                                int
}

func parseStrategyReplay(args []string) (replayOpts, error) {
	var o replayOpts
	fs := flag.NewFlagSet("strategy-replay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.lots, "lots", lotsShortPuts, "lot set: "+lotsShortPuts+" or "+lotsShortLots)
	fs.StringVar(&o.alias, "alias", "", "alias of the account (default: every stored alias)")
	fs.StringVar(&o.asOf, "as-of", "", "as-of day YYYY-MM-DD (default: today in "+marketZone+")")
	fs.StringVar(&o.earlyKeep, "early-keep", "0.50", "early rule: share of the credit kept at which the lot is closed")
	fs.StringVar(&o.earlyDTE, "early-dte", "21", "early rule: days to expiry at which the lot is closed")
	fs.IntVar(&o.minLots, "min-lots", replayMinLots, "a line over fewer lots carries no conclusion")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, errors.New("usage: ot strategy-replay [--lots short_puts|short_lots] [--alias a] [--as-of DAY] [--early-keep K] [--early-dte N] [--min-lots N]")
	}
	if o.lots != lotsShortPuts && o.lots != lotsShortLots {
		return o, fmt.Errorf("--lots %q is not %s or %s", o.lots, lotsShortPuts, lotsShortLots)
	}
	if o.minLots < 0 {
		return o, errors.New("--min-lots is negative")
	}
	return o, nil
}

// replayVariants are the early variant of the flags (the engine prices hold itself) and the hold strategy whose
// hash the header prints. A bad --early-keep or --early-dte is the error of shortput.New, which names the key.
func replayVariants(o replayOpts) (hold replayapi.Variant, early []replayapi.Variant, err error) {
	keep, ok := new(big.Rat).SetString(o.earlyKeep)
	if !ok {
		return hold, nil, errors.New("--early-keep is not a decimal")
	}
	if _, err := strconv.Atoi(o.earlyDTE); err != nil {
		return hold, nil, errors.New("--early-dte is not an integer")
	}
	build := func(v shortput.Values) (strategy.Strategy, error) {
		return shortput.New(nil, compute.CandidateParams{}, v)
	}
	hs, err := build(shortput.Values{CloseMode: shortput.CloseExpiry, EarlyKeep: o.earlyKeep, EarlyDTE: o.earlyDTE})
	if err != nil {
		return hold, nil, fmt.Errorf("--early-keep or --early-dte: %w", err)
	}
	es, err := build(shortput.Values{CloseMode: shortput.CloseEarly, EarlyKeep: o.earlyKeep, EarlyDTE: o.earlyDTE})
	if err != nil {
		return hold, nil, fmt.Errorf("--early-keep or --early-dte: %w", err)
	}
	pct := strings.TrimRight(strings.TrimRight(new(big.Rat).Mul(keep, big.NewRat(100, 1)).FloatString(4), "0"), ".")
	hold = replayapi.Variant{Name: replay.HoldVariant, Strategy: hs}
	early = []replayapi.Variant{{Name: "early_" + pct + "_" + o.earlyDTE, Strategy: es}}
	return hold, early, nil
}

// strategyReplayCmd is `ot strategy-replay`: it builds his short lots from the option ledger, resolves their stored
// options and closes, replays them under hold and the early rule of the flags and prints the report with a count
// beside every figure. It reads with OT_PG_DSN_OPTION_LEDGER, OT_PG_DSN_PAGES, OT_PG_DSN_RUNNER and OT_PG_DSN_BAR
// and writes nothing.
func strategyReplayCmd(a *app, args []string) error {
	o, err := parseStrategyReplay(args)
	if err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return errors.New("strategy-replay needs a real profile; the demo profile opens no store")
	}
	hold, early, err := replayVariants(o)
	if err != nil {
		return err
	}
	asOf, err := replayAsOf(a, o.asOf)
	if err != nil {
		return err
	}
	env, err := a.replayEnv()
	if err != nil {
		return err
	}
	lots, mkt, sessions, err := a.replayLots(env, o.lots, o.alias, asOf)
	if err != nil {
		return err
	}
	rep := replay.New().Replay(lots, mkt, early, sessions, asOf, o.minLots)
	names := []replayName{{Name: hold.Name, Hash: hold.Strategy.Params().Hash}}
	for _, v := range early {
		names = append(names, replayName{Name: v.Name, Hash: v.Strategy.Params().Hash})
	}
	printReplay(a.stdout, replayHeader{Lots: o.lots, AsOf: asOf, Variants: names, MinLots: o.minLots}, rep)
	return nil
}

func replayAsOf(a *app, flagDay string) (contracts.Day, error) {
	if flagDay != "" {
		t, err := time.Parse(dayLayout, flagDay)
		if err != nil {
			return "", fmt.Errorf("--as-of %q is not YYYY-MM-DD", flagDay)
		}
		return contracts.Day(t.Format(dayLayout)), nil
	}
	return dayOrDefault(a.deps.newClock(), "")
}

// replayPositions reads the stored rows of the alias (every stored alias when empty) with the ledger line and
// replays them; each position id carries its alias.
func (a *app) replayPositions(ctx context.Context, dsn, alias string, asOf contracts.Day) ([]compute.LedgerPosition, error) {
	st, err := a.deps.openOptionLedgerStore(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open option_ledger store with %s failed", store.DSNEnvOptionLedger)
	}
	defer func() { _ = st.Close() }()
	aliases := []string{alias}
	if alias == "" {
		if aliases, err = st.StoredAliases(ctx); err != nil {
			return nil, errors.New("read the stored aliases failed")
		}
	}
	rule := a.deps.newOptionLedger()
	var out []compute.LedgerPosition
	for _, al := range aliases {
		trades, err := ledgerTrades(ctx, st, al)
		if err != nil {
			return nil, err
		}
		res := rule.Replay(compute.LedgerInput{Alias: al, Trades: trades, AsOf: asOf}, compute.LedgerParams{MarkMaxAgeSessions: ledgerMarkAge})
		for _, p := range res.Positions {
			p.PositionID = al + "/" + p.PositionID
			out = append(out, p)
		}
	}
	return out, nil
}

// replaySessions reads the XNYS session days from..to with the runner line.
func (a *app) replaySessions(ctx context.Context, dsn string, from, to contracts.Day) ([]contracts.Day, error) {
	sd, err := a.deps.openSessionDay(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open session-day store with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sd.Close() }()
	rows, err := sd.SessionDays(ctx, ledgerCalendar, from, to)
	if err != nil {
		return nil, errors.New("read session days failed")
	}
	var out []contracts.Day
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	slices.Sort(out)
	return out, nil
}

// openReplayStores opens the mark readers with the pages line and the bar reader with the bar line, read-only
// (the bar store is held as its reader interface only). A failed open is a fixed text naming the line.
func (a *app) openReplayStores(ctx context.Context, env map[string]string) (markReaders, bar.PriceBarReader, func(), error) {
	var closers []func() error
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}
	fail := func(line string) (markReaders, bar.PriceBarReader, func(), error) {
		closeAll()
		return markReaders{}, nil, func() {}, fmt.Errorf("open the stores with %s failed", line)
	}
	var rd markReaders
	d, dsn := a.deps, env[store.DSNEnvPages]
	cat, err := d.openOptionCatalog(ctx, dsn)
	if err != nil {
		return fail(store.DSNEnvPages)
	}
	closers, rd.catalog = append(closers, cat.Close), cat
	eod, err := d.openOptionEod(ctx, dsn)
	if err != nil {
		return fail(store.DSNEnvPages)
	}
	closers, rd.eod = append(closers, eod.Close), eod
	terms, err := d.openOptionTerms(ctx, dsn)
	if err != nil {
		return fail(store.DSNEnvPages)
	}
	closers, rd.terms = append(closers, terms.Close), terms
	names, err := d.openInstrumentName(ctx, dsn)
	if err != nil {
		return fail(store.DSNEnvPages)
	}
	closers, rd.names = append(closers, names.Close), names
	bs, err := d.openBar(ctx, env[bar.DSNEnvBar])
	if err != nil {
		return fail(bar.DSNEnvBar)
	}
	closers = append(closers, bs.Close)
	return rd, bs, closeAll, nil
}

// replayEnv reads the four lines of the command; an unset one is an error naming it, and a missing opener is
// "not wired".
func (a *app) replayEnv() (map[string]string, error) {
	env, err := a.envFile()
	if err != nil {
		return nil, err
	}
	for _, name := range []string{store.DSNEnvOptionLedger, store.DSNEnvPages, store.DSNEnvRunner, bar.DSNEnvBar} {
		if env[name] == "" {
			return nil, errors.New(name + " is not set")
		}
	}
	d := a.deps
	if d.openOptionLedgerStore == nil || d.openSessionDay == nil || d.openBar == nil || d.newOptionLedger == nil ||
		d.openOptionCatalog == nil || d.openOptionEod == nil || d.openOptionTerms == nil || d.openInstrumentName == nil {
		return nil, errors.New("strategy-replay is not wired")
	}
	return env, nil
}

// replayLots reads the ledger, the session days and the market and returns the priced lots; no short lot gives
// none and the stores of the market are not opened.
func (a *app) replayLots(env map[string]string, set, alias string, asOf contracts.Day) ([]replayapi.Lot, replayapi.Market, []contracts.Day, error) {
	ctx := context.Background()
	var mkt replayapi.Market
	positions, err := a.replayPositions(ctx, env[store.DSNEnvOptionLedger], alias, asOf)
	if err != nil {
		return nil, mkt, nil, err
	}
	short := shortLots(positions, set)
	if len(short) == 0 {
		return nil, mkt, nil, nil
	}
	from, to := lotRange(short, asOf)
	sessions, err := a.replaySessions(ctx, env[store.DSNEnvRunner], from, to)
	if err != nil {
		return nil, mkt, nil, err
	}
	rd, bars, closeAll, err := a.openReplayStores(ctx, env)
	if err != nil {
		return nil, mkt, nil, err
	}
	defer closeAll()
	mkt, lots, err := resolveMarket(ctx, rd, bars, buildLots(short, sessions))
	if err != nil {
		return nil, mkt, nil, err
	}
	priceAssigned(lots, mkt)
	return lots, mkt, sessions, nil
}
