package trackapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fakeTracker keeps the promises of the interface: it returns one fixed result and
// remembers what it was given.
type fakeTracker struct {
	got    trackapi.TrackInput
	params trackapi.TrackParams
	result trackapi.TrackResult
}

var _ trackapi.OptionTracker = (*fakeTracker)(nil)

func (f *fakeTracker) Track(in trackapi.TrackInput, p trackapi.TrackParams) trackapi.TrackResult {
	f.got, f.params = in, p
	return f.result
}

func fixedResult() trackapi.TrackResult {
	row := trackapi.TrackedOption{
		PositionID: "p1", Underlying: "AAPL", Right: "put", Strike: "180", Expiry: "2026-12-18",
		Quantity: "-1", Multiplier: "100", Currency: "USD", Status: "open", Early: true, DTE: 39,
		Moneyness: "otm", OtmPercent: "5.50", Spot: "190.5", Mid: "1.25", MarkDay: "2026-11-09",
		MarkStatus: "ok", Extrinsic: "1.25", PremiumIncome: "150", Commissions: "1.3", Kept: "25",
		KeptPercent: "16.67", RealizedPnL: "148.7", Delta: "-0.2", Iv: "0.31",
		NextEarnings: "2026-12-01", NextExDate: "2026-11-20", ExAmount: "0.25",
		EarningsInLife: true, ExInLife: true, EventStatus: "ok", Guard: "unknown",
		GuardReason: "no_guard_row", GuardBasis: "bid", OpenedOn: "2026-10-01", ClosedOn: "2026-11-01",
	}
	return trackapi.TrackResult{
		Open:   []trackapi.TrackedOption{row},
		Recent: []trackapi.TrackedOption{row},
		Recon: []trackapi.TrackRecon{{
			Underlying: "AAPL", Right: "P", Strike: "180", Quantity: "-1", Expiry: "2026-12-18", Code: "not_in_flex",
		}},
		LongLegsLeftOut: 2,
	}
}

func fullInput() trackapi.TrackInput {
	return trackapi.TrackInput{
		Alias:       "main",
		AsOf:        "2026-11-09",
		SessionDays: []contracts.Day{"2026-11-06", "2026-11-09"},
		Positions:   []compute.LedgerPosition{{Underlying: "AAPL", Right: "P", Strike: "180"}},
		Quotes: []trackapi.TrackQuote{{
			Underlying: "AAPL", Expiry: "2026-12-18", Strike: "180", Right: "P", Day: "2026-11-09",
			Bid: "1.2", Ask: "1.3", UnderlyingClose: "190.5", Delta: "-0.2", Iv: "0.31",
		}},
		Events: []trackapi.TrackEvent{{
			Underlying: "AAPL", Expiry: "2026-12-18", Strike: "180", Right: "C", Kind: "held_call_guard",
			Day: "2026-11-09", NextEarnings: "2026-12-01", EarningsState: "known", NextExDate: "2026-11-20",
			ExAmount: "0.25", GuardStatus: "block", GuardReason: "early_assignment_risk",
		}},
		Held:      []trackapi.TrackHeld{{Underlying: "AAPL", Expiry: "2026-12-18", Strike: "180", Right: "P", Quantity: "-1"}},
		HeldKnown: true,
	}
}

func TestOptionTrackerContract(t *testing.T) {
	t.Parallel()
	fake := &fakeTracker{result: fixedResult()}
	var newTracker trackapi.NewOptionTrackerFunc = func() trackapi.OptionTracker { return fake }

	in := fullInput()
	params := trackapi.TrackParams{MarkMaxAgeSessions: 3, RecentDays: 30}
	res := newTracker().Track(in, params)

	// The tracker side: it received every field the caller filled.
	assertReceived(t, fake, in, params)

	// The caller side: it reads the result through the interface only.
	if len(res.Open) != 1 || len(res.Recent) != 1 || len(res.Recon) != 1 || res.LongLegsLeftOut != 2 {
		t.Fatalf("unexpected result shape: %+v", res)
	}
	if res.Open[0] != fixedResult().Open[0] || res.Recent[0] != fixedResult().Recent[0] {
		t.Fatalf("tracked option changed: %+v", res.Open[0])
	}
	if res.Recon[0] != fixedResult().Recon[0] {
		t.Fatalf("recon changed: %+v", res.Recon[0])
	}
	assertOption(t, res.Open[0])
}

// assertOption reads every field of a TrackedOption against the literal of fixedResult.
func assertOption(t *testing.T, o trackapi.TrackedOption) {
	t.Helper()
	text := map[string]string{
		"PositionID": o.PositionID, "Underlying": o.Underlying, "Right": o.Right, "Strike": o.Strike,
		"Expiry": o.Expiry, "Quantity": o.Quantity, "Multiplier": o.Multiplier, "Currency": o.Currency,
		"Status": o.Status, "Moneyness": o.Moneyness, "OtmPercent": o.OtmPercent, "Spot": o.Spot, "Mid": o.Mid,
		"MarkDay": string(o.MarkDay), "MarkStatus": o.MarkStatus, "Extrinsic": o.Extrinsic,
		"PremiumIncome": o.PremiumIncome, "Commissions": o.Commissions, "Kept": o.Kept,
		"KeptPercent": o.KeptPercent, "RealizedPnL": o.RealizedPnL, "Delta": o.Delta, "Iv": o.Iv,
		"NextEarnings": string(o.NextEarnings), "NextExDate": string(o.NextExDate), "ExAmount": o.ExAmount,
		"EventStatus": o.EventStatus, "Guard": o.Guard, "GuardReason": o.GuardReason,
		"GuardBasis": o.GuardBasis, "OpenedOn": string(o.OpenedOn), "ClosedOn": string(o.ClosedOn),
	}
	want := map[string]string{
		"PositionID": "p1", "Underlying": "AAPL", "Right": "put", "Strike": "180", "Expiry": "2026-12-18",
		"Quantity": "-1", "Multiplier": "100", "Currency": "USD", "Status": "open", "Moneyness": "otm",
		"OtmPercent": "5.50", "Spot": "190.5", "Mid": "1.25", "MarkDay": "2026-11-09", "MarkStatus": "ok",
		"Extrinsic": "1.25", "PremiumIncome": "150", "Commissions": "1.3", "Kept": "25", "KeptPercent": "16.67",
		"RealizedPnL": "148.7", "Delta": "-0.2", "Iv": "0.31", "NextEarnings": "2026-12-01",
		"NextExDate": "2026-11-20", "ExAmount": "0.25", "EventStatus": "ok", "Guard": "unknown",
		"GuardReason": "no_guard_row", "GuardBasis": "bid", "OpenedOn": "2026-10-01", "ClosedOn": "2026-11-01",
	}
	for k, v := range want {
		if text[k] != v {
			t.Errorf("%s = %q, want %q", k, text[k], v)
		}
	}
	if !o.Early || o.DTE != 39 || !o.EarningsInLife || !o.ExInLife {
		t.Errorf("flags or DTE lost: %+v", o)
	}
}

func assertReceived(t *testing.T, fake *fakeTracker, in trackapi.TrackInput, params trackapi.TrackParams) {
	t.Helper()
	got := fake.got
	if fake.params != params || got.Alias != in.Alias || got.AsOf != in.AsOf || !got.HeldKnown {
		t.Fatalf("the tracker did not receive the input: %+v", got)
	}
	if len(got.SessionDays) != 2 || len(got.Positions) != 1 {
		t.Fatalf("session days or positions lost: %+v", got)
	}
	if p := got.Positions[0]; p.Strike != in.Positions[0].Strike || p.Right != in.Positions[0].Right {
		t.Fatalf("a position field was lost: %+v", p)
	}
	if len(got.Quotes) != 1 || got.Quotes[0] != in.Quotes[0] {
		t.Fatalf("a quote field was lost: %+v", got.Quotes)
	}
	if len(got.Events) != 1 || got.Events[0] != in.Events[0] {
		t.Fatalf("an event field was lost: %+v", got.Events)
	}
	if len(got.Held) != 1 || got.Held[0] != in.Held[0] {
		t.Fatalf("a held field was lost: %+v", got.Held)
	}
}
