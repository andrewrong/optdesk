package optioneod

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestStatusMapping(t *testing.T) {
	t.Parallel()
	v := 1.0
	cases := []struct {
		name   string
		q      broker.Quote
		status int
		code   string
	}{
		{"ok", broker.Quote{Status: broker.StatusOK, Last: &v}, 1, ""},
		{"partial", broker.Quote{Status: broker.StatusPartial, Last: &v}, 1, ""},
		{"no data", broker.Quote{Status: broker.StatusNoData}, 2, ""},
		{"no subscription", broker.Quote{Status: broker.StatusNoSubscription}, 5, "NO_SUBSCRIPTION"},
		{"timeout", broker.Quote{Status: broker.StatusTimeout}, 5, "TIMEOUT"},
		{"error", broker.Quote{Status: broker.StatusError}, 5, "ERROR"},
		{"disconnected", broker.Quote{Status: broker.StatusDisconnected}, 5, "DISCONNECTED"},
		{"ibkr code", broker.Quote{Status: broker.StatusError, ErrorCode: 354}, 5, "354"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, code := statusOf(tc.q)
			if s != tc.status || (tc.code == "") != (code == nil) || (code != nil && *code != tc.code) {
				t.Fatalf("got %d %v", s, code)
			}
		})
	}
}

func TestEmptyQuoteStatus(t *testing.T) {
	t.Parallel()
	f := func(v float64) *float64 { return &v }
	day := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	for _, st := range []broker.Status{broker.StatusOK, broker.StatusPartial} {
		t.Run(fmt.Sprintf("%d empty", st), func(t *testing.T) {
			t.Parallel()
			q := broker.Quote{Status: st, Volume: f(4), OpenInterest: f(5), BidSize: f(6)}
			r := mapRow(q, 1, 2, day, uuid.New(), "USD")
			if r.Status != 2 || r.ErrorCode != nil {
				t.Fatalf("status %d code %v, want 2 nil", r.Status, r.ErrorCode)
			}
			if r.Volume == nil || *r.Volume != 4 || r.OpenInterest == nil || *r.OpenInterest != 5 || r.BidSize == nil || *r.BidSize != 6 {
				t.Fatalf("volume, open interest and size must still be written: %v %v %v", r.Volume, r.OpenInterest, r.BidSize)
			}
		})
		t.Run(fmt.Sprintf("%d last only", st), func(t *testing.T) {
			t.Parallel()
			if s, code := statusOf(broker.Quote{Status: st, Last: f(3)}); s != 1 || code != nil {
				t.Fatalf("got %d %v, want 1 nil", s, code)
			}
		})
	}
}

func TestMapRow(t *testing.T) {
	t.Parallel()
	f := func(v float64) *float64 { return &v }
	ev := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	q := broker.Quote{Status: broker.StatusOK, MarketDataType: 2, Generation: 7, ReceivedAt: ev.Add(time.Minute), EventTs: &ev,
		Bid: f(1), Ask: f(2), Last: f(3), Close: f(9), Volume: f(4), OpenInterest: f(5), BidSize: f(6), AskSize: f(7), LastSize: f(8),
		ImpliedVol: f(0.2), Delta: f(0.5), Gamma: f(0.1), Vega: f(0.3), Theta: f(-0.4), UndPrice: f(100)}
	cid := uuid.New()
	r := mapRow(q, 11, 22, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC), cid, "USD")
	got := []any{r.InstrumentID, r.UnderlyingID, r.Generation, r.MarketDataType, r.Status, r.ErrorCode == nil, r.SnapshotKind, r.Currency,
		r.UnderlyingCcy, r.CaptureID, r.SessionDay.Format(time.DateOnly), r.EventTs.Equal(ev)}
	want := []any{11, 22, 7, 2, 1, true, "eod", "USD", "USD", cid.String(), "2026-11-06", true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("row %v, want %v", got, want)
	}
	gotN := []float64{*r.Bid, *r.Ask, *r.Last, *r.Volume, *r.OpenInterest, *r.BidSize, *r.AskSize, *r.IvDecimal, *r.Delta, *r.Gamma,
		*r.VegaPerVolPoint, *r.ThetaPerCalendarDay, *r.UnderlyingPrice}
	wantN := []float64{1, 2, 3, 4, 5, 6, 7, 0.2, 0.5, 0.1, 0.3, -0.4, 100}
	if !reflect.DeepEqual(gotN, wantN) {
		t.Fatalf("numbers %v, want %v", gotN, wantN)
	}
	nils := []any{r.IvBidDecimal, r.IvMidDecimal, r.IvAskDecimal, r.TermsVersionID, r.OiAsofDate, r.UnderlyingEventTs}
	for i, v := range nils {
		if !reflect.ValueOf(v).IsNil() {
			t.Fatalf("unfilled column %d must stay nil", i)
		}
	}
	if got := mapRow(broker.Quote{}, 1, 1, time.Time{}, cid, "USD"); got.EventTs != nil {
		t.Fatal("nil event_ts stays nil")
	}
}

func TestOsiSymbol(t *testing.T) {
	t.Parallel()
	got, err := osiSymbol(broker.Contract{Symbol: "TEST", Expiry: "20261106", Right: "C", Strike: "95"})
	if err != nil || got != "TEST  261106C00095000" || len(got) != 21 {
		t.Fatalf("got %q %v", got, err)
	}
	got, _ = osiSymbol(broker.Contract{Symbol: "SPY", Expiry: "20271217", Right: "P", Strike: "1234.5"})
	if got != "SPY   271217P01234500" {
		t.Fatalf("got %q", got)
	}
	if _, err := osiSymbol(broker.Contract{Symbol: "TEST", Expiry: "bad", Right: "C", Strike: "x"}); err == nil {
		t.Fatal("want error")
	}
}

func TestOSISymbolShareClass(t *testing.T) {
	t.Parallel()
	c := broker.Contract{Symbol: "BRK B", Expiry: "20261120", Right: "C", Strike: "500"}
	got, err := osiSymbolWith(c, "BRKB")
	if err != nil || got != "BRKB  261120C00500000" || len(got) != 21 {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := osiSymbolWith(c, "TOOLONG"); err == nil {
		t.Fatal("a root over 6 is refused")
	}
}

func TestChainOfShareClass(t *testing.T) {
	t.Parallel()
	chains := []broker.Chain{
		{Exchange: "SMART", TradingClass: "BRK B"},
		{Exchange: "SMART", TradingClass: "BRKB"},
	}
	u := underlying{Symbol: "BRK B", Exchange: "SMART"}
	got, class, ok := manifest{}.chainOfRoot(chains, u, "BRKB")
	if !ok || class != "BRKB" || got.TradingClass != "BRKB" {
		t.Fatalf("got %+v %q %v", got, class, ok)
	}
	got, class, ok = manifest{TradingClass: "BRK B"}.chainOfRoot(chains, u, "BRKB")
	if !ok || class != "BRK B" || got.TradingClass != "BRK B" {
		t.Fatalf("manifest class wins: got %+v %q %v", got, class, ok)
	}
}

func TestCaptureIDValue(t *testing.T) {
	t.Parallel()
	k := contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "TEST"}
	want := uuid.NewSHA1(uuid.NameSpaceOID, []byte("2026-11-06|obs.option_eod|TEST"))
	if captureID("2026-11-06", k) != want {
		t.Fatal("capture id is the SHA1 uuid of day|dataset|subject")
	}
}
