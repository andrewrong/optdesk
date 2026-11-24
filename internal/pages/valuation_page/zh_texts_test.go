package valuation_page

import (
	"context"
	"errors"
	"testing"
	"unicode"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

type failingCloses struct{}

func (failingCloses) StockIDs(context.Context, []string) ([]store.InstrumentName, error) {
	return nil, errors.New("down")
}

func (failingCloses) PriceBars(context.Context, int64, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
	return nil, errors.New("down")
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func TestValuationGapTextsAreChinese(t *testing.T) {
	t.Parallel()
	old := []string{
		"Choose the R market of this company.", "No company with this key.",
		"The R version or the margin table is not stored; no price line is shown.",
		"The close price could not be read.", "A close price could not be read; the zone is missing for some names.",
		"The valuation store is not readable here.",
	}
	ctx := context.Background()
	entry := entryWith("US:AAA", "AAA", "US", "100", "3")
	withClose := func(in Input) Input { in.Closes = failingCloses{}; return in }
	noTables := func(in Input) Input { in.State.RVersion = nil; return in }
	noStore := func(in Input) Input { in.StateOK = false; return in }

	tableIn := baseInput(newFake())
	tableIn.State.Entries = []valuation.Entry{entry}
	companyIn := companyInput(newFake(), entry)
	formIn := formInput(newFake(), "US:AAA", Typed{V: "100", P: "3"})
	formIn.Underlying = []underlying.Row{{Key: "US:AAA", Symbol: "AAA", Market: "US", Currency: "USD"}}

	unknown := baseInput(newFake())
	unknown.Company = "US:NOPE"
	needMarket := formInput(newFake(), "XX:ZZZ", Typed{})
	needMarket.Underlying = nil

	cases := []struct {
		name  string
		build func(context.Context, Input) ([]byte, error)
		in    Input
		want  string // gap reason or problem field
	}{
		{"table close", BuildTable, withClose(tableIn), "close_unavailable"},
		{"table tables", BuildTable, noTables(tableIn), "tables_missing"},
		{"table store", BuildTable, noStore(tableIn), "store_unavailable"},
		{"company close", BuildCompany, withClose(companyIn), "close_unavailable"},
		{"company tables", BuildCompany, noTables(companyIn), "tables_missing"},
		{"company store", BuildCompany, noStore(companyIn), "store_unavailable"},
		{"company unknown", BuildCompany, unknown, ""},
		{"form tables", BuildForm, noTables(formIn), "tables_missing"},
		{"form store", BuildForm, noStore(formIn), "store_unavailable"},
		{"form market", BuildForm, needMarket, "r_market"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, err := c.build(ctx, c.in)
			if err != nil {
				t.Fatal(err)
			}
			m := validate(t, b)
			var texts []string
			for _, g := range m["gaps"].([]any) {
				texts = append(texts, obj(g)["text"].(string))
			}
			for _, p := range m["problems"].([]any) {
				texts = append(texts, obj(p)["text"].(string))
			}
			if len(texts) == 0 {
				t.Fatalf("no gap or problem text in %v", m)
			}
			for _, s := range texts {
				if !hasHan(s) {
					t.Errorf("text %q holds no Chinese character", s)
				}
				for _, o := range old {
					if s == o {
						t.Errorf("text %q is an old English sentence", s)
					}
				}
			}
		})
	}
}
