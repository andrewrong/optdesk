package underlyingresolve

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/stockreg"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

type fakeRegistrar struct {
	got []stockreg.Registration
	err error
}

func (f *fakeRegistrar) RegisterStock(_ context.Context, r stockreg.Registration) (int64, error) {
	f.got = append(f.got, r)
	return 1, f.err
}

type permanent struct{}

func (permanent) Error() string             { return "two issuers" }
func (permanent) Kind() contracts.ErrorKind { return contracts.Permanent }

func regRig(t *testing.T, reg stockreg.StockRegistrar, rows ...underlying.Row) *rig {
	t.Helper()
	r := &rig{
		b: &fakeBroker{answers: map[string]broker.Resolved{}, errs: map[string]error{}},
		s: &fakeSettler{rows: rows, written: true},
	}
	j, err := NewWithRegistrar(r.b, r.s, reg, clockwork.NewFakeClockAt(start))
	if err != nil {
		t.Fatal(err)
	}
	r.j = j
	return r
}

func TestRegistrationOfIssuer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text string
		want *stockreg.Issuer
	}{
		{"cik:0000320193", &stockreg.Issuer{CIK: "0000320193", Country: "US"}},
		{"edinet:E02144", &stockreg.Issuer{EdinetCode: "E02144", Country: "JP"}},
		{"", nil},
		{"cik:", nil},
		{"cik:123", nil},
		{"cik:00003201ab", nil},
		{"edinet:", nil},
		{"isin:US0378331005", nil},
	}
	for _, c := range cases {
		got := RegistrationOf(underlying.Row{Symbol: "AAPL", Market: "US", Issuer: c.text}).Issuer
		switch {
		case c.want == nil && got != nil, c.want != nil && (got == nil || *got != *c.want):
			t.Errorf("issuer %q: got %+v, want %+v", c.text, got, c.want)
		}
	}
}

func TestRegistrationOfTokyoName(t *testing.T) {
	t.Parallel()
	got := RegistrationOf(underlying.Row{
		ID: 3, Key: "JP:7203", Symbol: "7203", Market: "JP", Kind: underlying.KindStock, Exchange: "TSEJ",
		Currency: "JPY", Name: "Toyota", ConID: 14742, Issuer: "edinet:E02144",
	})
	want := stockreg.Registration{
		ConID: 14742, Kind: "stock", Symbol: "7203", Market: "JP", PrimaryExchange: "TSEJ", Name: "Toyota",
		Currency: "JPY", CalendarID: "XNYS", Issuer: &stockreg.Issuer{Name: "Toyota", EdinetCode: "E02144", Country: "JP"},
	}
	if got.Issuer == nil || *got.Issuer != *want.Issuer {
		t.Fatalf("issuer %+v", got.Issuer)
	}
	got.Issuer, want.Issuer = nil, nil
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRegistrationOfSmartExchangeIsNoPrimary(t *testing.T) {
	t.Parallel()
	got := RegistrationOf(underlying.Row{Symbol: "AAPL", Market: "US", Exchange: "SMART", ConID: 1})
	if got.PrimaryExchange != "" {
		t.Fatalf("primary exchange %q", got.PrimaryExchange)
	}
}

func TestSettledConIDIsRegistered(t *testing.T) {
	t.Parallel()
	reg := &fakeRegistrar{}
	r := regRig(t, reg, withIssuer(row(7, "AAPL", underlying.StatusPending, time.Hour), "cik:0000320193"))
	r.b.answers["AAPL"] = listed(265598)
	res := r.run(t)
	if res.Err != nil || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	if len(reg.got) != 1 || reg.got[0].ConID != 265598 || reg.got[0].Symbol != "AAPL" || reg.got[0].Market != "US" ||
		reg.got[0].Issuer == nil || reg.got[0].Issuer.CIK != "0000320193" {
		t.Fatalf("registered %+v", reg.got)
	}
}

func withIssuer(r underlying.Row, text string) underlying.Row {
	r.Issuer = text
	return r
}

func TestNoRegistrationWithoutAConID(t *testing.T) {
	t.Parallel()
	cases := map[string]broker.Resolved{
		"not listed": {ErrorText: "none"},
		"ambiguous":  {Listed: true},
	}
	for name, ans := range cases {
		reg := &fakeRegistrar{}
		r := regRig(t, reg, row(1, "ZZZZ", underlying.StatusPending, time.Hour))
		r.b.answers["ZZZZ"] = ans
		if res := r.run(t); res.Err != nil {
			t.Fatalf("%s: %+v", name, res)
		}
		if len(reg.got) != 0 {
			t.Fatalf("%s: registered %+v", name, reg.got)
		}
	}
}

func TestNoRegistrationWhenSettleWroteNothing(t *testing.T) {
	t.Parallel()
	reg := &fakeRegistrar{}
	r := regRig(t, reg, row(1, "AAPL", underlying.StatusPending, time.Hour))
	r.s.written = false
	r.b.answers["AAPL"] = listed(5)
	r.run(t)
	if len(reg.got) != 0 {
		t.Fatalf("registered %+v", reg.got)
	}
}

func TestRegistrarErrorIsRetryableAndKeepsSettle(t *testing.T) {
	t.Parallel()
	reg := &fakeRegistrar{err: errors.New("db down")}
	r := regRig(t, reg, row(1, "AAPL", underlying.StatusPending, time.Hour))
	r.b.answers["AAPL"] = listed(5)
	res := r.run(t)
	var te contracts.TypedError
	if !errors.As(res.Err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("err %v", res.Err)
	}
	if len(r.s.settled) != 1 || r.s.settled[0].ConID != 5 || res.Usable != 1 {
		t.Fatalf("settled %+v result %+v", r.s.settled, res)
	}
}

func TestRegistrarPermanentRefusalLeavesResult(t *testing.T) {
	t.Parallel()
	reg := &fakeRegistrar{err: permanent{}}
	r := regRig(t, reg, row(1, "AAPL", underlying.StatusPending, time.Hour))
	r.b.answers["AAPL"] = listed(5)
	res := r.run(t)
	if res.Err != nil || res.Usable != 1 || res.Returned != 1 || len(reg.got) != 1 {
		t.Fatalf("result %+v calls %d", res, len(reg.got))
	}
}

func TestNewCallsNoRegistrar(t *testing.T) {
	t.Parallel()
	r := newRig(t, row(1, "AAPL", underlying.StatusPending, time.Hour))
	r.b.answers["AAPL"] = listed(5)
	if res := r.run(t); res.Err != nil || res.Usable != 1 {
		t.Fatalf("%+v", res)
	}
}

func TestNewWithRegistrarRequiresRegistrar(t *testing.T) {
	t.Parallel()
	if _, err := NewWithRegistrar(&fakeBroker{}, &fakeSettler{}, nil, clockwork.NewFakeClock()); err == nil {
		t.Fatal("nil registrar accepted")
	}
}
