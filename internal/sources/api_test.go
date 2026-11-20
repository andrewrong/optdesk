package sources_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

type fakeSource struct{}

func (fakeSource) Fetch(_ context.Context, p sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, id := range p.SeriesIDs {
		out = append(out, contracts.SeriesPoint{SeriesID: id, Source: "fake"})
	}
	return out, nil
}

var _ sources.NewFREDFunc = func(cfg sources.FREDConfig) (sources.Source, error) { return fakeSource{}, nil }

func TestSourceContract(t *testing.T) {
	t.Parallel()
	newSource := sources.NewFREDFunc(func(sources.FREDConfig) (sources.Source, error) { return fakeSource{}, nil })
	src, err := newSource(sources.FREDConfig{BaseURL: "http://unused.invalid", Clock: clockwork.NewFakeClock()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2026-11-01", To: "2026-11-04"})
	if err != nil || len(got) != 1 || got[0].SeriesID != "DGS10" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

type fakeEarnings struct{}

func (fakeEarnings) Fetch(context.Context) ([]sources.EarningsListing, error) {
	return []sources.EarningsListing{{Symbol: "AAPL", ReportDate: "2026-11-10"}}, nil
}

func TestEarningsSourceContract(t *testing.T) {
	t.Parallel()
	newSource := sources.NewEarningsFunc(func(sources.EarningsConfig) (sources.EarningsSource, error) { return fakeEarnings{}, nil })
	src, err := newSource(sources.EarningsConfig{BaseURL: "http://unused.invalid", APIKey: "k", Symbols: []string{"7203"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.Fetch(context.Background())
	if err != nil || len(got) != 1 || got[0].Symbol != "AAPL" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestNewJQuantsFuncContract(t *testing.T) {
	t.Parallel()
	newSource := sources.NewJQuantsFunc(func(sources.EarningsConfig) (sources.EarningsSource, error) { return fakeEarnings{}, nil })
	src, err := newSource(sources.EarningsConfig{BaseURL: "http://unused.invalid", APIKey: "k", Symbols: []string{"7203"},
		Sleep: func(context.Context, time.Duration) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := src.Fetch(context.Background()); err != nil || len(got) != 1 {
		t.Fatalf("got %+v err %v", got, err)
	}
}

type fakeFacts struct{}

func (fakeFacts) Fetch(_ context.Context, p sources.FactPlan) ([]contracts.FundamentalFact, error) {
	var out []contracts.FundamentalFact
	for _, s := range p.Subjects {
		out = append(out, contracts.FundamentalFact{Subject: s, Source: "sec"})
	}
	return out, nil
}

type fakeCursor struct{ day contracts.Day }

func (c *fakeCursor) Last() (contracts.Day, error) { return c.day, nil }
func (c *fakeCursor) Save(d contracts.Day) error   { c.day = d; return nil }

func TestSECFactSourceContract(t *testing.T) {
	t.Parallel()
	newSource := sources.NewSECFunc(func(sources.SECConfig) (sources.FactSource, error) { return fakeFacts{}, nil })
	src, err := newSource(sources.SECConfig{BaseURL: "http://unused.invalid", UserAgent: "x", Clock: clockwork.NewFakeClock()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.Fetch(context.Background(), sources.FactPlan{Subjects: []string{"cik:0000320193"}, From: "2026-01-01", To: "2026-11-01"})
	if err != nil || len(got) != 1 || got[0].Subject != "cik:0000320193" {
		t.Fatalf("got %+v err %v", got, err)
	}
	var c sources.Cursor = &fakeCursor{}
	if err := c.Save("2026-11-01"); err != nil {
		t.Fatal(err)
	}
	if d, err := c.Last(); err != nil || d != "2026-11-01" {
		t.Fatalf("cursor %q %v", d, err)
	}
	var _ sources.NewEDINETFunc = func(sources.EDINETConfig) (sources.FactSource, error) { return fakeFacts{}, nil }
}

func TestParseFlexFuncContract(t *testing.T) {
	t.Parallel()
	parse := sources.ParseFlexFunc(func(r io.Reader) (sources.FlexReport, error) {
		b, err := io.ReadAll(r)
		return sources.FlexReport{RawSHA256: string(b)}, err
	})
	got, err := parse(strings.NewReader("body"))
	if err != nil || got.RawSHA256 != "body" || !got.FetchedAt.IsZero() {
		t.Fatalf("got %+v err %v", got, err)
	}
}

type fakeBars struct{}

func (fakeBars) DailyBars(ctx context.Context, req sources.BarRequest) (sources.BarResult, error) {
	var res sources.BarResult
	for _, p := range req.Pairs {
		switch {
		case ctx.Err() != nil:
			res.Unanswered = append(res.Unanswered, sources.BarUnanswered{Pair: p, Code: sources.BarCodeTimeout})
		case p.Symbol == "NONE":
			res.Unanswered = append(res.Unanswered, sources.BarUnanswered{Pair: p, Code: sources.BarCodeNoData})
		default:
			res.Bars = append(res.Bars, sources.RefBar{Pair: p, Day: req.From, Close: "187.2"})
		}
	}
	return res, nil
}

func TestFakeBarSourceServesCaller(t *testing.T) {
	t.Parallel()
	newSource := sources.NewBarSourceFunc(func(sources.BarConfig) (sources.BarSource, error) { return fakeBars{}, nil })
	src, err := newSource(sources.BarConfig{Command: []string{"unused"}, Key: "k", PerPair: 15 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	req := sources.BarRequest{
		Pairs: []sources.BarPair{{Symbol: "AAPL", Currency: "USD"}, {Symbol: "NONE", Currency: "USD"}},
		From:  "2026-11-17", To: "2026-11-17",
	}
	got, err := src.DailyBars(context.Background(), req)
	if err != nil || len(got.Bars) != 1 || got.Bars[0].Pair.Symbol != "AAPL" || got.Bars[0].Close != "187.2" {
		t.Fatalf("got %+v err %v", got, err)
	}
	if len(got.Unanswered) != 1 || got.Unanswered[0].Code != sources.BarCodeNoData {
		t.Fatalf("unanswered %+v", got.Unanswered)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = src.DailyBars(ctx, req)
	if err != nil || len(got.Bars) != 0 || len(got.Unanswered) != 2 || got.Unanswered[0].Code != sources.BarCodeTimeout {
		t.Fatalf("cancelled: got %+v err %v", got, err)
	}
}

func TestBarCodesAreFive(t *testing.T) {
	t.Parallel()
	want := []string{"rate_limited", "no_data", "network", "timeout", "source_changed"}
	got := []string{sources.BarCodeRateLimited, sources.BarCodeNoData, sources.BarCodeNetwork, sources.BarCodeTimeout, sources.BarCodeSourceChanged}
	seen := map[string]bool{}
	for i, c := range got {
		if c != want[i] || seen[c] {
			t.Fatalf("code %d is %q, want %q (seen %v)", i, c, want[i], seen[c])
		}
		seen[c] = true
	}
}

func TestRefBarKeyedLiteral(t *testing.T) {
	t.Parallel()
	pair := sources.BarPair{}
	old := sources.RefBar{Pair: pair, Close: "10.0000"}
	full := sources.RefBar{Pair: pair, Close: "10.0000", Open: "", High: "", Low: "", Volume: ""}
	if old != full {
		t.Fatalf("keyed literal without the new fields %+v differs from %+v", old, full)
	}
	filled := sources.RefBar{Pair: pair, Close: "10.0000", Open: "9.0000", High: "11.0000", Low: "8.5000", Volume: "100"}
	if filled == old || filled.Open != "9.0000" || filled.High != "11.0000" || filled.Low != "8.5000" || filled.Volume != "100" {
		t.Fatalf("new fields not carried: %+v", filled)
	}
}
