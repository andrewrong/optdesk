package company_data_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

func TestRowsCopySummaryOnly(t *testing.T) {
	t.Parallel()
	calc := &fakeCalc{sum: companydataapi.Summary{BarCount: 7}}
	in, b := newInput([]underlying.Row{aapl}, calc)
	b.bars[1] = []contracts.PriceBar{mkBar("2026-11-12"), mkBar("2026-11-13"), mkBar("2026-11-16")}
	c := companies(build(t, in))[0]
	if c["bar_count"] != "7" {
		t.Fatalf("bar_count = %v", c["bar_count"])
	}
	if len(calc.got) != 1 || len(calc.got[0].BarDays) != 3 || calc.got[0].BarDays[2] != "2026-11-16" {
		t.Fatalf("Summarize got %+v", calc.got)
	}
	if calc.got[0].Market != "US" || calc.got[0].Issuer != aapl.Issuer || !calc.got[0].Fundamentals || !calc.got[0].HasInstrument {
		t.Fatalf("Summarize got %+v", calc.got[0])
	}
}

func TestNoInstrumentReadsNoBars(t *testing.T) {
	t.Parallel()
	row := aapl
	row.ConID = 0
	calc := &fakeCalc{}
	in, b := newInput([]underlying.Row{row}, calc)
	b.failFor = 0 // would fail if read with id 0
	companies(build(t, in))
	if calc.got[0].HasInstrument || len(calc.got[0].BarDays) != 0 {
		t.Fatalf("Summarize got %+v", calc.got[0])
	}
}
