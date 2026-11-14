package main

import (
	"context"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

func replayDay(d string) types.SerializableDate {
	t, err := time.Parse(time.DateOnly, d)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: t}
}

func fptr(v float64) *float64 { return &v }

// replayEod is one option_eod row of instrument 101 on day.
func replayEod(day string, status int, received time.Time, capture string, bid, ask *float64) contracts.OptionEod {
	return contracts.OptionEod{
		InstrumentID: 101, UnderlyingID: 7, SessionDay: replayDay(day), Status: status,
		ReceivedAt: received, CaptureID: capture, Bid: bid, Ask: ask,
	}
}

type replayCatalog struct{ ids []int64 }

func (c replayCatalog) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	var out []store.OptionEodLatest
	for _, id := range c.ids {
		out = append(out, store.OptionEodLatest{UnderlyingID: id, SessionDay: "2026-11-20"})
	}
	return out, nil
}

func (replayCatalog) Close() error { return nil }

type replayEodStore struct {
	store.OptionEodStore
	rows map[int64][]contracts.OptionEod
}

func (s replayEodStore) OptionEod(_ context.Context, id int64, _, _ contracts.Day) ([]contracts.OptionEod, error) {
	return s.rows[id], nil
}

func (replayEodStore) Close() error { return nil }

type replayTerms struct{ terms []store.OptionTerms }

func (r replayTerms) OptionTerms(context.Context, []int64) ([]store.OptionTerms, error) {
	return r.terms, nil
}

func (replayTerms) Close() error { return nil }

type replayNames struct{}

func (replayNames) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, id := range ids {
		if id == 7 {
			out = append(out, store.InstrumentName{InstrumentID: 7, Type: "stock", Name: "XYZ"})
		}
	}
	return out, nil
}

func (replayNames) Close() error { return nil }

// replayBars holds closes by basis and day for instrument 7.
type replayBars struct {
	bar.BarStore
	closes map[string]map[string]string
}

func (b replayBars) PriceBars(_ context.Context, id int64, basis string, _, _ contracts.Day) ([]contracts.PriceBar, error) {
	if id != 7 {
		return nil, nil
	}
	var out []contracts.PriceBar
	for d, c := range b.closes[basis] {
		out = append(out, contracts.PriceBar{InstrumentID: 7, SessionDay: replayDay(d), Close: c})
	}
	return out, nil
}

func (replayBars) Close() error { return nil }

func replayReaders(rows []contracts.OptionEod) markReaders {
	return markReaders{
		catalog: replayCatalog{ids: []int64{7}},
		eod:     replayEodStore{rows: map[int64][]contracts.OptionEod{7: rows}},
		terms: replayTerms{terms: []store.OptionTerms{
			{InstrumentID: 101, Right: "P", Strike: "100.00", ExpirationDate: "2026-11-20"},
		}},
		names: replayNames{},
	}
}

func marketLot() replayapi.Lot {
	return replayapi.Lot{ID: "L1", Underlying: "XYZ", Right: "P", Strike: "100", Expiry: "2026-11-20",
		EntryDay: "2026-11-09", SettleDay: "2026-11-20", Credit: "2"}
}

func TestMarketPicksLatestReceivedAtThenCaptureID(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 9, 14, 30, 0, 0, time.UTC)
	rows := []contracts.OptionEod{
		// Two rows on one day: the later received_at wins whatever the capture id.
		replayEod("2026-11-10", eodOK, t0, "zzz", fptr(0.5), fptr(0.60)),
		replayEod("2026-11-10", eodOK, t0.Add(time.Minute), "aaa", fptr(0.3), fptr(0.40)),
		// Same received_at: the larger capture id wins.
		replayEod("2026-11-11", eodOK, t0, "a", fptr(0.5), fptr(0.70)),
		replayEod("2026-11-11", eodOK, t0, "b", fptr(0.5), fptr(0.75)),
		// A newer row that is not ok never wins.
		replayEod("2026-11-12", eodOK, t0, "a", fptr(0.1), fptr(0.20)),
		replayEod("2026-11-12", 2, t0.Add(time.Hour), "b", nil, nil),
	}
	mkt, lots, err := resolveMarket(context.Background(), replayReaders(rows), replayBars{}, []replayapi.Lot{marketLot()})
	if err != nil {
		t.Fatal(err)
	}
	if lots[0].InstrumentID != "101" {
		t.Fatalf("instrument %q", lots[0].InstrumentID)
	}
	q := mkt.Quotes["101"]
	for day, ask := range map[contracts.Day]string{"2026-11-10": "0.4", "2026-11-11": "0.75", "2026-11-12": "0.2"} {
		if q[day].Ask != ask {
			t.Errorf("%s: ask %q, want %s", day, q[day].Ask, ask)
		}
	}
	if q["2026-11-10"].Bid != "0.3" {
		t.Errorf("bid %q", q["2026-11-10"].Bid)
	}
}

func TestMarketLeavesAMissingDayAbsent(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 9, 14, 30, 0, 0, time.UTC)
	rows := []contracts.OptionEod{
		replayEod("2026-11-10", eodOK, t0, "a", fptr(0.3), fptr(0.40)),
		replayEod("2026-11-12", 4, t0, "a", fptr(0.3), fptr(0.40)), // stale: not ok
		replayEod("2026-11-13", eodOK, t0, "a", fptr(0.3), nil),    // no ask
	}
	bars := replayBars{closes: map[string]map[string]string{
		"trades":        {"2026-11-09": "101", "2026-11-20": "99", "2026-11-15": "50"},
		"adjusted_last": {"2026-11-09": "100.5"}, // no settle-day adjusted close
	}}
	mkt, _, err := resolveMarket(context.Background(), replayReaders(rows), bars, []replayapi.Lot{marketLot()})
	if err != nil {
		t.Fatal(err)
	}
	if len(mkt.Quotes["101"]) != 1 {
		t.Errorf("quotes %v, want only 2026-11-10", mkt.Quotes["101"])
	}
	if _, has := mkt.Quotes["101"]["2026-11-11"]; has {
		t.Error("a day without a row got a quote")
	}
	if got := mkt.Closes["XYZ"]; len(got) != 2 || got["2026-11-09"] != "101" || got["2026-11-20"] != "99" {
		t.Errorf("closes %v: entry and settle day only", got)
	}
	if got := mkt.AdjustedCloses["XYZ"]; len(got) != 1 || got["2026-11-09"] != "100.5" {
		t.Errorf("adjusted closes %v", got)
	}
}

func TestMarketUnmatchedLotHasNoInstrument(t *testing.T) {
	t.Parallel()
	l := marketLot()
	l.Strike = "105"
	mkt, lots, err := resolveMarket(context.Background(), replayReaders(nil), replayBars{}, []replayapi.Lot{l})
	if err != nil {
		t.Fatal(err)
	}
	if lots[0].InstrumentID != "" || len(mkt.Quotes) != 0 {
		t.Fatalf("instrument %q quotes %v", lots[0].InstrumentID, mkt.Quotes)
	}
}
