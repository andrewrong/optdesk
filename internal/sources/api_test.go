package sources_test

import (
	"context"
	"io"
	"strings"
	"testing"

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
	src, err := newSource(sources.EarningsConfig{BaseURL: "http://unused.invalid", APIKey: "k", Symbols: []string{"7203"}})
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
