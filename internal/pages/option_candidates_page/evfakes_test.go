package option_candidates_page

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeEval records the rows it is asked about and answers with fn.
type fakeEval struct {
	mu   sync.Mutex
	rows []optionevidenceapi.Row
	fn   func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error)
}

func (f *fakeEval) Evaluate(r optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
	f.mu.Lock()
	f.rows = append(f.rows, r)
	f.mu.Unlock()
	if f.fn == nil {
		return fullEvidence(), nil
	}
	return f.fn(r)
}

func (f *fakeEval) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

func fullEvidence() optionevidenceapi.Evidence {
	return optionevidenceapi.Evidence{
		BreakEven: "89.06", BelowBuy: "0.94", BelowBuyPct: "1.0444",
		OneSigma: "8.5775", OneSigmaPct: "8.8886", BreakEvenInSigmas: "0.8674",
		Fair: "0.73", EdgePerShare: "0.21", EdgePerContract: "21.29",
		Skew: "0.011", SkewPct: "1.1000", ATMIVPct: "29.4000",
		ForecastErr: "0.041", ForecastPct: "27.5000", ForecastErrPct: "4.1000",
		IVPPct: "58.0000", VolEdgePct: "3.7000", Labels: []string{"ex_div_inside_life"},
	}
}

// atmCalls records the calls of an ATMFunc and answers with iv and strike.
type atmCalls struct {
	mu         sync.Mutex
	iv, strike string
	err        error
	got        []struct {
		id          int
		day, expiry contracts.Day
		spot        string
	}
}

func (a *atmCalls) fn(_ context.Context, id int, day, expiry contracts.Day, spot string) (string, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.got = append(a.got, struct {
		id          int
		day, expiry contracts.Day
		spot        string
	}{id, day, expiry, spot})
	return a.iv, a.strike, a.err
}

// fakeView holds readings by figure id and records the reads.
type fakeView struct {
	mu       sync.Mutex
	recs     map[string][]contracts.ReadingRecord
	subjects []string
	err      error
}

func (f *fakeView) WinnerReadings(_ context.Context, name, _, subject string, _, _ contracts.Day) ([]contracts.ReadingRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subjects = append(f.subjects, subject)
	return f.recs[name], f.err
}

func (f *fakeView) reads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.subjects)
}

func recAt(day, state string, value *string, code *int) contracts.ReadingRecord {
	var r contracts.ReadingRecord
	t, _ := time.Parse(time.DateOnly, day)
	r.Day.Time = t
	r.State = gen.ReadingRecordState(state)
	r.Value, r.ReasonCode = value, code
	return r
}

func okRec(day, value string) contracts.ReadingRecord { return recAt(day, "ok", &value, nil) }

// allReadings is an ok reading of the day for the four figures.
func allReadings(day string) *fakeView {
	return &fakeView{recs: map[string][]contracts.ReadingRecord{
		idVolForecast:    {okRec(day, "0.275")},
		idVolForecastErr: {okRec(day, "0.041")},
		idIVP:            {okRec(day, "0.58")},
		idVolEdge:        {okRec(day, "0.037")},
	}}
}

// okPut is an ok short-put row with a spot and a buy price.
func okPut(id int) store.Candidate {
	c := cand("short_put", id, "ok")
	c.Spot, c.BuyPrice, c.UnderlyingID, c.Multiplier = strp("96.50"), strp("90.00"), 7, 100
	c.Strike, c.BidEod = "90.00", "0.94"
	return c
}

func evConfig(rows []store.Candidate, e optionevidenceapi.Evaluator) Config {
	c := cfg(fakeReader{rows: map[string][]store.Candidate{"2026-11-09/short_put": rows}})
	c.Evidence = e
	return c
}

func buildOut(t *testing.T, c Config, series store.SeriesReader) outPage {
	t.Helper()
	b, err := Build(context.Background(), c, series, buildDay, time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p outPage
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// firstRow is the first row of the main table of the short-put group.
func firstRow(p outPage) outRow { return p.Groups[0].Main.Rows[0] }

var evidenceIDs = []string{"below_buy", "one_sigma", "bid_vs_forecast", "skew", "forecast_vol", "ivp", "vol_edge", "labels"}

var testNow = time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC)

func dayp(s string) *contracts.Day { d := contracts.Day(s); return &d }

func (f *fakeView) Close() error { return nil }
