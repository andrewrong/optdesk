package main

import (
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// demoOptionIDs are the two underlyings of the demo catalog.
var demoOptionIDs = []int64{9000, 9010}

// demoOption holds the option readers of the demo profile: two underlyings, three contracts each,
// in process. It opens no store and keeps no state.
type demoOption struct{}

// demoOptionInstruments returns the three contract ids of an underlying: its id times ten, plus 1 to 3.
func demoOptionInstruments(underlying int64) []int64 {
	return []int64{underlying*10 + 1, underlying*10 + 2, underlying*10 + 3}
}

func demoDay(d contracts.Day) types.SerializableDate {
	t, _ := time.Parse(dayLayout, string(d))
	return types.SerializableDate{Time: t}
}

// OptionEodLatest lists both demo underlyings with the day 7 days after since, the build day the
// site derives since from; an empty since gives the day of the demo points.
func (demoOption) OptionEodLatest(_ context.Context, since contracts.Day) ([]store.OptionEodLatest, error) {
	day := contracts.Day("2026-11-04")
	if t, err := time.Parse(dayLayout, string(since)); err == nil {
		day = contracts.Day(t.AddDate(0, 0, 7).Format(dayLayout))
	}
	out := make([]store.OptionEodLatest, 0, len(demoOptionIDs))
	for _, id := range demoOptionIDs {
		out = append(out, store.OptionEodLatest{UnderlyingID: id, SessionDay: day})
	}
	return out, nil
}

// OptionEod returns three rows of the underlying with session day to; an unknown underlying has none.
func (demoOption) OptionEod(_ context.Context, underlyingID int64, _, to contracts.Day) ([]contracts.OptionEod, error) {
	known := false
	for _, id := range demoOptionIDs {
		known = known || id == underlyingID
	}
	if !known {
		return nil, nil
	}
	received := demoDay(to).Add(21 * time.Hour)
	var out []contracts.OptionEod
	for i, id := range demoOptionInstruments(underlyingID) {
		f := func(v float64) *float64 { return &v }
		out = append(out, contracts.OptionEod{
			CaptureID: "demo-" + strconv.FormatInt(id, 10), Currency: "USD", UnderlyingCcy: "USD",
			InstrumentID: int(id), UnderlyingID: int(underlyingID), UnderlyingPrice: f(100),
			Bid: f(1 + float64(i)), Ask: f(1.1 + float64(i)), Last: f(1.05 + float64(i)),
			OpenInterest: f(float64(1000 - 100*i)), Status: 1, SnapshotKind: "eod",
			ReceivedAt: received, SessionDay: demoDay(to),
		})
	}
	return out, nil
}

// QuoteSnapshots returns two slots on the day to for an instrument of the demo contracts.
func (demoOption) QuoteSnapshots(_ context.Context, instrumentID int64, _, to contracts.Day) ([]contracts.QuoteSnapshot, error) {
	if instrumentID/10 != demoOptionIDs[0] && instrumentID/10 != demoOptionIDs[1] {
		return nil, nil
	}
	f := func(v float64) *float64 { return &v }
	var out []contracts.QuoteSnapshot
	for i, slot := range []string{"open", "close"} {
		out = append(out, contracts.QuoteSnapshot{
			CaptureID: "demo-q-" + strconv.FormatInt(instrumentID, 10) + slot, Currency: "USD",
			InstrumentID: int(instrumentID), Bid: f(1 + 0.1*float64(i)), Ask: f(1.2 + 0.1*float64(i)),
			Last: f(1.1 + 0.1*float64(i)), Status: 1, Slot: slot,
			ReceivedAt: demoDay(to).Add(time.Duration(14+6*i) * time.Hour), SessionDay: demoDay(to),
		})
	}
	return out, nil
}

// OptionTerms returns the terms of the demo contracts among ids: calls and puts alternate.
func (demoOption) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	var out []store.OptionTerms
	for _, id := range ids {
		if id/10 != demoOptionIDs[0] && id/10 != demoOptionIDs[1] {
			continue
		}
		right := "C"
		if id%2 == 0 {
			right = "P"
		}
		out = append(out, store.OptionTerms{
			InstrumentID: id, TermsVersionID: id, Right: right, Strike: strconv.FormatInt(95+(id%10)*5, 10) + ".00",
			StrikeCurrency: "USD", Multiplier: "100", ExpirationDate: "2026-12-18",
			ExerciseStyle: "american", SettlementType: "physical",
		})
	}
	return out, nil
}

// demoOptionNames are the symbols of the two demo underlyings.
var demoOptionNames = map[int64]string{9000: "DEMA", 9010: "DEMB"}

// InstrumentNames returns the names of the demo underlyings among ids, ordered by id.
func (demoOption) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, id := range demoOptionIDs {
		if name, ok := demoOptionNames[id]; ok && slices.Contains(ids, id) {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: name})
		}
	}
	return out, nil
}
