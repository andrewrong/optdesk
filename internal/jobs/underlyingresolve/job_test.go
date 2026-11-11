package underlyingresolve

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

var start = time.Date(2026, 11, 11, 21, 0, 0, 0, time.UTC)

type fakeBroker struct {
	broker.Broker
	answers map[string]broker.Resolved
	errs    map[string]error
	asked   []broker.Contract
}

func (f *fakeBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	f.asked = append(f.asked, c)
	if err := f.errs[c.Symbol]; err != nil {
		return broker.Resolved{}, err
	}
	return f.answers[c.Symbol], nil
}

// fakeSettler follows the contract of underlying.Settler: Pending returns pending rows and unresolved rows set
// before retryBefore.
type fakeSettler struct {
	underlying.Settler
	rows    []underlying.Row
	written bool
	settled []underlying.Outcome
}

func (f *fakeSettler) Pending(_ context.Context, before time.Time) ([]underlying.Row, error) {
	var out []underlying.Row
	for _, r := range f.rows {
		if r.Status == underlying.StatusPending || (r.Status == underlying.StatusUnresolved && r.SetAt.Before(before)) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeSettler) Settle(_ context.Context, o underlying.Outcome) (bool, error) {
	f.settled = append(f.settled, o)
	return f.written, nil
}

type retryable struct{}

func (retryable) Error() string             { return "unavailable" }
func (retryable) Kind() contracts.ErrorKind { return contracts.Retryable }

func row(id int64, sym, status string, age time.Duration) underlying.Row {
	return underlying.Row{
		ID: id, Key: "US:" + sym, Symbol: sym, Market: "US", Kind: underlying.KindStock, Exchange: "SMART",
		Currency: "USD", Status: status, SetAt: start.Add(-age),
	}
}

type rig struct {
	b *fakeBroker
	s *fakeSettler
	j contracts.Job
}

func newRig(t *testing.T, rows ...underlying.Row) *rig {
	t.Helper()
	r := &rig{
		b: &fakeBroker{answers: map[string]broker.Resolved{}, errs: map[string]error{}},
		s: &fakeSettler{rows: rows, written: true},
	}
	j, err := New(r.b, r.s, clockwork.NewFakeClockAt(start))
	if err != nil {
		t.Fatal(err)
	}
	r.j = j
	return r
}

func (r *rig) run(t *testing.T) contracts.JobResult {
	t.Helper()
	keys, _, err := r.j.Plan(context.Background(), "2026-11-11")
	if err != nil {
		t.Fatal(err)
	}
	return r.j.Run(context.Background(), "2026-11-11", keys)
}

func listed(id int64) broker.Resolved {
	return broker.Resolved{Listed: true, Contract: broker.Contract{ConID: id}}
}

func TestListedSettlesWithConID(t *testing.T) {
	t.Parallel()
	r := newRig(t, row(7, "AAPL", underlying.StatusPending, time.Hour))
	r.b.answers["AAPL"] = listed(265598)
	res := r.run(t)
	if res.Err != nil || res.Returned != 1 || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	want := underlying.Outcome{Key: "US:AAPL", AfterID: 7, ConID: 265598}
	if len(r.s.settled) != 1 || r.s.settled[0] != want {
		t.Fatalf("settled %+v, want %+v", r.s.settled, want)
	}
	c := r.b.asked[0]
	if c.SecType != "STK" || c.Symbol != "AAPL" || c.Exchange != "SMART" || c.Currency != "USD" {
		t.Fatalf("asked %+v", c)
	}
}

func TestNotListedSettlesUnresolvedWithReason(t *testing.T) {
	t.Parallel()
	r := newRig(t, row(1, "ZZZZ", underlying.StatusPending, time.Hour))
	r.b.answers["ZZZZ"] = broker.Resolved{ErrorCode: 200, ErrorText: "no security definition"}
	res := r.run(t)
	if res.Err != nil || len(r.s.settled) != 1 {
		t.Fatalf("result %+v settled %+v", res, r.s.settled)
	}
	o := r.s.settled[0]
	if o.ConID != 0 || !strings.Contains(o.Reason, "not listed") || !strings.Contains(o.Reason, "no security definition") {
		t.Fatalf("outcome %+v", o)
	}
}

func TestAmbiguousSettlesUnresolved(t *testing.T) {
	t.Parallel()
	r := newRig(t, row(1, "BRK", underlying.StatusPending, time.Hour))
	r.b.answers["BRK"] = listed(0) // listed with no single contract id
	r.run(t)
	if len(r.s.settled) != 1 || r.s.settled[0].ConID != 0 || !strings.Contains(r.s.settled[0].Reason, "ambiguous") {
		t.Fatalf("settled %+v", r.s.settled)
	}
}

func TestTransportErrorSettlesNothingAndIsRetryable(t *testing.T) {
	t.Parallel()
	r := newRig(t,
		row(1, "AAPL", underlying.StatusPending, time.Hour),
		row(2, "MSFT", underlying.StatusPending, time.Hour))
	r.b.errs["AAPL"] = retryable{}
	r.b.answers["MSFT"] = listed(272093)
	res := r.run(t)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("err %v", res.Err)
	}
	if res.Returned != 2 || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	if len(r.s.settled) != 1 || r.s.settled[0].Key != "US:MSFT" {
		t.Fatalf("settled %+v", r.s.settled)
	}
	// the name stays pending: the next plan asks it again
	keys, _, _ := r.j.Plan(context.Background(), "2026-11-12")
	if len(keys) != 2 {
		t.Fatalf("keys %v", keys)
	}
}

func TestUnresolvedAgeDecidesRetry(t *testing.T) {
	t.Parallel()
	r := newRig(t,
		row(1, "NEW3", underlying.StatusUnresolved, 3*24*time.Hour),
		row(2, "OLD8", underlying.StatusUnresolved, 8*24*time.Hour))
	r.b.answers["OLD8"] = listed(5)
	r.run(t)
	if len(r.b.asked) != 1 || r.b.asked[0].Symbol != "OLD8" {
		t.Fatalf("asked %+v", r.b.asked)
	}
}

func TestAtMostTwentyNamesPerRun(t *testing.T) {
	t.Parallel()
	var rows []underlying.Row
	for i := 1; i <= 21; i++ {
		sym := fmt.Sprintf("S%02d", i)
		rows = append(rows, row(int64(i), sym, underlying.StatusPending, time.Hour))
	}
	r := newRig(t, rows...)
	keys, _, err := r.j.Plan(context.Background(), "2026-11-11")
	if err != nil || len(keys) != 20 {
		t.Fatalf("keys %d, %v", len(keys), err)
	}
	// even a caller that hands over all 21 keys gets 20 asked
	all := make([]contracts.ExpectedKey, 0, 21)
	for _, rw := range rows {
		all = append(all, contracts.ExpectedKey{DatasetID: DatasetID, SubjectKey: rw.Key})
	}
	res := r.j.Run(context.Background(), "2026-11-11", all)
	if res.Returned != 20 || len(r.b.asked) != 20 {
		t.Fatalf("returned %d asked %d", res.Returned, len(r.b.asked))
	}
	for _, c := range r.b.asked {
		if c.Symbol == "S21" {
			t.Fatal("the 21st name was asked")
		}
	}
}

func TestSettleNotWrittenIsNoError(t *testing.T) {
	t.Parallel()
	r := newRig(t, row(1, "AAPL", underlying.StatusPending, time.Hour))
	r.s.written = false // the owner paused the name meanwhile
	r.b.answers["AAPL"] = listed(1)
	res := r.run(t)
	if res.Err != nil || res.Returned != 1 || res.Usable != 1 || len(r.s.settled) != 1 {
		t.Fatalf("result %+v settled %+v", res, r.s.settled)
	}
}

func TestSettleErrorIsRetryable(t *testing.T) {
	t.Parallel()
	r := newRig(t, row(1, "AAPL", underlying.StatusPending, time.Hour))
	r.b.answers["AAPL"] = listed(1)
	st := &errSettler{fakeSettler: r.s}
	j, _ := New(r.b, st, clockwork.NewFakeClockAt(start))
	keys, _, _ := j.Plan(context.Background(), "2026-11-11")
	res := j.Run(context.Background(), "2026-11-11", keys)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || res.Usable != 0 {
		t.Fatalf("result %+v", res)
	}
}

type errSettler struct{ *fakeSettler }

func (errSettler) Settle(context.Context, underlying.Outcome) (bool, error) {
	return false, errors.New("db down")
}

func TestNewRefusesNil(t *testing.T) {
	t.Parallel()
	b, s, c := &fakeBroker{}, &fakeSettler{}, clockwork.NewFakeClockAt(start)
	cases := map[string]func() error{
		"broker":  func() error { _, err := New(nil, s, c); return err },
		"settler": func() error { _, err := New(b, nil, c); return err },
		"clock":   func() error { _, err := New(b, s, nil); return err },
	}
	for name, f := range cases {
		if f() == nil {
			t.Errorf("nil %s: no error", name)
		}
	}
}

func TestScheduleOfJob(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	cal, anchor, zone, off := r.j.(interface {
		Schedule() (string, string, string, time.Duration)
	}).Schedule()
	if r.j.ID() != "underlying_resolve" || cal != "XNYS" || anchor != "session_close" ||
		zone != "America/New_York" || off != 45*time.Minute {
		t.Fatalf("%s %s %s %s %s", r.j.ID(), cal, anchor, zone, off)
	}
}
