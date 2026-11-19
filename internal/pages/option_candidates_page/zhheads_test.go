package option_candidates_page

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestSubtitleAndHeads(t *testing.T) {
	t.Parallel()
	const subtitle = "先卖出看跌期权（short put）的候选合约，按公司分组；只有你设了买入价的公司才会被评判。"
	c, _ := tierScenario("tiers_ok")
	b := buildBytes(t, c, buildDay)
	if !strings.Contains(string(b), `"subtitle":"`+subtitle+`"`) {
		t.Errorf("the page has no subtitle: %s", b)
	}
	v := buildTiersOf(t, c)
	if v.Heads == nil {
		t.Fatal("the tier view has no heads")
	}
	want := tierHeads{Companies: "公司", CompaniesNote: "每家公司两行：上行看价格，下行看估值和合约数量", Tiers: "合约分档"}
	if *v.Heads != want {
		t.Errorf("heads = %+v", *v.Heads)
	}

	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	nb := buildBytes(t, tierConfig(rows, nil), buildDay)
	if strings.Contains(string(nb), `"heads"`) {
		t.Errorf("a page with no tier view has heads: %s", nb)
	}
	if !strings.Contains(string(nb), `"subtitle":"`+subtitle+`"`) {
		t.Errorf("a page with no tier view has no subtitle")
	}
}
