package companies_page

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const testToday = contracts.Day("2026-11-10")

type fakeRead struct {
	rows []contracts.ReadingRecord
	err  error
}

func (f fakeRead) WinnerReadings(_ context.Context, name, kind, subject string, _, _ contracts.Day) ([]contracts.ReadingRecord, error) {
	if name != "valuation.band_approval" || kind != "underlying" || subject != "SYN1" {
		return nil, nil
	}
	return f.rows, f.err
}

func (fakeRead) Close() error { return nil }

type fakeFacts struct{}

func (fakeFacts) FactsFiledBy(context.Context, string, string, contracts.Day) ([]contracts.FundamentalFact, error) {
	return []contracts.FundamentalFact{{}}, nil
}

type fakeOverride map[string]string

func (f fakeOverride) Line(_ context.Context, symbol string) (string, bool, error) {
	p, ok := f[symbol]
	return p, ok, nil
}

type fakeText map[string]string

func (f fakeText) Moat(_ context.Context, symbol string) (string, bool, error) {
	t, ok := f[symbol]
	return t, ok, nil
}

func (fakeText) FifthQuestion(context.Context, string) (string, bool, error) { return "", false, nil }

func approval(day, value string) contracts.ReadingRecord {
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	return contracts.ReadingRecord{
		Day:   types.SerializableDate{Time: d},
		State: "ok",
		Value: &value,
	}
}

// okBand is a band whose 8% value is 142.50.
var okBand = valuationapi.BandFigure{ID: "valuation.buy_band", State: valuationapi.StateOK, Value: "142.50", Low: "114.00", RefHigh: "190.00", Currency: "USD"}

func testConfig(band valuationapi.BandFigure, rows ...contracts.ReadingRecord) Config {
	return Config{
		Today:    func() contracts.Day { return testToday },
		Subjects: []Subject{{Issuer: "cik:0000000001", Market: "US", Symbol: "SYN1", HandPrice: "150"}},
		Read:     fakeRead{rows: rows},
		Facts:    fakeFacts{},
		StandardOf: func([]contracts.FundamentalFact) valuationapi.Standard {
			return valuationapi.USGAAP
		},
		BuildFilings: func([]contracts.FundamentalFact, contracts.Day, valuationapi.Standard) valuationapi.FilingsResult {
			return valuationapi.FilingsResult{State: valuationapi.StateOK}
		},
		Band: func(string, valuationapi.Filings, contracts.Day) valuationapi.BandFigure { return band },
	}
}

type testCell struct {
	Text       string `json:"text"`
	PriceState string `json:"price_state"`
	HandPrice  string `json:"hand_price"`
}

// priceCell builds the page and returns the price line cell of the first watch row.
func priceCell(t *testing.T, cfg Config) testCell {
	t.Helper()
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		SchemaVersion int `json:"schema_version"`
		WatchList     struct {
			Rows []struct {
				Cells []testCell `json:"cells"`
			} `json:"rows"`
		} `json:"watch_list"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.SchemaVersion != 2 || len(page.WatchList.Rows) != 1 {
		t.Fatalf("page = %s", b)
	}
	return page.WatchList.Rows[0].Cells[1]
}

func TestBandIsThePriceLine(t *testing.T) {
	t.Parallel()
	c := priceCell(t, testConfig(okBand, approval("2026-10-20", "142.50")))
	if c.Text != "114.00 to 142.50" || c.PriceState != "band" {
		t.Fatalf("cell = %+v", c)
	}
}

func TestOverrideIsLabelled(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand, approval("2026-10-20", "142.50"))
	cfg.Override = fakeOverride{"SYN1": "120"}
	c := priceCell(t, cfg)
	if c.Text != "120" || c.PriceState != "owner override" {
		t.Fatalf("cell = %+v", c)
	}
}

func TestExpiredAfter90Days(t *testing.T) {
	t.Parallel()
	// 2026-11-10 is exactly 90 days after 2026-08-12: still a band; one day earlier is expired.
	c := priceCell(t, testConfig(okBand, approval("2026-08-12", "142.50")))
	if c.PriceState != "band" {
		t.Fatalf("day 90: %+v", c)
	}
	c = priceCell(t, testConfig(okBand, approval("2026-08-11", "142.50")))
	if c.PriceState != "expired" || c.Text != "expired" {
		t.Fatalf("day 91: %+v", c)
	}
}

func TestAwaitingWhenValueDiffers(t *testing.T) {
	t.Parallel()
	c := priceCell(t, testConfig(okBand, approval("2026-10-20", "130.00")))
	if c.PriceState != "awaiting approval" || c.Text != "awaiting approval: 114.00 to 142.50" {
		t.Fatalf("different value: %+v", c)
	}
	c = priceCell(t, testConfig(okBand))
	if c.PriceState != "awaiting approval" {
		t.Fatalf("no approval: %+v", c)
	}
}

func TestBankIsUnvalued(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.BuildFilings = func([]contracts.FundamentalFact, contracts.Day, valuationapi.Standard) valuationapi.FilingsResult {
		return valuationapi.FilingsResult{State: valuationapi.StateUnvalued, Reasons: []string{"financial_sic"}}
	}
	c := priceCell(t, cfg)
	if c.Text != "unvalued: financial_sic" || c.PriceState != "" {
		t.Fatalf("filings refusal: %+v", c)
	}
	cfg = testConfig(valuationapi.BandFigure{State: valuationapi.StateMissingInput, Reasons: []string{"fewer_than_5_years"}})
	c = priceCell(t, cfg)
	if c.Text != "unvalued: fewer_than_5_years" || c.PriceState != "" {
		t.Fatalf("band missing input: %+v", c)
	}
}

func TestHandPriceShownBesideBand(t *testing.T) {
	t.Parallel()
	c := priceCell(t, testConfig(okBand, approval("2026-10-20", "142.50")))
	if c.HandPrice != "150" {
		t.Fatalf("hand price = %q", c.HandPrice)
	}
	cfg := testConfig(okBand)
	cfg.Subjects[0].HandPrice = "150.0"
	if c := priceCell(t, cfg); c.HandPrice != "150.0" {
		t.Fatalf("hand price changed: %q", c.HandPrice)
	}
	cfg.Subjects[0].HandPrice = ""
	if c := priceCell(t, cfg); c.HandPrice != "" {
		t.Fatalf("hand price invented: %q", c.HandPrice)
	}
}

func TestAwaitingStillShowsTheFigure(t *testing.T) {
	t.Parallel()
	c := priceCell(t, testConfig(okBand))
	if c.PriceState != "awaiting approval" || !strings.Contains(c.Text, "142.50") {
		t.Fatalf("awaiting cell hides the band: %+v", c)
	}
}

func TestOverrideTextNamedUnvaluedIsNotListed(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Override = fakeOverride{"SYN1": "unvalued by me"}
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Unvalued    *string `json:"unvalued"`
		GeneratedAt string  `json:"generated_at"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.Unvalued != nil {
		t.Fatalf("unvalued = %q", *page.Unvalued)
	}
	if !strings.HasSuffix(page.GeneratedAt, "+08:00") {
		t.Fatalf("generated_at = %q", page.GeneratedAt)
	}
}
