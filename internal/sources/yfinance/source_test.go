package yfinance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

type call struct {
	argv []string
	req  wireRequest
}

// fakeRun answers each call by reply(callIndex, request); it records the calls.
type fakeRun struct {
	mu    sync.Mutex
	calls []call
	reply func(i int, r wireRequest) ([]byte, int, error)
}

func (f *fakeRun) run(_ context.Context, argv []string, stdin []byte) ([]byte, int, error) {
	var r wireRequest
	if err := json.Unmarshal(stdin, &r); err != nil {
		return nil, -1, err
	}
	f.mu.Lock()
	i := len(f.calls)
	f.calls = append(f.calls, call{argv, r})
	f.mu.Unlock()
	return f.reply(i, r)
}

func okReply(r wireRequest) []byte {
	var a wireAnswer
	for _, p := range r.Pairs {
		a.Results = append(a.Results, wireResult{Symbol: p.Symbol, Currency: p.Currency,
			Bars: []wireBar{{SessionDay: "2026-11-17", Close: "1.2300"}}})
	}
	b, _ := json.Marshal(a)
	return b
}

func pairs(n int) []sources.BarPair {
	out := make([]sources.BarPair, n)
	for i := range out {
		out[i] = sources.BarPair{Symbol: "S" + strconv.Itoa(i), Currency: "USD"}
	}
	return out
}

func request(n int) sources.BarRequest {
	return sources.BarRequest{Pairs: pairs(n), From: "2026-11-11", To: "2026-11-17"}
}

func newTest(f *fakeRun, clk *clockwork.FakeClock) *source {
	return newSource(sources.BarConfig{Command: []string{"cmd", "run"}}, f.run, func(d time.Time) time.Duration { return d.Sub(clk.Now()) })
}

func codes(r sources.BarResult) map[string]string {
	m := map[string]string{}
	for _, u := range r.Unanswered {
		m[u.Pair.Symbol] = u.Code
	}
	return m
}

func TestBatchesOfTen(t *testing.T) {
	t.Parallel()
	f := &fakeRun{reply: func(_ int, r wireRequest) ([]byte, int, error) { return okReply(r), 0, nil }}
	res, err := newTest(f, clockwork.NewFakeClock()).DailyBars(context.Background(), request(23))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 3 || len(f.calls[0].req.Pairs) != 10 || len(f.calls[2].req.Pairs) != 3 {
		t.Fatalf("calls = %d", len(f.calls))
	}
	if len(res.Bars) != 23 || len(res.Unanswered) != 0 {
		t.Fatalf("bars %d unanswered %d", len(res.Bars), len(res.Unanswered))
	}
	// no context deadline: the bound is 15 s per pair plus 15, and --deadline is that minus 10
	want := []string{"cmd", "run", "--deadline", "155"}
	for i, a := range f.calls[0].argv {
		if a != want[i] {
			t.Fatalf("argv = %v", f.calls[0].argv)
		}
	}
	if f.calls[2].argv[3] != "50" || f.calls[0].req.FromDay != "2026-11-11" || f.calls[0].req.ToDay != "2026-11-17" {
		t.Fatalf("last argv %v, request %+v", f.calls[2].argv, f.calls[0].req)
	}
}

func TestEarlierBatchesKept(t *testing.T) {
	t.Parallel()
	f := &fakeRun{reply: func(i int, r wireRequest) ([]byte, int, error) {
		if i == 1 {
			return nil, 1, nil
		}
		return okReply(r), 0, nil
	}}
	res, err := newTest(f, clockwork.NewFakeClock()).DailyBars(context.Background(), request(25))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bars) != 15 || len(res.Unanswered) != 10 {
		t.Fatalf("bars %d unanswered %d", len(res.Bars), len(res.Unanswered))
	}
	for _, u := range res.Unanswered {
		if u.Code != sources.BarCodeSourceChanged {
			t.Fatalf("code %q", u.Code)
		}
	}
}

func TestAllBatchesFailingIsAnError(t *testing.T) {
	t.Parallel()
	f := &fakeRun{reply: func(int, wireRequest) ([]byte, int, error) { return []byte("not json"), 0, nil }}
	_, err := newTest(f, clockwork.NewFakeClock()).DailyBars(context.Background(), request(3))
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v", err)
	}
	f = &fakeRun{reply: func(int, wireRequest) ([]byte, int, error) { return nil, -1, context.DeadlineExceeded }}
	if _, err := newTest(f, clockwork.NewFakeClock()).DailyBars(context.Background(), request(3)); err == nil {
		t.Fatal("no error")
	}
}

func TestKilledBatchIsTimeout(t *testing.T) {
	t.Parallel()
	f := &fakeRun{reply: func(i int, r wireRequest) ([]byte, int, error) {
		if i == 1 {
			return nil, -1, context.DeadlineExceeded
		}
		return okReply(r), 0, nil
	}}
	res, err := newTest(f, clockwork.NewFakeClock()).DailyBars(context.Background(), request(12))
	if err != nil || len(res.Unanswered) != 2 || res.Unanswered[0].Code != sources.BarCodeTimeout {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestRateLimitStopsBatches(t *testing.T) {
	t.Parallel()
	f := &fakeRun{reply: func(_ int, r wireRequest) ([]byte, int, error) {
		var a wireAnswer
		for i, p := range r.Pairs {
			w := wireResult{Symbol: p.Symbol, Currency: p.Currency}
			if i >= 4 {
				w.ErrorCode = "rate_limited"
			}
			a.Results = append(a.Results, w)
		}
		b, _ := json.Marshal(a)
		return b, 0, nil
	}}
	res, err := newTest(f, clockwork.NewFakeClock()).DailyBars(context.Background(), request(25))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("calls = %d", len(f.calls))
	}
	c := codes(res)
	if len(c) != 21 || c["S3"] != "" || c["S4"] != "rate_limited" || c["S24"] != "rate_limited" {
		t.Fatalf("codes = %v", c)
	}
}

func TestErrorCodeBecomesUnanswered(t *testing.T) {
	t.Parallel()
	f := &fakeRun{reply: func(_ int, r wireRequest) ([]byte, int, error) {
		a := wireAnswer{Results: []wireResult{
			{Symbol: "S0", Currency: "USD", ErrorCode: "no_data"},
			{Symbol: "S1", Currency: "USD", ErrorCode: "network"},
			{Symbol: "S2", Currency: "USD", Bars: []wireBar{{SessionDay: "2026-11-16", Close: "187.2000"}}},
		}}
		b, _ := json.Marshal(a)
		return b, 0, nil
	}}
	res, err := newTest(f, clockwork.NewFakeClock()).DailyBars(context.Background(), request(3))
	if err != nil {
		t.Fatal(err)
	}
	c := codes(res)
	if len(c) != 2 || c["S0"] != "no_data" || c["S1"] != "network" {
		t.Fatalf("codes = %v", c)
	}
	if len(res.Bars) != 1 || res.Bars[0] != (sources.RefBar{Pair: pairs(3)[2], Day: "2026-11-16", Close: "187.2000"}) {
		t.Fatalf("bars = %+v", res.Bars)
	}
}

func TestStopsAtContextDeadline(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	ctx, cancel := context.WithDeadline(context.Background(), clk.Now().Add(200*time.Second))
	defer cancel()
	f := &fakeRun{}
	f.reply = func(_ int, r wireRequest) ([]byte, int, error) {
		clk.Advance(100 * time.Second) // each batch takes 100 s of the fake time
		return okReply(r), 0, nil
	}
	res, err := newTest(f, clk).DailyBars(ctx, request(35))
	if err != nil {
		t.Fatal(err)
	}
	// batch 1 at 200 s left, batch 2 at 100 s left (bound 100, --deadline 90), then 0 left
	if len(f.calls) != 2 || f.calls[0].argv[3] != "155" || f.calls[1].argv[3] != "90" {
		t.Fatalf("calls %d argv %v", len(f.calls), f.calls)
	}
	c := codes(res)
	if len(res.Bars) != 20 || len(c) != 15 || c["S20"] != "timeout" || c["S34"] != "timeout" {
		t.Fatalf("bars %d codes %v", len(res.Bars), c)
	}
}

func TestStopsWhenTenSecondsLeft(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	ctx, cancel := context.WithDeadline(context.Background(), clk.Now().Add(10*time.Second))
	defer cancel()
	f := &fakeRun{reply: func(_ int, r wireRequest) ([]byte, int, error) { return okReply(r), 0, nil }}
	if _, err := newTest(f, clk).DailyBars(ctx, request(3)); err == nil || len(f.calls) != 0 {
		t.Fatalf("calls %d err %v", len(f.calls), err)
	}
}

func TestAnswerMatchesGolden(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../pyside/tests/refbars/answer.json")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRun{reply: func(int, wireRequest) ([]byte, int, error) { return raw, 0, nil }}
	req := sources.BarRequest{From: "2026-11-16", To: "2026-11-17", Pairs: []sources.BarPair{
		{Symbol: "AAPL", Currency: "USD"}, {Symbol: "7203", Currency: "JPY"}, {Symbol: "ZZZZ", Currency: "USD"}}}
	res, err := newTest(f, clockwork.NewFakeClock()).DailyBars(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	aapl := req.Pairs[0]
	wantBars := []sources.RefBar{{Pair: aapl, Day: "2026-11-16", Close: "189.5000"}, {Pair: aapl, Day: "2026-11-17", Close: "190.2500"}}
	if len(res.Bars) != 2 || res.Bars[0] != wantBars[0] || res.Bars[1] != wantBars[1] {
		t.Fatalf("bars = %+v", res.Bars)
	}
	if len(res.Unanswered) != 2 || res.Unanswered[0] != (sources.BarUnanswered{Pair: req.Pairs[1], Code: "rate_limited"}) ||
		res.Unanswered[1] != (sources.BarUnanswered{Pair: req.Pairs[2], Code: "no_data"}) {
		t.Fatalf("unanswered = %+v", res.Unanswered)
	}
}

func TestConformsToBarSource(t *testing.T) {
	t.Parallel()
	var _ sources.NewBarSourceFunc = New
	var _ sources.BarSource = (*source)(nil)
	src, err := New(sources.BarConfig{Command: []string{"x"}})
	if err != nil || src == nil {
		t.Fatalf("New = %v, %v", src, err)
	}
	if _, err := New(sources.BarConfig{}); err == nil {
		t.Fatal("an empty command was accepted")
	}
}
