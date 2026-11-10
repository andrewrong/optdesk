package valuation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// goldenReturnCase is one case of testdata/golden_return.json: the inputs as the old system
// took them and its output as decimal text.
type goldenReturnCase struct {
	Name  string `json:"name"`
	Input struct {
		Years []struct {
			PeriodEnd string  `json:"period_end"`
			Revenue   *string `json:"revenue"`
			OCF       *string `json:"operating_cash_flow"`
			Capex     *string `json:"capital_spending"`
			Dep       *string `json:"depreciation"`
			SBC       *string `json:"stock_compensation"`
			Shares    *string `json:"diluted_shares"`
			Plant     *string `json:"gross_plant"`
			Equity    *string `json:"equity"`
		} `json:"years"`
		SharesNow      string   `json:"shares_now"`
		Costs          []string `json:"costs"`
		PriceDate      string   `json:"price_date"`
		AnalystRevenue []string `json:"analyst_revenue"`
	} `json:"input"`
	Expected returnOut `json:"expected"`
}

func loadReturnGolden(t *testing.T) []goldenReturnCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "golden_return.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []goldenReturnCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("no cases")
	}
	return f.Cases
}

func (c goldenReturnCase) find(t *testing.T) goldenReturnCase {
	t.Helper()
	for _, g := range loadReturnGolden(t) {
		if g.Name == c.Name {
			return g
		}
	}
	t.Fatalf("no golden case %q", c.Name)
	return c
}

func goldenCase(t *testing.T, name string) goldenReturnCase {
	t.Helper()
	return goldenReturnCase{Name: name}.find(t)
}

// filingsOfReturn builds the Filings of a golden case, newest first.
func filingsOfReturn(c goldenReturnCase) valuationapi.Filings {
	f := valuationapi.Filings{Standard: valuationapi.USGAAP, SharesOutstanding: c.Input.SharesNow}
	for i := len(c.Input.Years) - 1; i >= 0; i-- {
		y := c.Input.Years[i]
		vals := map[string]string{}
		for role, v := range map[string]*string{
			roleRevenue: y.Revenue, roleOperatingCashFlow: y.OCF, roleCapitalSpending: y.Capex,
			roleDepreciation: y.Dep, roleStockCompensation: y.SBC, roleDilutedShares: y.Shares,
			roleGrossPlant: y.Plant, roleEquity: y.Equity,
		} {
			if v != nil {
				vals[role] = *v
			}
		}
		f.Years = append(f.Years, valuationapi.FilingYear{FYEnd: contracts.Day(y.PeriodEnd), Values: vals})
	}
	return f
}

// runCore runs the core on a golden case, anchoring any analyst revenue to the latest
// reported revenue.
func runCore(t *testing.T, c goldenReturnCase) (returnOut, string) {
	t.Helper()
	f := filingsOfReturn(c)
	var years []retYear
	for _, y := range f.Years {
		ry, ok := yearOf(y)
		if !ok {
			t.Fatal("bad year")
		}
		years = append(years, ry)
	}
	day, _ := parseDay(contracts.Day(c.Input.PriceDate))
	var costs []decimal.Decimal
	for _, s := range c.Input.Costs {
		costs = append(costs, decimal.RequireFromString(s))
	}
	var analyst *analystIn
	if c.Input.AnalystRevenue != nil {
		var rev []decimal.Decimal
		for _, s := range c.Input.AnalystRevenue {
			rev = append(rev, decimal.RequireFromString(s))
		}
		reported := decimal.RequireFromString(*c.Input.Years[len(c.Input.Years)-1].Revenue)
		a := analystFromRevenue(rev, reported)
		analyst = &a
	}
	return priceReturn(years, decimal.RequireFromString(c.Input.SharesNow), costs, day, analyst)
}

func TestReturnGolden(t *testing.T) {
	t.Parallel()
	for _, c := range loadReturnGolden(t) {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			got, why := runCore(t, c)
			if c.Expected.State == "refused" {
				if why != c.Expected.Reason {
					t.Fatalf("refusal %q, want %q", why, c.Expected.Reason)
				}
				return
			}
			if why != "" {
				t.Fatalf("refused with %q, want an answer", why)
			}
			if !reflect.DeepEqual(got, c.Expected) {
				g, _ := json.Marshal(got)
				w, _ := json.Marshal(c.Expected)
				t.Fatalf("result differs\n got: %s\nwant: %s", g, w)
			}
		})
	}
}

func TestNilAnalystLeavesTwoLinesOut(t *testing.T) {
	t.Parallel()
	c := goldenCase(t, "steady_no_analyst")
	res := ReturnAtPrice(filingsOfReturn(c), c.Input.Costs, nil, contracts.Day(c.Input.PriceDate))
	if res.State != valuationapi.StateOK {
		t.Fatalf("state %q, reasons %v", res.State, res.Reasons)
	}
	if len(res.Lines) != 4*len(c.Input.Costs) {
		t.Fatalf("%d lines, want %d", len(res.Lines), 4*len(c.Input.Costs))
	}
	want := c.Expected.Scenarios
	for i, cost := range c.Input.Costs {
		for k, line := range res.Lines[4*i : 4*i+4] {
			if line.Cost != cost || line.Key != want[k].Key {
				t.Fatalf("line %d/%d is %s at %s, want %s at %s", i, k, line.Key, line.Cost, want[k].Key, cost)
			}
			if k >= 2 {
				if line.State != "left_out" || line.Reason != "no analyst input" || line.Value != "" {
					t.Errorf("%s at %s: %+v, want left out: no analyst input", line.Key, cost, line)
				}
				continue
			}
			if line.State != valuationapi.StateOK || line.Value != *want[k].Solved[i].Rate {
				t.Errorf("%s at %s: %+v, want rate %s", line.Key, cost, line, *want[k].Solved[i].Rate)
			}
		}
	}
	if len(res.Flags) != 0 {
		t.Errorf("flags %v, want none", res.Flags)
	}
}

func TestAnalystInputGivesTheAnalystLines(t *testing.T) {
	t.Parallel()
	c := goldenCase(t, "steady_with_analyst")
	a := valuationapi.Analyst{Growth: c.Expected.Analyst.Growth}
	res := ReturnAtPrice(filingsOfReturn(c), c.Input.Costs, &a, contracts.Day(c.Input.PriceDate))
	if res.State != valuationapi.StateOK {
		t.Fatalf("state %q, reasons %v", res.State, res.Reasons)
	}
	for _, line := range res.Lines {
		if line.State != valuationapi.StateOK || line.Value == "" {
			t.Errorf("%s at %s: %+v, want a rate", line.Key, line.Cost, line)
		}
	}
	bad := ReturnAtPrice(filingsOfReturn(c), c.Input.Costs, &valuationapi.Analyst{Growth: "x"}, contracts.Day(c.Input.PriceDate))
	for _, line := range bad.Lines {
		if strings.HasPrefix(line.Key, "ANALYST") && line.State != valuationapi.StateMissingInput {
			t.Errorf("%s: %+v, want missing_input for a growth that is no number", line.Key, line)
		}
	}
}

func TestGrowthLimitedByR(t *testing.T) {
	t.Parallel()
	c := goldenCase(t, "growth_limited_by_r")
	got, why := runCore(t, c)
	if why != "" {
		t.Fatalf("refused: %s", why)
	}
	var limitedLines int
	for _, s := range got.Scenarios {
		for _, f := range s.Flags {
			if f == flagGrowthLimitedByR {
				limitedLines++
				if s.R == nil {
					t.Errorf("%s is limited by R but holds none", s.Key)
				}
			}
		}
	}
	if limitedLines == 0 {
		t.Fatalf("no line is flagged %s", flagGrowthLimitedByR)
	}
	// The public result carries the flag once.
	a := valuationapi.Analyst{Growth: got.Analyst.Growth}
	res := ReturnAtPrice(filingsOfReturn(c), c.Input.Costs, &a, contracts.Day(c.Input.PriceDate))
	n := 0
	for _, f := range res.Flags {
		if f == flagGrowthLimitedByR {
			n++
		}
	}
	if n != 1 {
		t.Errorf("flags %v, want %s once", res.Flags, flagGrowthLimitedByR)
	}
}
