package option_candidates_page

import (
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestTierCompaniesSumToRows(t *testing.T) {
	t.Parallel()
	c, _ := tierScenario("tiers_ok")
	view := buildTiersOf(t, c)
	const rowsRead = 5 // the tiers_ok scenario stores five rows
	total, shown := 0, 0
	for _, tr := range view.Tiers {
		sum := 0
		var ids []string
		for _, co := range tr.Companies {
			sum += co.Count
			if co.Count != len(co.RowIDs) {
				t.Errorf("%s %s: count %d but %d ids", tr.ID, co.Company, co.Count, len(co.RowIDs))
			}
			ids = append(ids, co.RowIDs...)
		}
		if sum != len(tr.Rows) {
			t.Errorf("%s: companies sum %d, rows %d", tr.ID, sum, len(tr.Rows))
		}
		total += sum
		shown += len(tr.Rows)
		for i, r := range tr.Rows {
			if !slices.Contains(ids, r.ID) {
				t.Errorf("%s: row %d (%s) in no company", tr.ID, i, r.ID)
			}
		}
		if tr.Companies == nil {
			t.Errorf("%s: companies is nil", tr.ID)
		}
	}
	if total != shown || total != rowsRead {
		t.Errorf("tiers sum %d, shown %d, rows read %d", total, shown, rowsRead)
	}
	bbb := tierByID(view, "grey_no_buy_price")
	if len(bbb.Companies) != 1 || bbb.Companies[0].Company != "BBB" {
		t.Errorf("grey_no_buy_price companies = %+v", bbb.Companies)
	}
}

func TestTierCompaniesFirstRowOrder(t *testing.T) {
	t.Parallel()
	mk := func(id int, co int, issuer string) store.Candidate {
		r := tierPut(id, "ok", "85.00", "0.42")
		r.UnderlyingID, r.Issuer = co, issuer
		return r
	}
	rows := []store.Candidate{mk(1, 7, "ZZZ"), mk(2, 3, "AAA"), mk(3, 7, "ZZZ")}
	res := resultOf(fitsVerdict(keyOf(rows, 0)), fitsVerdict(keyOf(rows, 1)), fitsVerdict(keyOf(rows, 2)))
	c := tierConfig(rows, &fakeTierer{res: res})
	c.Names = &namesFake{names: map[int]string{7: "ZZZ", 3: "AAA"}}
	view := buildTiersOf(t, c)
	cs := tierByID(view, "fits").Companies
	if len(cs) != 2 || cs[0].Company != "ZZZ" || cs[0].Count != 2 || cs[1].Company != "AAA" || cs[1].Count != 1 {
		t.Errorf("companies = %+v", cs)
	}
}

func TestEarningsTextUnknown(t *testing.T) {
	t.Parallel()
	mk := func(id, co int, issuer string, next *contracts.Day) store.Candidate {
		r := tierPut(id, "ok", "85.00", "0.42")
		r.UnderlyingID, r.Issuer, r.NextEarnings = co, issuer, next
		return r
	}
	date, past := contracts.Day("2026-12-10"), contracts.Day("2026-11-01")
	rows := []store.Candidate{mk(1, 7, "AAA", &date), mk(2, 8, "BBB", nil), mk(3, 9, "CCC", &past)}
	res := resultOf(fitsVerdict(keyOf(rows, 0)), fitsVerdict(keyOf(rows, 1)), fitsVerdict(keyOf(rows, 2)))
	res.Companies = []optiontierapi.CompanyCount{{Company: "7", Fits: 1}, {Company: "8", Fits: 1}, {Company: "9", Fits: 1}}
	c := tierConfig(rows, &fakeTierer{res: res})
	c.Names = &namesFake{names: map[int]string{7: "AAA", 8: "BBB", 9: "CCC"}}
	cs := buildTiersOf(t, c).Companies
	const unknown = "来源暂无日期（未查到，不等于没有）"
	if len(cs) != 3 || cs[0].EarningsText != "2026-12-10" || cs[1].EarningsText != unknown || cs[2].EarningsText != unknown {
		t.Errorf("earnings texts = %q %q %q", cs[0].EarningsText, cs[1].EarningsText, cs[2].EarningsText)
	}
}
