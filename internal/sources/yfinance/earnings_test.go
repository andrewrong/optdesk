package yfinance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/sources"
)

type fakeRefearn struct {
	mu       sync.Mutex
	requests []earningsRequest
	answer   func(call int, req earningsRequest) ([]byte, int, error)
}

func (f *fakeRefearn) run(_ context.Context, argv []string, stdin []byte) ([]byte, int, error) {
	var req earningsRequest
	if err := json.Unmarshal(stdin, &req); err != nil {
		return nil, -1, err
	}
	f.mu.Lock()
	f.requests = append(f.requests, req)
	call := len(f.requests)
	f.mu.Unlock()
	if !slices.Contains(argv, "optdesk.refearn") || !slices.Contains(argv, "--deadline") {
		return nil, -1, fmt.Errorf("unexpected argv %v", argv)
	}
	return f.answer(call, req)
}

func okAnswer(req earningsRequest, errorCode string) []byte {
	var rs []map[string]any
	for _, s := range req.Symbols {
		dates := []string{"2026-12-03"}
		if errorCode != "" {
			dates = []string{}
		}
		rs = append(rs, map[string]any{"symbol": s.Symbol, "currency": s.Currency, "error_code": errorCode, "dates": dates})
	}
	b, _ := json.Marshal(map[string]any{"results": rs})
	return b
}

func symbols(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("S%02d", i)
	}
	return out
}

func newTestEarnings(t *testing.T, market string, f *fakeRefearn, syms []string) sources.EarningsSource {
	t.Helper()
	src, err := NewEarnings(market, f.run, sources.EarningsConfig{Symbols: syms})
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var ce sources.CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("not a CodedError: %v", err)
	}
	return ce.ErrorCode()
}

func TestEarningsOneBatch(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/earnings_answer.json")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRefearn{answer: func(int, earningsRequest) ([]byte, int, error) { return body, 0, nil }}
	src := newTestEarnings(t, "US", f, []string{"AAPL", "MSFT", "NVDA"})
	got, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []sources.EarningsListing{{Symbol: "AAPL", ReportDate: "2026-12-03"}, {Symbol: "AAPL", ReportDate: "2026-12-07"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if len(f.requests) != 1 || f.requests[0].Symbols[0].Currency != "USD" || f.requests[0].FromDay == "" {
		t.Fatalf("requests %+v", f.requests)
	}
}

func TestEarningsJapanAsksYen(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(_ int, r earningsRequest) ([]byte, int, error) { return okAnswer(r, ""), 0, nil }}
	src := newTestEarnings(t, "JP", f, []string{"7203"})
	if _, err := src.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.requests[0].Symbols[0] != (earningsSymbol{Symbol: "7203", Currency: "JPY"}) {
		t.Fatalf("%+v", f.requests[0])
	}
}

func TestEarningsUnknownMarket(t *testing.T) {
	t.Parallel()
	if _, err := NewEarnings("XX", (&fakeRefearn{}).run, sources.EarningsConfig{}); err == nil {
		t.Fatal("want an error")
	}
}

func TestEarningsSplitsBatchesOfTen(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(_ int, r earningsRequest) ([]byte, int, error) { return okAnswer(r, ""), 0, nil }}
	src := newTestEarnings(t, "US", f, symbols(25))
	got, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int
	for _, r := range f.requests {
		sizes = append(sizes, len(r.Symbols))
	}
	if !slices.Equal(sizes, []int{10, 10, 5}) || len(got) != 25 {
		t.Fatalf("sizes %v listings %d", sizes, len(got))
	}
}

func TestEarningsRateLimitStopsLaterBatches(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(call int, r earningsRequest) ([]byte, int, error) {
		if call == 2 {
			return okAnswer(r, "rate_limited"), 0, nil
		}
		return okAnswer(r, ""), 0, nil
	}}
	src := newTestEarnings(t, "US", f, symbols(25))
	got, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("partly answered must not fail: %v", err)
	}
	if len(f.requests) != 2 || len(got) != 10 {
		t.Fatalf("requests %d listings %d", len(f.requests), len(got))
	}
}

func TestEarningsRateLimitFirstBatchIsAnError(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(_ int, r earningsRequest) ([]byte, int, error) { return okAnswer(r, "rate_limited"), 0, nil }}
	_, err := newTestEarnings(t, "US", f, symbols(25)).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "rate_limited" || len(f.requests) != 1 {
		t.Fatalf("err %v requests %d", err, len(f.requests))
	}
}

func TestEarningsAllNoData(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(_ int, r earningsRequest) ([]byte, int, error) { return okAnswer(r, "no_data"), 0, nil }}
	_, err := newTestEarnings(t, "US", f, symbols(12)).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "no_data" {
		t.Fatalf("err %v", err)
	}
}

func TestEarningsNonZeroExit(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(int, earningsRequest) ([]byte, int, error) { return []byte("x"), 2, nil }}
	_, err := newTestEarnings(t, "US", f, symbols(12)).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "source_changed" {
		t.Fatalf("err %v", err)
	}
}

func TestEarningsBadJSON(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(int, earningsRequest) ([]byte, int, error) { return []byte("{not json"), 0, nil }}
	_, err := newTestEarnings(t, "US", f, symbols(3)).Fetch(context.Background())
	if err == nil || codeOf(t, err) != "source_changed" {
		t.Fatalf("err %v", err)
	}
}

func TestEarningsRunFailureIsTimeoutOrSourceChanged(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err  error
		code string
	}{{context.DeadlineExceeded, "timeout"}, {errors.New("exec /secret/path/uv: not found"), "source_changed"}} {
		f := &fakeRefearn{answer: func(int, earningsRequest) ([]byte, int, error) { return nil, -1, c.err }}
		_, err := newTestEarnings(t, "US", f, symbols(1)).Fetch(context.Background())
		if err == nil || codeOf(t, err) != c.code {
			t.Fatalf("err %v want %s", err, c.code)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("error leaks command text: %v", err)
		}
	}
}

func TestEarningsLaterBatchFailureKeepsEarlierListings(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(call int, r earningsRequest) ([]byte, int, error) {
		if call == 2 {
			return nil, 1, nil
		}
		return okAnswer(r, ""), 0, nil
	}}
	got, err := newTestEarnings(t, "US", f, symbols(15)).Fetch(context.Background())
	if err != nil || len(got) != 10 {
		t.Fatalf("err %v listings %d", err, len(got))
	}
}

func TestEarningsFromDayIsTodayOfTheClock(t *testing.T) {
	t.Parallel()
	f := &fakeRefearn{answer: func(_ int, r earningsRequest) ([]byte, int, error) { return okAnswer(r, ""), 0, nil }}
	src := newTestEarnings(t, "US", f, symbols(1)).(*earningsSource)
	src.now = func() time.Time { return time.Date(2026, 11, 27, 23, 0, 0, 0, time.UTC) }
	if _, err := src.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.requests[0].FromDay != "2026-11-27" {
		t.Fatalf("from_day %q", f.requests[0].FromDay)
	}
}
