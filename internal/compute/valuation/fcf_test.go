package valuation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// goldenFcfCase is one case of testdata/golden_fcf.json: the inputs as the old system took
// them and its outputs as Decimal text.
type goldenFcfCase struct {
	Name  string `json:"name"`
	Input struct {
		Years []struct {
			End   string `json:"end"`
			OCF   string `json:"ocf"`
			Capex string `json:"capex"`
			SBC   string `json:"sbc"`
		} `json:"years"`
		Shares      string  `json:"shares"`
		CoverShares *string `json:"cover_shares"`
		Debt        *string `json:"debt"`
		AsOf        string  `json:"as_of"`
	} `json:"input"`
	Expected struct {
		Band struct {
			State        string            `json:"state"`
			Reasons      []string          `json:"reasons"`
			Values       map[string]string `json:"values"`
			PrimaryValue string            `json:"primary_value"`
			Flags        []string          `json:"flags"`
		} `json:"band"`
		Reverse struct {
			State   string   `json:"state"`
			Reasons []string `json:"reasons"`
			Implied []struct {
				Spot  string `json:"spot"`
				Rates []struct {
					Rate   string  `json:"rate"`
					Status string  `json:"status"`
					Growth *string `json:"growth"`
				} `json:"rates"`
			} `json:"implied"`
		} `json:"reverse"`
	} `json:"expected"`
}

func loadFcfGolden(t *testing.T) []goldenFcfCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "golden_fcf.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []goldenFcfCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("no golden cases")
	}
	return f.Cases
}

// filingsOf builds the Filings of a golden case: the years newest first, the debt on the
// latest year, a SIC outside the financial and cyclical ranges.
func filingsOf(c goldenFcfCase) valuationapi.Filings {
	f := valuationapi.Filings{
		Standard: valuationapi.USGAAP, Currency: "USD", SIC: "7372", SharesOutstanding: c.Input.Shares,
	}
	if c.Input.CoverShares != nil {
		f.CoverShares = *c.Input.CoverShares
	}
	for i := len(c.Input.Years) - 1; i >= 0; i-- {
		y := c.Input.Years[i]
		fy := valuationapi.FilingYear{FYEnd: contracts.Day(y.End), Values: map[string]string{
			roleOperatingCashFlow: y.OCF, roleCapitalSpending: y.Capex, roleStockCompensation: y.SBC,
		}}
		if i == len(c.Input.Years)-1 && c.Input.Debt != nil {
			fy.Values[roleLongTermDebt] = *c.Input.Debt
		}
		f.Years = append(f.Years, fy)
	}
	return f
}

// plainReasons drops the detail reasons of the old system ("insufficient_years:ocf=4"),
// which this port folds into the plain reason that follows them.
func plainReasons(rs []string) []string {
	var out []string
	for _, r := range rs {
		if !strings.Contains(r, ":") {
			out = append(out, r)
		}
	}
	return out
}

func TestBandGolden(t *testing.T) {
	t.Parallel()
	for _, c := range loadFcfGolden(t) {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			got := Band("AAA", filingsOf(c), contracts.Day(c.Input.AsOf))
			want := c.Expected.Band
			if got.ID != "AAA" {
				t.Errorf("ID %q", got.ID)
			}
			if want.State != "ok" {
				if got.State != valuationapi.StateMissingInput {
					t.Fatalf("state %q, want missing_input", got.State)
				}
				if !reflect.DeepEqual(got.Reasons, plainReasons(want.Reasons)) {
					t.Errorf("reasons %v, want %v", got.Reasons, plainReasons(want.Reasons))
				}
				if got.Value != "" || got.Low != "" || got.RefHigh != "" {
					t.Errorf("a refused band holds values: %+v", got)
				}
				return
			}
			if got.State != valuationapi.StateOK || len(got.Reasons) != 0 {
				t.Fatalf("state %q reasons %v", got.State, got.Reasons)
			}
			if got.Value != want.PrimaryValue || got.Value != want.Values["0.08"] {
				t.Errorf("value %q, want %q", got.Value, want.PrimaryValue)
			}
			if got.Low != want.Values["0.10"] || got.RefHigh != want.Values["0.06"] {
				t.Errorf("low %q ref_high %q, want %q %q", got.Low, got.RefHigh, want.Values["0.10"], want.Values["0.06"])
			}
			if got.Currency != "USD" || got.BaseFYEnd != "2024-12-31" {
				t.Errorf("currency %q base %q", got.Currency, got.BaseFYEnd)
			}
			if len(got.Flags) != len(want.Flags) || (len(want.Flags) > 0 && !reflect.DeepEqual(got.Flags, want.Flags)) {
				t.Errorf("flags %v, want %v", got.Flags, want.Flags)
			}
		})
	}
}

// The band is price-free: the function takes the symbol, the filings and the as-of day.
func TestNoPriceParameter(t *testing.T) {
	t.Parallel()
	var fn valuationapi.BandFunc = Band
	typ := reflect.TypeOf(fn)
	want := []reflect.Type{
		reflect.TypeOf(""), reflect.TypeOf(valuationapi.Filings{}), reflect.TypeOf(contracts.Day("")),
	}
	if typ.NumIn() != len(want) {
		t.Fatalf("%d parameters, want %d", typ.NumIn(), len(want))
	}
	for i, w := range want {
		if typ.In(i) != w {
			t.Errorf("parameter %d is %v, want %v", i, typ.In(i), w)
		}
	}
	if _, ok := reflect.TypeOf(valuationapi.BandFigure{}).FieldByName("Spot"); ok {
		t.Error("BandFigure carries a price")
	}
}

func TestBandUnvaluedSIC(t *testing.T) {
	t.Parallel()
	c := loadFcfGolden(t)[0]
	for sic, reason := range map[string]string{"6022": reasonFinancialSIC, "3312": reasonCyclicalSIC} {
		f := filingsOf(c)
		f.SIC = sic
		got := Band("AAA", f, "2025-03-10")
		if got.State != valuationapi.StateUnvalued || !reflect.DeepEqual(got.Reasons, []string{reason}) {
			t.Errorf("sic %s: %+v", sic, got)
		}
	}
}
