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

var sideNames = map[string]string{"sold": "Sold", "bought": "Bought"}

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
		Lead:  heading + leadMid + alias + leadEnd,
		Scope: alias, FigureNotes: []string{}, EndingsTitle: "Endings",
		BooksTitle: "Reconciliation with the broker", BooksNote: booksNote,
	}
	kept := gapText
	if s.KeptShare != "" {
		p, err := percent(s.KeptShare)
		if err != nil {
			return out, err
		}
		kept = p
	} else {
		out.FigureNotes = append(out.FigureNotes, noKeptNote)
	}
	if s.Realized == "" || s.Premium == "" {
		out.FigureNotes = append(out.FigureNotes, noRealizedNote)
	}
	out.Figures = []pair{
		{"Realized", orGap(moneyText(s.Realized, s.Currency))}, {"Premium", orGap(moneyText(s.Premium, s.Currency))}, {"Retained ratio", kept},
		{"Closed trades", count(ends, "closed")}, {"Assigned trades", count(ends, "assigned")}, {"Expired trades", count(ends, "expired")},
	}
	var err error
	if out.Months, out.Running, err = monthSeries(s.Months, s.Currency); err != nil {
		return out, err
	}
	if out.Endings, out.EndingsLines, err = endingSeries(s.Endings, s.Currency); err != nil {
		return out, err
	}
	if count(ends, "assigned") != "0" {
		prem := orGap(moneyText(s.AssignedPremium, s.Currency))
		t := assignPremium + prem + assignShares + orGap(s.AssignedShares) + assignEnd
		out.Assignment = &t
	}
	r := s.Reliability
	out.BooksLines = []string{
		sameBooks + itoa(r.Equal) + tradeUnit + differBooks + itoa(r.Differ) + tradeUnit,
		noBrokerBooks + itoa(r.NoBroker) + tradeUnit,
	}
	return out, nil
}
