package household_page

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// maxFieldRunes is the schema's limit of a note, a gap text and a lead note.
const maxFieldRunes = 200

// riskBlocks are the four family risk blocks, the lead notes and the page gap lines that go with them.
type riskBlocks struct {
	obligations, cashAfter, stress, concentration moneyBlock
	leadNotes                                     []string
	gaps                                          []gapJSON
}

// riskItem is one row of a block before it is laid out; amount is the figure of the row.
type riskItem struct {
	name, scope, note string
	amount            compute.FigureValue
	share             *shareFigure
}

// familyRisk reads the open short puts and the stock lines of every counted account, calls the figure and lays
// its text out as the four blocks. It is nil when Config.FamilyRisk is nil. The first alias failing to read is
// an error; another alias failing is left out of the sums and named in the gap of each block. Config.Marks or
// Config.Holdings nil makes only the concentration the gap no_reader.
func (c Config) familyRisk(ctx context.Context, aliases []string, res blockapi.FamilyResult, caps map[string]capture,
	sessions []contracts.Day) (*riskBlocks, error) {
	if c.FamilyRisk == nil {
		return nil, nil
	}
	if len(res.Included) == 0 {
		g := gapJSON{Reason: "no_data", Text: riskGapNoAccts}
		return &riskBlocks{
			obligations: gapBlock(riskTitleObligations, riskMeaningObligations, g), cashAfter: gapBlock(riskTitleCashAfter, riskMeaningCashAfter, g),
			stress: gapBlock(riskTitleStress, riskMeaningStress, g), concentration: gapBlock(riskTitleConcentration, riskMeaningConc, g),
			leadNotes: c.notCountedLine(res),
		}, nil
	}
	in := blockapi.FamilyRiskIn{Net: res.Net, UsdJpy: res.FxRate}
	cash := map[string]compute.FigureValue{}
	for _, r := range res.Rows {
		cash[r.Alias] = r.Cash
	}
	haveMarks := c.Marks != nil && c.Holdings != nil
	rate := c.lazyYenRate(ctx, res, caps)
	var unread []string
	for i, alias := range aliases {
		if !slices.Contains(res.Included, alias) {
			continue
		}
		a := blockapi.FamilyRiskAccountIn{Alias: alias, Cash: cash[alias], Read: true}
		var lines []blockapi.HeldLineIn
		var err error
		a.Obligations, _, _, err = c.obligations(ctx, alias, sessions, contracts.Day(caps[alias].day))
		if err == nil && haveMarks {
			lines, err = c.heldLines(ctx, alias, caps[alias].day, rate)
		}
		switch {
		case err != nil && i == 0:
			return nil, err
		case err != nil:
			a.Read, a.Obligations = false, nil
			unread = append(unread, familyAccountLabel(alias))
		default:
			in.Stocks = append(in.Stocks, lines...)
		}
		in.Accounts = append(in.Accounts, a)
	}
	out := c.FamilyRisk().FamilyRisk(in)
	return layoutRisk(res, out, unread, haveMarks, c.notCountedLine(res)), nil
}

// notCountedLine is the lead note that counts the accounts of the book the blocks do not hold.
func (Config) notCountedLine(res blockapi.FamilyResult) []string {
	if len(res.Excluded) == 0 {
		return nil
	}
	return []string{fmt.Sprintf(riskLineNotCounted, len(res.Excluded))}
}

func usdCell(f compute.FigureValue) (moneyCell, bool) {
	return cell(blockapi.MoneyLine{Amount: f, Currency: currencyUSD})
}

func clip(s string) string {
	if r := []rune(s); len(r) > maxFieldRunes {
		return string(r[:maxFieldRunes-1]) + "…"
	}
	return s
}

// riskGapReason is the reason the figure gave, one the schema knows.
func riskGapReason(f compute.FigureValue) string {
	if f.Gap != "" {
		return f.Gap
	}
	return "missing_input"
}

// riskBlock lays out items as rows and total as the one total cell. An item or total with no cell is left out
// and named in the gap with the reason of the first; extra texts (the unread accounts) join the gap text.
func riskBlock(title, meaning string, items []riskItem, total *compute.FigureValue, unread []string) moneyBlock {
	b := moneyBlock{Title: title, Meaning: meaning, Rows: []moneyRow{}, Totals: []moneyCell{}}
	var parts []string
	reason := ""
	for _, it := range items {
		c, ok := usdCell(it.amount)
		if !ok {
			if reason == "" {
				reason = riskGapReason(it.amount)
			}
			parts = append(parts, fmt.Sprintf(riskGapRow, it.name))
			continue
		}
		b.Rows = append(b.Rows, moneyRow{Name: it.name, Amount: c, Note: clip(it.note), Share: it.share, Scope: it.scope})
	}
	if total != nil {
		if c, ok := usdCell(*total); ok {
			b.Totals = append(b.Totals, c)
		} else {
			if reason == "" {
				reason = riskGapReason(*total)
			}
			parts = append(parts, riskGapTotal)
		}
	}
	if len(unread) > 0 {
		if reason == "" {
			reason = "missing_input"
		}
		parts = append(parts, fmt.Sprintf(riskGapUnread, strings.Join(unread, riskNoteList)))
	}
	if reason != "" {
		b.Gap = &gapJSON{Reason: reason, Text: clip(strings.Join(parts, riskGapSeparate))}
	}
	return b
}

// layoutRisk copies the text of the figure's result into the four blocks.
func layoutRisk(res blockapi.FamilyResult, out blockapi.FamilyRiskResult, unread []string, haveMarks bool, lead []string) *riskBlocks {
	r := &riskBlocks{leadNotes: lead}

	var puts []riskItem
	for _, p := range out.Puts {
		labels := make([]string, 0, len(p.Aliases))
		for _, a := range p.Aliases {
			labels = append(labels, familyAccountLabel(a))
		}
		note := strings.Join(labels, riskNoteList)
		if _, ok := usdCell(compute.FigureValue{Value: p.Premium}); ok && p.Premium != "" {
			note += riskNoteAccounts + moneyText("$", p.Premium)
		}
		puts = append(puts, riskItem{name: p.Underlying + riskNameSuffix, amount: p.Cash, note: note})
	}
	r.obligations = riskBlock(riskTitleObligations, riskMeaningObligations, puts, &out.NeedTotal, unread)

	var accts []riskItem
	for _, a := range out.Accounts {
		it := riskItem{name: familyAccountLabel(a.Alias), scope: a.Alias, amount: a.CashAfter}
		cashC, cok := usdCell(a.Cash)
		needC, nok := usdCell(a.Need)
		if cok && nok {
			it.note = fmt.Sprintf(riskNoteCash, cashC.Text, needC.Text)
		}
		if a.Short {
			it.note += riskNoteShort
		}
		accts = append(accts, it)
	}
	r.cashAfter = riskBlock(riskTitleCashAfter, riskMeaningCashAfter, accts, &out.AfterTotal, unread)
	if out.ShortCount > 0 {
		r.leadNotes = append(r.leadNotes, fmt.Sprintf(riskLineShort, out.ShortCount))
	}

	stress := []riskItem{
		{name: riskStressNeed, amount: out.NeedTotal, share: shareOf(out.NeedShare)},
		{name: riskStressCash, amount: out.CashTotal, share: shareOf(res.CashShare)},
		{name: riskStressAfter, amount: out.AfterTotal, share: shareOf(out.AfterShare)},
	}
	r.stress = riskBlock(riskTitleStress, riskMeaningStress, stress, nil, unread)
	r.gaps = append(r.gaps, gapJSON{Reason: "missing_quote", Text: riskStressGap})

	if !haveMarks {
		r.concentration = gapBlock(riskTitleConcentration, riskMeaningConc, gapJSON{Reason: "no_reader", Text: riskGapNoMarks})
	} else {
		var cos []riskItem
		for _, co := range out.Companies {
			it := riskItem{name: co.Underlying, scope: familyAliasKey, amount: co.Exposure, share: shareOf(co.Share)}
			stockC, sok := usdCell(co.Stock)
			putC, pok := usdCell(co.Put)
			if sok && pok {
				it.note = fmt.Sprintf(riskNoteStock, stockC.Text, putC.Text)
			}
			cos = append(cos, it)
		}
		r.concentration = riskBlock(riskTitleConcentration, riskMeaningConc, cos, nil, unread)
	}
	if out.Hidden > 0 {
		r.leadNotes = append(r.leadNotes, fmt.Sprintf(riskLineHidden, out.Hidden))
	}
	return r
}

// setRisk replaces the four blocks of p and the page gap lines that named them not_built; a block that is
// itself a gap names that gap instead.
func (p *pageJSON) setRisk(r riskBlocks) {
	old := []string{titleObligations, titleCashAfter, titleStress, titleConcentration}
	p.Gaps = slices.DeleteFunc(p.Gaps, func(g gapJSON) bool {
		return g.Reason == "not_built" && slices.ContainsFunc(old, func(t string) bool { return strings.HasPrefix(g.Text, t+": ") })
	})
	p.Obligations, p.CashAfter, p.Stress, p.Concentration = r.obligations, r.cashAfter, r.stress, r.concentration
	for _, b := range []moneyBlock{r.obligations, r.cashAfter, r.stress, r.concentration} {
		if b.Gap != nil {
			p.Gaps = append(p.Gaps, gapJSON{Reason: b.Gap.Reason, Text: clip(b.Title + ": " + b.Gap.Text)})
		}
	}
	p.Gaps = append(p.Gaps, r.gaps...)
	p.LeadNotes = append(p.LeadNotes, r.leadNotes...)
}
