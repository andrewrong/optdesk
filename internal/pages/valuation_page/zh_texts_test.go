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
		want  string // gap reason, or problem field
		text  string // the exact text of it
	}{
		{"table close", BuildTable, withClose(tableIn), "close_unavailable", "有些公司读不到收盘价，这些公司的价格区间缺失。"},
		{"table tables", BuildTable, noTables(tableIn), "tables_missing", "没有存放 R 版本或保证金表，所以不显示价格线。"},
		{"table store", BuildTable, noStore(tableIn), "store_unavailable", "这里读不到估值存档。"},
		{"company close", BuildCompany, withClose(companyIn), "close_unavailable", "读不到收盘价。"},
		{"company tables", BuildCompany, noTables(companyIn), "tables_missing", "没有存放 R 版本或保证金表，所以不显示价格线。"},
		{"company store", BuildCompany, noStore(companyIn), "store_unavailable", "这里读不到估值存档。"},
		{"company unknown", BuildCompany, unknown, "store_unavailable", "没有这个键对应的公司。"},
		{"form tables", BuildForm, noTables(formIn), "tables_missing", "没有存放 R 版本或保证金表，所以不显示价格线。"},
		{"form store", BuildForm, noStore(formIn), "store_unavailable", "这里读不到估值存档。"},
		{"form market", BuildForm, needMarket, "r_market", "请选择这家公司的 R 市场。"},
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
			found := false
			for _, g := range m["gaps"].([]any) {
				texts = append(texts, obj(g)["text"].(string))
				if obj(g)["reason"] == c.want {
					found = true
					if obj(g)["text"] != c.text {
						t.Errorf("gap %q text = %q, want %q", c.want, obj(g)["text"], c.text)
					}
				}
			}
			for _, p := range m["problems"].([]any) {
				texts = append(texts, obj(p)["text"].(string))
				if obj(p)["field"] == c.want {
					found = true
					if obj(p)["text"] != c.text {
						t.Errorf("problem %q text = %q, want %q", c.want, obj(p)["text"], c.text)
					}
				}
			}
			if !found {
				t.Fatalf("no gap or problem %q in %v", c.want, m)
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
