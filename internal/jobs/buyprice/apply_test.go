package buyprice_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/buyprice"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// fakeReader counts the reads and answers one fixed state.
type fakeReader struct {
	valuation.Reader
	st    valuation.State
	err   error
	reads int
}

func (f *fakeReader) Current(context.Context) (valuation.State, error) {
	f.reads++
	return f.st, f.err
}

// fakeResolver answers by the input: a zone for the symbol gives the valuation price "91.123456",
// else the hand price, else none. It records every input.
type fakeResolver struct{ seen []buypriceapi.Input }

func (f *fakeResolver) Resolve(in buypriceapi.Input) buypriceapi.Result {
	f.seen = append(f.seen, in)
	for _, z := range in.Zones {
		if z.Symbol == in.Symbol && z.Currency == in.Currency {
			return buypriceapi.Result{Price: "91.123456", Source: buypriceapi.SourceValuation}
		}
	}
	if in.Hand != "" {
		return buypriceapi.Result{Price: in.Hand, Source: buypriceapi.SourceHand}
	}
	return buypriceapi.Result{Source: buypriceapi.SourceNone}
}

func state() valuation.State {
	return valuation.State{
		Entries: []valuation.Entry{
			{
				Profile: valuation.Profile{CompanyKey: "k1", Symbol: "AAA", Currency: "USD", RMarket: "US", Pool: "in_pool"},
				Current: &valuation.Review{V: "100", P: "3", VersionDate: "2026-11-17"},
			},
			{Profile: valuation.Profile{CompanyKey: "k2", Symbol: "BBB", Currency: "JPY", RMarket: "JP", Pool: "watch"}},
		},
		RVersion: &valuation.RVersion{RUS: "0.08", RJP: "0.07", RHKCN: "0.09"},
		Margin:   &valuation.MarginTableVersion{Rows: []valuation.MarginTableRow{{PFrom: "1", M: "0.1", W: "0.2", H: "0.3", S: "0.4"}}},
	}
}

func entry(issuer, symbol, cur, hand string) jobs.WatchEntry {
	return jobs.WatchEntry{
		Issuer: issuer, Market: "US", BuyPrice: hand, SalePrice: "200", ReportDateOverride: "2026-12-01",
		Contract: broker.Contract{Symbol: symbol, Currency: cur},
	}
}

func TestApplyValuationWins(t *testing.T) {
	t.Parallel()
	out, err := buyprice.Apply(context.Background(), &fakeReader{st: state()}, &fakeResolver{}, []jobs.WatchEntry{entry("AAA", "AAA", "USD", "90.00")})
	if err != nil || len(out) != 1 {
		t.Fatalf("got %v %v", out, err)
	}
	if out[0].BuyPrice != "91.123456" || out[0].BuySource != buypriceapi.SourceValuation {
		t.Errorf("got %q %q", out[0].BuyPrice, out[0].BuySource)
	}
}

func TestApplyHandAndNone(t *testing.T) {
	t.Parallel()
	in := []jobs.WatchEntry{entry("CCC", "CCC", "USD", "41.00"), entry("DDD", "DDD", "USD", "")}
	out, err := buyprice.Apply(context.Background(), &fakeReader{st: state()}, &fakeResolver{}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].BuyPrice != "41.00" || out[0].BuySource != buypriceapi.SourceHand {
		t.Errorf("hand: got %q %q", out[0].BuyPrice, out[0].BuySource)
	}
	if out[1].BuyPrice != "" || out[1].BuySource != "" {
		t.Errorf("none: got %q %q", out[1].BuyPrice, out[1].BuySource)
	}
}

func TestApplyReadErrorReturned(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	out, err := buyprice.Apply(context.Background(), &fakeReader{err: boom}, &fakeResolver{}, []jobs.WatchEntry{entry("AAA", "AAA", "USD", "1")})
	if !errors.Is(err, boom) || out != nil {
		t.Fatalf("got %v %v", out, err)
	}
}

func TestApplyNilReaderKeepsHand(t *testing.T) {
	t.Parallel()
	in := []jobs.WatchEntry{entry("AAA", "AAA", "USD", "41.00"), entry("DDD", "DDD", "USD", "")}
	res := &fakeResolver{}
	out, err := buyprice.Apply(context.Background(), nil, res, in)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].BuySource != buypriceapi.SourceHand || out[0].BuyPrice != "41.00" || out[1].BuySource != "" || out[1].BuyPrice != "" {
		t.Errorf("got %+v", out)
	}
	for _, s := range res.seen {
		if len(s.Zones) != 0 {
			t.Errorf("zones given with a nil reader: %v", s.Zones)
		}
	}
	// A nil resolver leaves the entries as they are, marking hand where a price exists.
	out, err = buyprice.Apply(context.Background(), &fakeReader{st: state()}, nil, in)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].BuySource != buypriceapi.SourceHand || out[0].BuyPrice != "41.00" || out[1].BuySource != "" || out[1].BuyPrice != "" {
		t.Errorf("nil resolver: got %+v", out)
	}
}

func TestApplyOneRead(t *testing.T) {
	t.Parallel()
	rd := &fakeReader{st: state()}
	in := []jobs.WatchEntry{entry("AAA", "AAA", "USD", ""), entry("BBB", "BBB", "JPY", ""), entry("CCC", "CCC", "USD", "5")}
	if _, err := buyprice.Apply(context.Background(), rd, &fakeResolver{}, in); err != nil {
		t.Fatal(err)
	}
	if rd.reads != 1 {
		t.Errorf("reads = %d, want 1", rd.reads)
	}
}

func TestApplyDoesNotChangeInput(t *testing.T) {
	t.Parallel()
	in := []jobs.WatchEntry{entry("AAA", "AAA", "USD", "90.00"), entry("DDD", "DDD", "USD", "")}
	before := append([]jobs.WatchEntry(nil), in...)
	out, err := buyprice.Apply(context.Background(), &fakeReader{st: state()}, &fakeResolver{}, in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, before) {
		t.Errorf("input changed: %+v", in)
	}
	for i := range out {
		want := in[i]
		want.BuyPrice, want.BuySource = out[i].BuyPrice, out[i].BuySource
		if !reflect.DeepEqual(out[i], want) {
			t.Errorf("entry %d: another field changed: %+v", i, out[i])
		}
	}
}

func TestApplyKeyIsContractSymbol(t *testing.T) {
	t.Parallel()
	// The issuer is SEC key text: the contract symbol decides the match.
	out, err := buyprice.Apply(context.Background(), &fakeReader{st: state()}, &fakeResolver{},
		[]jobs.WatchEntry{entry("cik:0000320193", "AAA", "USD", ""), entry("AAA", "ZZZ", "USD", "")})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].BuySource != buypriceapi.SourceValuation || out[0].BuyPrice != "91.123456" {
		t.Errorf("symbol match: got %q %q", out[0].BuySource, out[0].BuyPrice)
	}
	if out[1].BuySource != "" {
		t.Errorf("issuer must not match: got %q", out[1].BuySource)
	}
	res := &fakeResolver{}
	if _, err := buyprice.Apply(context.Background(), &fakeReader{st: state()}, res, []jobs.WatchEntry{entry("AAA.X", "AAA", "JPY", "")}); err != nil {
		t.Fatal(err)
	}
	if res.seen[0].Symbol != "AAA" || res.seen[0].Currency != "JPY" {
		t.Errorf("input = %+v", res.seen[0])
	}
}

func TestApplyEmptySymbolKeepsHand(t *testing.T) {
	t.Parallel()
	out, err := buyprice.Apply(context.Background(), &fakeReader{st: state()}, &fakeResolver{},
		[]jobs.WatchEntry{entry("AAA", "", "USD", "41.00")})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].BuyPrice != "41.00" || out[0].BuySource != buypriceapi.SourceHand {
		t.Errorf("got %q %q", out[0].BuyPrice, out[0].BuySource)
	}
}

func TestApplyCopiesState(t *testing.T) {
	t.Parallel()
	res := &fakeResolver{}
	if _, err := buyprice.Apply(context.Background(), &fakeReader{st: state()}, res, []jobs.WatchEntry{entry("AAA", "AAA", "USD", "")}); err != nil {
		t.Fatal(err)
	}
	in := res.seen[0]
	wantZones := []buypriceapi.Zone{
		{Symbol: "AAA", Currency: "USD", Pool: "in_pool", Market: "US", V: "100", P: "3", ReviewDay: "2026-11-17", Reviewed: true},
		{Symbol: "BBB", Currency: "JPY", Pool: "watch", Market: "JP"},
	}
	if !reflect.DeepEqual(in.Zones, wantZones) {
		t.Errorf("zones = %+v", in.Zones)
	}
	wantTables := buypriceapi.Tables{
		Present: true,
		R:       valuelinesapi.RValues{US: "0.08", JP: "0.07", HKCN: "0.09"},
		Margin:  []valuelinesapi.MarginRow{{PFrom: "1", M: "0.1", W: "0.2", H: "0.3", S: "0.4"}},
	}
	if !reflect.DeepEqual(in.Tables, wantTables) {
		t.Errorf("tables = %+v", in.Tables)
	}
}
