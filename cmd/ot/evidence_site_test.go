package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// chainDays is a DayReader with a fixed chain; it counts its ChainDay calls and records the underlying.
type chainDays struct {
	optionday.DayReader
	rows  []optionday.Day
	calls atomic.Int32
	under atomic.Int64
}

func (f *chainDays) ChainDay(_ context.Context, u int64, _ contracts.Day) ([]optionday.Day, error) {
	f.calls.Add(1)
	f.under.Store(u)
	return f.rows, nil
}

// chainTerms answers the terms of every id but those in skip and records the size of each call.
type chainTerms struct {
	skip  map[int64]bool
	sizes []int
}

func (f *chainTerms) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	f.sizes = append(f.sizes, len(ids))
	var out []store.OptionTerms
	for _, id := range ids {
		if f.skip[id] {
			continue
		}
		out = append(out, store.OptionTerms{InstrumentID: id, Right: "P", Strike: "90.00", ExpirationDate: "2026-12-18"})
	}
	return out, nil
}

// chainPicker records the chain, expiry, spot and source it was given.
type chainPicker struct {
	chain  []optionevidenceapi.ChainQuote
	source string
}

func (p *chainPicker) ATM(chain []optionevidenceapi.ChainQuote, _ contracts.Day, _, source string) (string, string) {
	p.chain, p.source = chain, source
	return "0.294", "95.00"
}

func chainRows(n int) []optionday.Day {
	iv := "0.30"
	rows := make([]optionday.Day, n)
	for i := range rows {
		rows[i] = optionday.Day{InstrumentID: int64(i + 1), Source: "option_eod", Iv: &iv}
	}
	return rows
}

func TestATMForJoinsChainAndTerms(t *testing.T) {
	t.Parallel()
	days, p := &chainDays{rows: chainRows(2)}, &chainPicker{}
	f := newATMFor(days, &chainTerms{}, p)
	iv, strike, err := f(t.Context(), 7, "2026-11-17", "2026-12-18", "96.50")
	if err != nil || iv != "0.294" || strike != "95.00" {
		t.Fatalf("%q %q %v", iv, strike, err)
	}
	want := optionevidenceapi.ChainQuote{Expiry: "2026-12-18", Right: "P", Strike: "90.00", IV: "0.30", Source: "option_eod"}
	if len(p.chain) != 2 || p.chain[0] != want || p.source != "option_eod" || days.under.Load() != 7 {
		t.Fatalf("chain %+v source %q under %d", p.chain, p.source, days.under.Load())
	}
}

func TestATMForChunksOf500(t *testing.T) {
	t.Parallel()
	ft := &chainTerms{}
	f := newATMFor(&chainDays{rows: chainRows(501)}, ft, &chainPicker{})
	if _, _, err := f(t.Context(), 1, "2026-11-17", "2026-12-18", "96.50"); err != nil {
		t.Fatal(err)
	}
	if len(ft.sizes) != 2 || ft.sizes[0] != 500 || ft.sizes[1] != 1 {
		t.Fatalf("calls %v", ft.sizes)
	}
}

func TestATMForOncePerUnderlyingDay(t *testing.T) {
	t.Parallel()
	days := &chainDays{rows: chainRows(2)}
	f := newATMFor(days, &chainTerms{}, &chainPicker{})
	for _, e := range []contracts.Day{"2026-12-18", "2027-01-15"} {
		if _, _, err := f(t.Context(), 1, "2026-11-17", e, "96.50"); err != nil {
			t.Fatal(err)
		}
	}
	if days.calls.Load() != 1 {
		t.Fatalf("ChainDay calls %d", days.calls.Load())
	}
}

func TestATMForNoTermsRowSkipped(t *testing.T) {
	t.Parallel()
	p := &chainPicker{}
	f := newATMFor(&chainDays{rows: chainRows(3)}, &chainTerms{skip: map[int64]bool{2: true}}, p)
	if _, _, err := f(t.Context(), 1, "2026-11-17", "2026-12-18", "96.50"); err != nil {
		t.Fatal(err)
	}
	if len(p.chain) != 2 {
		t.Fatalf("chain %+v", p.chain)
	}
}

func TestATMForNilReaderIsNil(t *testing.T) {
	t.Parallel()
	d, tm, p := &chainDays{}, &chainTerms{}, &chainPicker{}
	if newATMFor(nil, tm, p) != nil || newATMFor(d, nil, p) != nil || newATMFor(d, tm, nil) != nil {
		t.Fatal("want nil")
	}
	if newATMFor(d, tm, p) == nil {
		t.Fatal("want a function")
	}
}

func TestATMForReaderErrorReturned(t *testing.T) {
	t.Parallel()
	f := newATMFor(errDays{}, &chainTerms{}, &chainPicker{})
	if _, _, err := f(t.Context(), 1, "2026-11-17", "2026-12-18", "96.50"); err == nil {
		t.Fatal("want error")
	}
}

type errDays struct{ optionday.DayReader }

func (errDays) ChainDay(context.Context, int64, contracts.Day) ([]optionday.Day, error) {
	return nil, errors.New("boom")
}

func TestEvidenceConfigWired(t *testing.T) {
	t.Parallel()
	cfg := candidatesConfig(time.UTC, "r", "b", "candidates", candidateReaders{View: demoView{}, Terms: demoOption{}, Days: &chainDays{}})
	if cfg.Evidence == nil || cfg.ATMFor == nil || cfg.Readings == nil {
		t.Fatalf("config %+v", cfg)
	}
	ev, err := cfg.Evidence.Evaluate(optionevidenceapi.Row{
		ChainDay: "2026-11-17", Expiry: "2026-12-18", DTE: 31, Spot: "96.50", Strike: "90.00", Bid: "0.94", IV: "0.312",
	})
	if err != nil || ev.BreakEven != "89.06" || ev.OneSigma != "8.7744" {
		t.Fatalf("%+v %v", ev, err)
	}
	if bare := candidatesConfig(time.UTC, "r", "b", "candidates", candidateReaders{}); bare.Evidence == nil || bare.ATMFor != nil {
		t.Fatalf("bare config %+v", bare)
	}
}
