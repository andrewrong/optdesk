package option_result_page

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

type pair struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type section struct {
	Key          string   `json:"key"`
	Heading      string   `json:"heading"`
	Lead         string   `json:"lead"`
	Scope        string   `json:"scope"`
	Figures      []pair   `json:"figures"`
	FigureNotes  []string `json:"figure_notes"`
	Months       series   `json:"months"`
	Running      series   `json:"running"`
	EndingsTitle string   `json:"endings_title"`
	EndingsLines []string `json:"endings_lines"`
	Endings      series   `json:"endings"`
	Assignment   *string  `json:"assignment"`
	Protection   *string  `json:"protection"`
	BooksTitle   string   `json:"books_title"`
	BooksNote    string   `json:"books_note"`
	BooksLines   []string `json:"books_lines"`
}

var sideNames = map[string]string{"sold": "卖出", "bought": "买入"}

// sectionOf is the section of one side and currency of the rule, in the shape of the schema.
func sectionOf(alias string, s compute.ResultSection) (section, error) {
	heading := sideNames[s.Side]
	if heading == "" {
		heading = s.Side
	}
	heading += " " + s.Currency
	ends := make([]ending, len(s.Endings))
	for i, e := range s.Endings {
		ends[i] = ending{status: e.Status, realized: e.Realized, count: e.Count}
	}
	out := section{
		Key: s.Side + "_" + strings.ToLower(s.Currency), Heading: heading,
		Lead:  heading + "的已平仓期权, 每个币种单独列出, 不相加",
		Scope: alias, FigureNotes: []string{}, EndingsTitle: "结束方式",
		BooksTitle: "与券商对账", BooksNote: "券商数字取自 Flex 的 fifoPnlRealized, 只对照, 不入任何数字",
	}
	kept := gapText
	if s.KeptShare != "" {
		p, err := percent(s.KeptShare)
		if err != nil {
			return out, err
		}
		kept = p
	} else {
		out.FigureNotes = append(out.FigureNotes, "留存比例没有值: 没有卖出开仓的权利金可除")
	}
	if s.Realized == "" || s.Premium == "" {
		out.FigureNotes = append(out.FigureNotes, "已实现或权利金没有值")
	}
	out.Figures = []pair{
		{"已实现", orGap(s.Realized)}, {"权利金", orGap(s.Premium)}, {"留存比例", kept},
		{"已平仓笔数", count(ends, "closed")}, {"被行权笔数", count(ends, "assigned")}, {"到期笔数", count(ends, "expired")},
	}
	var err error
	if out.Months, out.Running, err = monthSeries(s.Months); err != nil {
		return out, err
	}
	if out.Endings, out.EndingsLines, err = endingSeries(s.Endings); err != nil {
		return out, err
	}
	if count(ends, "assigned") != "0" {
		t := "被行权的权利金 " + orGap(s.AssignedPremium) + ", 股数 " + orGap(s.AssignedShares) + "; 成本仍是行权价, 不扣权利金"
		out.Assignment = &t
	} else {
		out.FigureNotes = append(out.FigureNotes, "被行权说明为空: 本段没有被行权的结束")
	}
	out.FigureNotes = append(out.FigureNotes, "保护说明为空: 本设计没有这个数值")
	r := s.Reliability
	out.BooksLines = []string{
		"一致 " + itoa(r.Equal) + " 笔, 不一致 " + itoa(r.Differ) + " 笔",
		"没有券商数字 " + itoa(r.NoBroker) + " 笔",
	}
	return out, nil
}
