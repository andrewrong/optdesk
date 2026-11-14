package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
	"github.com/andrewrong/optdesk/internal/strategy/shortput"
)

const replayEnv = "OT_PG_DSN_OPTION_LEDGER=led-secret\nOT_PG_DSN_PAGES=pages-secret\nOT_PG_DSN_RUNNER=run-secret\nOT_PG_DSN_BAR=bar-secret\n"

type replayLedgerRule struct{ positions []compute.LedgerPosition }

func (r replayLedgerRule) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{Positions: r.positions}
}

// replaySessions holds every weekday of the range.
type replaySessionDays struct{ store.SessionDayStore }

func (replaySessionDays) SessionDays(_ context.Context, cal string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	f, _ := time.Parse(time.DateOnly, string(from))
	e, _ := time.Parse(time.DateOnly, string(to))
	var out []contracts.SessionDay
	for d := f; !d.After(e); d = d.AddDate(0, 0, 1) {
		out = append(out, contracts.SessionDay{CalendarID: cal, SessionDay: replayDay(d.Format(time.DateOnly)),
			IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday})
	}
	return out, nil
}

func (replaySessionDays) Close() error { return nil }

// replayRig wires fakes into an app: one expired short put on option 101 whose first quote day has an ask that
// makes the 50 percent rule close it.
func replayRig(t *testing.T) (*app, *bytes.Buffer) {
	t.Helper()
	a, _ := testApp(t, replayEnv)
	var out bytes.Buffer
	a.stdout = &out
	pos := shortPos("held", "expired", "2026-11-09", "2026-11-13", "2026-11-13", "100")
	pos.Quantity = "-1"
	pos.RealizedPnL = "200"
	t0 := time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	rows := []contracts.OptionEod{replayEod("2026-11-10", eodOK, t0, "a", fptr(0.7), fptr(0.80))}
	d := &a.deps
	d.openOptionLedgerStore = func(_ context.Context, dsn string) (store.OptionLedgerStore, error) {
		if dsn != "led-secret" {
			t.Errorf("ledger dsn %q", dsn)
		}
		return &ledgerFakeStore{aliases: []string{"main"}}, nil
	}
	d.newOptionLedger = func() compute.OptionLedger { return replayLedgerRule{positions: []compute.LedgerPosition{pos}} }
	d.openSessionDay = func(_ context.Context, dsn string) (store.SessionDayStore, error) {
		if dsn != "run-secret" {
			t.Errorf("session dsn %q", dsn)
		}
		return replaySessionDays{}, nil
	}
	rd := replayReaders(rows)
	rd.terms = replayTerms{terms: []store.OptionTerms{{InstrumentID: 101, Right: "P", Strike: "100.00", ExpirationDate: "2026-11-13"}}}
	d.openOptionCatalog = func(_ context.Context, dsn string) (store.OptionEodCatalogStore, error) {
		if dsn != "pages-secret" {
			t.Errorf("catalog dsn %q", dsn)
		}
		return rd.catalog.(replayCatalog), nil
	}
	d.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) { return rd.eod.(replayEodStore), nil }
	d.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) { return rd.terms.(replayTerms), nil }
	d.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) { return replayNames{}, nil }
	d.openBar = func(_ context.Context, dsn string) (bar.BarStore, error) {
		if dsn != "bar-secret" {
			t.Errorf("bar dsn %q", dsn)
		}
		return replayBars{closes: map[string]map[string]string{
			"trades":        {"2026-11-09": "101", "2026-11-13": "105"},
			"adjusted_last": {"2026-11-09": "101", "2026-11-13": "105"},
		}}, nil
	}
	return a, &out
}

func TestStrategyReplayDefaultsToShortPuts(t *testing.T) {
	t.Parallel()
	o, err := parseStrategyReplay(nil)
	if err != nil {
		t.Fatal(err)
	}
	if o.lots != "short_puts" || o.minLots != 20 || o.earlyKeep != "0.50" || o.earlyDTE != "21" {
		t.Fatalf("defaults %+v", o)
	}
	if o, err = parseStrategyReplay([]string{"--lots", "short_lots", "--alias", "main", "--as-of", "2026-11-20", "--min-lots", "5"}); err != nil ||
		o.lots != "short_lots" || o.alias != "main" || o.asOf != "2026-11-20" || o.minLots != 5 {
		t.Fatalf("flags %+v, %v", o, err)
	}
	_, err = parseStrategyReplay([]string{"--lots", "calls"})
	if err == nil || !strings.Contains(err.Error(), "short_puts") || !strings.Contains(err.Error(), "short_lots") {
		t.Fatalf("error %v must name both values", err)
	}
}

func TestStrategyReplayWiresEngineAndShortPutVariants(t *testing.T) {
	t.Parallel()
	a, out := replayRig(t)
	if err := strategyReplayCmd(a, []string{"--as-of", "2026-11-20", "--early-keep", "0.4", "--early-dte", "14"}); err != nil {
		t.Fatal(err)
	}
	hash := func(mode string) string {
		s, err := shortput.New(nil, compute.CandidateParams{}, shortput.Values{CloseMode: mode, EarlyKeep: "0.4", EarlyDTE: "14"})
		if err != nil {
			t.Fatal(err)
		}
		return s.Params().Hash
	}
	text := out.String()
	for _, want := range []string{
		"variant hold params_hash=" + hash("expiry"),
		"variant early_40_14 params_hash=" + hash("early"),
		"stored quote days: first=2026-11-10 last=2026-11-10",
		"lots set: short_puts",
		"as-of day: 2026-11-20",
		"set=all variant=hold currency=USD lots=1",
		"set=priced:early_40_14 variant=early_40_14 currency=USD lots=1",
		"under 20 [T]: no conclusion",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	if !strings.HasPrefix(text, "stored quote days:") {
		t.Errorf("the first line is not the stored quote days:\n%s", text)
	}
}

func TestStrategyReplayDemoRefused(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	err := strategyReplayCmd(a, nil)
	if err == nil || !strings.Contains(err.Error(), "demo profile") {
		t.Fatalf("error %v", err)
	}
}

func TestStrategyReplayOpenErrorWithheld(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		break_ func(d *deps)
		line   string
	}{
		"ledger": {func(d *deps) {
			d.openOptionLedgerStore = func(context.Context, string) (store.OptionLedgerStore, error) {
				return nil, errors.New("dial led-secret refused")
			}
		}, store.DSNEnvOptionLedger},
		"sessions": {func(d *deps) {
			d.openSessionDay = func(context.Context, string) (store.SessionDayStore, error) {
				return nil, errors.New("dial run-secret refused")
			}
		}, store.DSNEnvRunner},
		"pages": {func(d *deps) {
			d.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
				return nil, errors.New("dial pages-secret refused")
			}
		}, store.DSNEnvPages},
		"bar": {func(d *deps) {
			d.openBar = func(context.Context, string) (bar.BarStore, error) { return nil, errors.New("dial bar-secret refused") }
		}, "OT_PG_DSN_BAR"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, _ := replayRig(t)
			c.break_(&a.deps)
			err := strategyReplayCmd(a, []string{"--as-of", "2026-11-20"})
			if err == nil || !strings.Contains(err.Error(), c.line) || strings.Contains(err.Error(), "secret") {
				t.Fatalf("error %v", err)
			}
		})
	}
}

func TestStrategyReplayPrintsCountsBesideFigures(t *testing.T) {
	t.Parallel()
	rep := replayapi.Report{
		Lots: 7, LotsWithQuote: 2, StillOpen: 1, LotDays: 5, QuoteDays: 3,
		FirstQuoteDay: "2026-11-07", LastQuoteDay: "2026-11-13",
		Excluded: map[string]int{"hold:not_expired": 1, "early_50_21:gap_before_close": 4, "hold:no_settle_price": 2},
		Lines: []replayapi.Line{
			{Set: "all", Variant: "hold", Currency: "USD", Lots: 25, KeptShare: "0.8", MeanGross: "1.2", WorstGross: "-9"},
			{Set: "priced:early_50_21", Variant: "early_50_21", ParamHash: "abc", Currency: "USD", Lots: 3, KeptShare: "0.6",
				MeanGross: "0.9", WorstGross: "-2", HoldKeptShare: "0.5", ActualKeptShare: "0.7", UnderMinimum: true},
		},
	}
	var out bytes.Buffer
	h := replayHeader{Lots: "short_puts", AsOf: "2026-11-14", MinLots: 20,
		Variants: []replayName{{"hold", "h1"}, {"early_50_21", "abc"}}}
	printReplay(&out, h, rep)
	text := out.String()
	lines := strings.Split(text, "\n")
	if lines[0] != "stored quote days: first=2026-11-07 last=2026-11-13" {
		t.Errorf("first line %q", lines[0])
	}
	for _, want := range []string{
		"early assignment is not modelled; a day without a stored quote is a gap, not a guess",
		"the stored quote is the morning snapshot 90 minutes after the open; settlement is the close",
		"coverage: lots=7 with_quote_after_entry=2 lot_days=5 quote_days=3",
		"still open: 1",
		"variant early_50_21 params_hash=abc",
		"kept_share=0.8 (n=25) mean_gross=1.2 (n=25) worst_gross=-9 (n=25)",
		"kept_share=0.6 (n=3) mean_gross=0.9 (n=3) worst_gross=-2 (n=3) hold_kept_share=0.5 (n=3) actual_kept_share=0.7 (n=3) under 20 [T]: no conclusion",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	// Exclusions come sorted, whatever the map order.
	i1 := strings.Index(text, "excluded early_50_21:gap_before_close: 4")
	i2 := strings.Index(text, "excluded hold:no_settle_price: 2")
	i3 := strings.Index(text, "excluded hold:not_expired: 1")
	if i1 < 0 || i1 > i2 || i2 > i3 {
		t.Errorf("exclusions out of order:\n%s", text)
	}
	if strings.Count(text, "under 20 [T]: no conclusion") != 1 {
		t.Errorf("the words are printed for a line over the minimum:\n%s", text)
	}
	var again bytes.Buffer
	printReplay(&again, h, rep)
	if again.String() != text {
		t.Error("two prints of one report differ")
	}
}

func TestStrategyReplayPrintsRolledNote(t *testing.T) {
	t.Parallel()
	rep := replayapi.Report{Lines: []replayapi.Line{{Set: "rolled", Variant: "actual", Currency: "USD", Lots: 2, Rolled: 2}}}
	var out bytes.Buffer
	printReplay(&out, replayHeader{Lots: "short_puts", AsOf: "2026-11-14", MinLots: 20}, rep)
	text := out.String()
	if !strings.Contains(text, "a rolled lot's actual does not include the lot it was rolled into") {
		t.Errorf("no rolled note:\n%s", text)
	}
	if !strings.Contains(text, "set=rolled variant=actual currency=USD lots=2") || !strings.Contains(text, "rolled=2") {
		t.Errorf("no rolled row:\n%s", text)
	}
	if !strings.HasPrefix(text, "stored quote days: none\n") {
		t.Errorf("first line:\n%s", text)
	}
}
