package optionfigapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionfigapi"
)

// fake keeps the promises of the interface: it remembers its input and returns one
// fixed result with every field set.
type fake struct {
	got optionfigapi.PutFigureInput
}

func (f *fake) Put(in optionfigapi.PutFigureInput) optionfigapi.PutFigures {
	f.got = in
	return optionfigapi.PutFigures{
		ItmPercent: "27.5%", ItmKey: "2755", ItmGap: "stale_input", ItmWhy: "cash rate is 9 days old",
		ExcessPercent: "4.5%", ExcessAbove: true, ExcessKey: "-450", ExcessGap: "missing_input", ExcessWhy: "annual yield not stored",
	}
}

// caller uses the interface only: it sets every input field and reads every output field.
func caller(f optionfigapi.OptionFigures) optionfigapi.PutFigures {
	return f.Put(optionfigapi.PutFigureInput{
		ChainDay: "2026-11-13", DTE: 30, Spot: "100", Strike: "95", IV: "0.3",
		AnnualYield: "12", Rate: "4.5", RateDay: "2026-11-12", MaxRateAgeDays: 7,
	})
}

func TestOptionFiguresContract(t *testing.T) {
	t.Parallel()
	f := &fake{}
	var ctor optionfigapi.NewOptionFiguresFunc = func() optionfigapi.OptionFigures { return f }
	out := caller(ctor())

	want := optionfigapi.PutFigureInput{
		ChainDay: "2026-11-13", DTE: 30, Spot: "100", Strike: "95", IV: "0.3",
		AnnualYield: "12", Rate: "4.5", RateDay: "2026-11-12", MaxRateAgeDays: 7,
	}
	if f.got != want {
		t.Fatalf("input = %+v, want %+v", f.got, want)
	}
	if out.ItmPercent != "27.5%" || out.ItmKey != "2755" || out.ItmGap != "stale_input" || out.ItmWhy != "cash rate is 9 days old" {
		t.Errorf("itm fields = %+v", out)
	}
	if out.ExcessPercent != "4.5%" || !out.ExcessAbove || out.ExcessKey != "-450" ||
		out.ExcessGap != "missing_input" || out.ExcessWhy != "annual yield not stored" {
		t.Errorf("excess fields = %+v", out)
	}
}
