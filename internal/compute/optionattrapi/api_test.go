package optionattrapi_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fakeAttr answers a made-up AAA lot of 2 contracts sold at 1.80, or ErrBadInput for a lot with no fills.
type fakeAttr struct{}

func (fakeAttr) Attribute(in optionattrapi.Input) (optionattrapi.Result, error) {
	if len(in.Lot.Fills) == 0 {
		return optionattrapi.Result{}, fmt.Errorf("lot without an open fill: %w", optionattrapi.ErrBadInput)
	}
	if len(in.Quotes) == 0 {
		return optionattrapi.Result{
			Currency: in.Lot.Currency, Steps: []optionattrapi.Step{}, GapDays: []contracts.Day{}, NoRate: []contracts.Day{},
			NoQuotes: true, Commission: "-2.60", AssignedIntrinsic: "0", LedgerPnL: in.LedgerPnL,
			Reason: "no stored quotes",
		}, nil
	}
	return optionattrapi.Result{
		Currency: in.Lot.Currency,
		Steps: []optionattrapi.Step{
			{From: "2026-11-09", To: "2026-11-11", CalendarDays: 2, MultiDay: true,
				Direction: "10.00", MoveCost: "-3.00", IVRemark: "1.00", Time: "12.00", Unsplit: "0"},
			{From: "2026-11-11", To: "2026-11-12", CalendarDays: 1, Terminal: true, Unsplit: "5.00", Reason: "no solution"},
		},
		Entry: "10.00", Exit: "0", Commission: "-2.60", Unsplit: "5.00", AssignedIntrinsic: "0",
		LedgerPnL: in.LedgerPnL, Difference: "0", UnsplitSteps: 1,
		GapDays: []contracts.Day{"2026-11-10"}, NoRate: []contracts.Day{},
	}, nil
}

func lot() optionattrapi.Lot {
	return optionattrapi.Lot{
		Currency: "USD", Strike: "38.00", Multiplier: "100", Expiry: "2026-11-20",
		Fills: []optionattrapi.Fill{{Day: "2026-11-09", Side: "open", Quantity: "2", Price: "1.80", Commission: "1.30"}},
		End:   "expired", EndDay: "2026-11-20", EndClose: "40.00",
	}
}

// TestAttributorContractFake: a caller reads the fields of a Result from a fake Attributor.
func TestAttributorContractFake(t *testing.T) {
	t.Parallel()
	var a optionattrapi.Attributor = fakeAttr{}
	in := optionattrapi.Input{
		Lot:       lot(),
		Quotes:    []optionattrapi.Quote{{Day: "2026-11-09", Bid: "1.70", Ask: "1.90", Spot: "41.00", Ref: optionattrapi.Ref{IV: "0.30"}}},
		Rates:     []optionattrapi.Rate{{Day: "2026-11-09", Percent: "4.5"}},
		LedgerPnL: "357.40", MaxRateAgeDays: 7,
	}
	r, err := a.Attribute(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Steps) != 2 || !r.Steps[0].MultiDay || !r.Steps[1].Terminal {
		t.Fatalf("steps = %+v", r.Steps)
	}
	if len(r.GapDays) != 1 || r.GapDays[0] != "2026-11-10" {
		t.Fatalf("gap days = %v", r.GapDays)
	}
	if r.Unsplit != "5.00" || r.Steps[1].Unsplit != "5.00" || r.UnsplitSteps != 1 {
		t.Fatalf("unsplit = %q, %d", r.Unsplit, r.UnsplitSteps)
	}
	if r.LedgerPnL != "357.40" || r.Difference != "0" || r.Commission != "-2.60" {
		t.Fatalf("ledger %q difference %q commission %q", r.LedgerPnL, r.Difference, r.Commission)
	}
}

// fakePricer prices from a fixed table; its error carries NoSolution.
type fakePricer struct{}

type noSolution struct{}

func (noSolution) Error() string    { return "no solution" }
func (noSolution) NoSolution() bool { return true }

func (fakePricer) Value(right, spot, strike, years, rate, vol string) (string, error) {
	if right == "P" && vol == "0.30" {
		return "1.85", nil
	}
	return "", errors.New("not in table")
}

func (fakePricer) Implied(right, spot, strike, years, rate, price string) (string, error) {
	if price == "1.85" {
		return "0.30", nil
	}
	return "", noSolution{}
}

// TestFakePricerSatisfiesPricer: the shape of stress.Pricer is kept, with NoSolution() on the error.
func TestFakePricerSatisfiesPricer(t *testing.T) {
	t.Parallel()
	var p optionattrapi.Pricer = fakePricer{}
	v, err := p.Value("P", "41", "38", "0.05", "0.045", "0.30")
	if err != nil || v != "1.85" {
		t.Fatalf("value = %q, %v", v, err)
	}
	if iv, err := p.Implied("P", "41", "38", "0.05", "0.045", "1.85"); err != nil || iv != "0.30" {
		t.Fatalf("implied = %q, %v", iv, err)
	}
	_, err = p.Implied("P", "41", "38", "0.05", "0.045", "9.99")
	var ns interface{ NoSolution() bool }
	if !errors.As(err, &ns) || !ns.NoSolution() {
		t.Fatalf("error %v has no NoSolution", err)
	}
}

// TestErrBadInputIsSentinel: the fake's error matches the sentinel.
func TestErrBadInputIsSentinel(t *testing.T) {
	t.Parallel()
	_, err := fakeAttr{}.Attribute(optionattrapi.Input{})
	if !errors.Is(err, optionattrapi.ErrBadInput) {
		t.Fatalf("err = %v", err)
	}
}

// TestEveryListIsNonNil: the empty answers of the fake are non-nil slices.
func TestEveryListIsNonNil(t *testing.T) {
	t.Parallel()
	in := optionattrapi.Input{Lot: lot(), LedgerPnL: "357.40"}
	r, err := fakeAttr{}.Attribute(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Steps == nil || r.GapDays == nil || r.NoRate == nil {
		t.Fatalf("nil list in %+v", r)
	}
	r, _ = fakeAttr{}.Attribute(optionattrapi.Input{Lot: lot(), Quotes: []optionattrapi.Quote{{}}})
	if r.GapDays == nil || r.NoRate == nil {
		t.Fatalf("nil list in %+v", r)
	}
}

// fakeBooker answers one Book per currency with made-up marked numbers.
type fakeBooker struct{}

func (fakeBooker) Combine(rs []optionattrapi.Result) []optionattrapi.Book {
	out := []optionattrapi.Book{}
	for _, r := range rs {
		out = append(out, optionattrapi.Book{
			Currency: r.Currency, Lots: 1, Time: "412", MoveCost: "-186", RentMinusCost: "226",
			Total: "100", Difference: "0", NoQuoteLots: 0,
		})
	}
	return out
}

// TestBookerContractFake: a caller reads the fields of a Book from a fake Booker.
func TestBookerContractFake(t *testing.T) {
	t.Parallel()
	var b optionattrapi.Booker = fakeBooker{}
	got := b.Combine([]optionattrapi.Result{{Currency: "USD"}, {Currency: "JPY"}})
	if len(got) != 2 || got[0].Currency != "USD" || got[1].Currency != "JPY" {
		t.Fatalf("books = %+v", got)
	}
	if got[0].RentMinusCost != "226" || got[0].Total != "100" || got[0].Difference != "0" || got[0].NoQuoteLots != 0 {
		t.Fatalf("book = %+v", got[0])
	}
}

// TestBookerEmptyIsNonNil: no results give a non-nil empty slice.
func TestBookerEmptyIsNonNil(t *testing.T) {
	t.Parallel()
	got := fakeBooker{}.Combine(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("got = %#v", got)
	}
}
