package companies_page

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	diagnosisBasisNote = "Context for cyclical and financial names; not a band."
	questionsBasisNote = "Q1 to Q4 from filings; the fifth is the owner's text."
	notEnteredText     = "not entered"
	notFormedText      = "not formed"
)

type figure struct {
	Value num    `json:"value"`
	Text  string `json:"text"`
	Unit  string `json:"unit"`
}

type questionRow struct {
	Symbol string `json:"symbol"`
	Q1     any    `json:"q1"`
	Q2     any    `json:"q2"`
	Q3     any    `json:"q3"`
	Q4     any    `json:"q4"`
	Fifth  any    `json:"fifth"`
}

type questionsBlock struct {
	BasisNote string        `json:"basis_note"`
	Rows      []questionRow `json:"rows"`
}

type yearRow struct {
	FY         string `json:"fy"`
	Revenue    string `json:"revenue"`
	Margin     string `json:"margin"`
	Extra      string `json:"extra"`
	BarPercent num    `json:"bar_percent"`
}

type diagRow struct {
	Symbol   string    `json:"symbol"`
	LatestFY string    `json:"latest_fy"`
	Latest   string    `json:"latest"`
	RankText string    `json:"rank_text"`
	Extra    string    `json:"extra"`
	Usual    string    `json:"usual"`
	Years    []yearRow `json:"years"`
}

type diagGroup struct {
	Key     string   `json:"key"`
	Title   string   `json:"title"`
	Columns []string `json:"columns"`
	Rows    []any    `json:"rows"`
	Legend  []string `json:"legend"`
}

type diagnosisBlock struct {
	BasisNote string      `json:"basis_note"`
	Groups    []diagGroup `json:"groups"`
}

// groupSpec names the figures and the words of one diagnosis group.
type groupSpec struct {
	key, title, primaryColumn, legend string
	primary, secondary, scale         string
	secondaryLatest                   string
	extraLabel                        func(pct string) string
}

var groupSpecs = []groupSpec{
	{
		key: "cyclical", title: "Cyclical", primaryColumn: "Margin", legend: "Margin is operating margin.",
		primary: "operating_margin", secondary: "inventory_to_revenue", scale: "revenue",
		secondaryLatest: "inventory_to_revenue_latest",
		extraLabel:      func(p string) string { return "inventory " + p + " of revenue" },
	},
	{
		key: "financial", title: "Financial", primaryColumn: "ROE",
		legend:  "ROE is net income over the mean of the opening and closing equity.",
		primary: "roe", secondary: "roa", scale: "equity", secondaryLatest: "roa_latest",
		extraLabel: func(p string) string { return "ROA " + p },
	},
}

// questionUnits is the unit of each question: a percent or a ratio.
var questionUnits = [4]string{"percent", "ratio", "percent", "ratio"}

// diagnosisBlockOf builds the diagnosis block from the figures of Diagnosis.
func (c Config) diagnosisBlockOf(figs []subjectFigs) any {
	if c.Diagnosis == nil {
		return blockGap(reasonNoReader, "diagnosis: function not set")
	}
	blk := diagnosisBlock{BasisNote: diagnosisBasisNote, Groups: []diagGroup{}}
	for _, spec := range groupSpecs {
		g := diagGroup{
			Key: spec.key, Title: spec.title, Columns: []string{"Company", "Latest FY", spec.primaryColumn, "Rank"},
			Rows: []any{}, Legend: []string{spec.legend},
		}
		for _, f := range figs {
			if f.diag.Group == spec.key {
				g.Rows = append(g.Rows, diagRowOf(f.sub.Symbol, spec, f.diag))
			}
		}
		if len(g.Rows) > 0 {
			blk.Groups = append(blk.Groups, g)
		}
	}
	return blk
}

// diagRowOf is the row of one company, or a gap naming the figure that was not formed.
func diagRowOf(symbol string, spec groupSpec, d valuationapi.DiagnosisResult) any {
	byKey := make(map[string]valuationapi.DiagnosisFigure, len(d.Figures))
	for _, f := range d.Figures {
		byKey[f.Key] = f
	}
	val := func(key string) (decimal.Decimal, string) {
		f, ok := byKey[key]
		if !ok {
			return decimal.Zero, symbol + ": " + key + " not given"
		}
		if f.State != valuationapi.StateOK {
			return decimal.Zero, symbol + ": " + reasonText(f.Reason)
		}
		v, ok := parseDec(f.Value)
		if !ok {
			return decimal.Zero, symbol + ": " + key + " is not a number"
		}
		return v, ""
	}
	ends := sortedEnds(byKey, spec.primary)
	if len(ends) == 0 {
		return blockGap(reasonMissingIn, symbol+": no fiscal years")
	}
	latest, why := val(spec.primary + "@" + ends[0])
	if why != "" {
		return blockGap(reasonMissingIn, why)
	}
	rank, why := val("latest_rank")
	if why != "" {
		return blockGap(reasonMissingIn, why)
	}
	ranked, why := val("ranked")
	if why != "" {
		return blockGap(reasonMissingIn, why)
	}
	low, why := val(spec.primary + "_usual_low")
	if why != "" {
		return blockGap(reasonMissingIn, why)
	}
	high, why := val(spec.primary + "_usual_high")
	if why != "" {
		return blockGap(reasonMissingIn, why)
	}
	extra := notFormedText
	if s, why := val(spec.secondaryLatest); why == "" {
		extra = spec.extraLabel(pct(s, 0) + "%")
	}
	row := diagRow{
		Symbol: symbol, LatestFY: fyLabel(ends[0]), Latest: pct(latest, 1) + "%",
		RankText: fmt.Sprintf("%s of %s years", rank.String(), ranked.String()),
		Extra:    extra, Usual: pct(low, 1) + "% to " + pct(high, 1) + "%", Years: []yearRow{},
	}
	for _, e := range ends {
		p, why1 := val(spec.primary + "@" + e)
		s, why2 := val(spec.scale + "@" + e)
		if why1 != "" || why2 != "" {
			continue
		}
		sec := notFormedText
		if v, why := val(spec.secondary + "@" + e); why == "" {
			sec = pct(v, 0) + "%"
		}
		row.Years = append(row.Years, yearRow{
			FY: fyLabel(e), Revenue: grouped(s), Margin: pct(p, 1) + "%", Extra: sec,
			BarPercent: marker(p.Mul(hundred), diagnosisBarMax),
		})
	}
	return row
}

// questionsBlockOf builds the questions block: Q1 to Q4 from Questions, the fifth from the owner.
func (c Config) questionsBlockOf(ctx context.Context, figs []subjectFigs, today contracts.Day) (any, error) {
	if c.Questions == nil {
		return blockGap(reasonNoReader, "questions: function not set"), nil
	}
	blk := questionsBlock{BasisNote: questionsBasisNote, Rows: []questionRow{}}
	for _, f := range figs {
		fifth, err := c.fifth(ctx, f.sub.Symbol)
		if err != nil {
			return nil, err
		}
		row := questionRow{Symbol: f.sub.Symbol, Fifth: fifth}
		var cells [4]any
		switch {
		case f.refused != "":
			for i := range cells {
				cells[i] = blockGap(reasonMissingIn, f.refused)
			}
		case !f.hasYears():
			for i := range cells {
				cells[i] = blockGap(reasonMissingIn, reasonText(f.fr.Reasons...))
			}
		default:
			cells = questionCells(c.Questions(f.fr.Filings, today))
		}
		row.Q1, row.Q2, row.Q3, row.Q4 = cells[0], cells[1], cells[2], cells[3]
		blk.Rows = append(blk.Rows, row)
	}
	return blk, nil
}

// questionCells is the figure or gap of each of Q1 to Q4.
func questionCells(res valuationapi.QuestionsResult) [4]any {
	var out [4]any
	for i := range out {
		key := fmt.Sprintf("Q%d", i+1)
		out[i] = blockGap(reasonMissingIn, key+" not given")
		for _, q := range res.Questions {
			if q.Key != key {
				continue
			}
			out[i] = questionCell(q, questionUnits[i])
		}
	}
	return out
}

func questionCell(q valuationapi.Question, unit string) any {
	if q.State != valuationapi.StateOK {
		return blockGap(reasonMissingIn, reasonText(q.Reason))
	}
	v, ok := parseDec(q.Value)
	if !ok {
		return blockGap(reasonMissingIn, q.Key+" is not a number")
	}
	if unit == "percent" {
		t := pct(v, 1)
		return figure{Value: num(t), Text: t + "%", Unit: unit}
	}
	t := v.RoundBank(2).StringFixed(2)
	return figure{Value: num(t), Text: t, Unit: unit}
}

// fifth is the owner's text of the fifth question, or the not-entered gap.
func (c Config) fifth(ctx context.Context, symbol string) (any, error) {
	if c.Text == nil {
		return blockGap(reasonNoData, notEnteredText), nil
	}
	text, ok, err := c.Text.FifthQuestion(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("companies_page: fifth question of %s: %w", symbol, err)
	}
	if !ok || strings.TrimSpace(text) == "" {
		return blockGap(reasonNoData, notEnteredText), nil
	}
	return text, nil
}
