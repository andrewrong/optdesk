package household_page

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	familyAliasKey = "household" // the key of the family view; never an account
	currencyUSD    = "USD"
)

func familyAccountLabel(alias string) string { return familyLabelOpen + alias + familyLabelClose }

// BuildFamily writes the family page for day: the latest capture of every alias of c.Aliases goes to the
// FamilyFigure and its text is copied into the page. An account the total does not hold is a row with
// gap words and a line of lead_notes. The first alias is the main account: its reader error, like that of
// the session days, is an error; any other alias that fails is left out as read_error. A book of one
// alias gets the page of Build with the alias "".
func BuildFamily(ctx context.Context, c Config, day contracts.Day, at time.Time) ([]byte, error) {
	distinct := distinctAliases(c.Aliases)
	if len(distinct) <= 1 {
		return Build(ctx, c, "", day, at)
	}
	if c.Zone == nil {
		return nil, errors.New("household_page: no zone")
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("household_page: bad day %q: %w", day, err)
	}
	at = at.In(c.Zone)
	scopes := familyScopes(distinct)
	if c.Account == nil || c.Ledger == nil || c.Sessions == nil || c.Ledgers == nil || c.Figures == nil || c.Family == nil {
		p := gapPage(c, familyAliasKey, scopes, at, "no_reader", textNoReader)
		return marshal(p)
	}
	sessions, err := c.sessionDays(ctx, d)
	if err != nil {
		return nil, err
	}
	newest := ""
	if len(sessions) > 0 {
		newest = string(sessions[len(sessions)-1])
	}
	in, caps, err := c.readBook(ctx, distinct, sessions)
	if err != nil {
		return nil, err
	}
	res := c.Family().Family(in)
	trend, trendNotes := c.familyTrend(ctx, res, caps, sessions)
	return marshal(c.assembleFamily(distinct, res, caps, newest, at, trend, trendNotes))
}

// readBook reads the capture of every alias and the rate when a capture holds yen; it is the input of the
// family figure and the captures that are counted.
func (c Config) readBook(ctx context.Context, distinct []string, sessions []contracts.Day) (blockapi.FamilyIn, map[string]capture, error) {
	in := blockapi.FamilyIn{Accounts: make([]blockapi.FamilyAccountIn, 0, len(distinct))}
	caps := map[string]capture{}
	var counted []string
	jpy := false
	for i, alias := range distinct {
		a := blockapi.FamilyAccountIn{Alias: alias, Whose: whoseOf(i)}
		if alias == familyAliasKey {
			a.Reason = "read_error"
			in.Accounts = append(in.Accounts, a)
			continue
		}
		snap, err := c.capture(ctx, alias, sessions)
		switch {
		case err != nil && i == 0:
			return in, nil, err
		case err != nil:
			a.Reason = "read_error"
		case snap.day == "":
			a.Reason = "no_data"
		case len(sessions)-1-slices.Index(sessions, contracts.Day(snap.day)) > staleSessions:
			a.Reason, a.Day = "stale_input", snap.day
		default:
			a.Day, a.Values = snap.day, valuesIn(snap.values)
			caps[alias] = snap
			counted = append(counted, snap.day)
			for _, v := range snap.values {
				jpy = jpy || v.Currency == currencyJPY
			}
		}
		in.Accounts = append(in.Accounts, a)
	}
	if jpy && c.Fx != nil {
		latest := slices.Max(counted)
		if r, found, err := c.Fx.LatestFxRate(ctx, pairUsdJpy, contracts.Day(latest)); err == nil && found {
			in.UsdJpy, in.RateDay, in.RateSource = r.Rate, r.RateDay.Format(dayLayout), string(r.Source)
		}
	}
	return in, caps, nil
}

// distinctAliases is the book without repeats, in order: the one list of accounts of every page.
func distinctAliases(aliases []string) []string {
	var out []string
	for _, a := range aliases {
		if !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}

func whoseOf(i int) string {
	if i == 0 {
		return familyWhoseMain
	}
	return familyWhoseOther
}

func familyScopes(aliases []string) []accountScope {
	tags := make([]accountTag, 0, len(aliases))
	for i, a := range aliases {
		tags = append(tags, accountTag{Label: familyAccountLabel(a), Whose: whoseOf(i)})
	}
	return []accountScope{{Key: familyAliasKey, Label: familyTitle, Accounts: tags}}
}

// cellWords are the words of a row that has no cell, by the reason it is left out.
func cellWords(reason string) string {
	switch reason {
	case "no_data":
		return familyCellNoData
	case "stale_input":
		return familyCellStale
	case "unsupported_currency":
		return familyCellNoRate
	case "read_error":
		return familyCellError
	}
	return familyCellMissing
}

// assembleFamily writes the page of a family result. Every figure is copied from res.
func (c Config) assembleFamily(aliases []string, res blockapi.FamilyResult, caps map[string]capture, newest string, at time.Time, trend *trendBlock, trendNotes []string) pageJSON {
	p := base(c, familyScopes(aliases), familyAliasKey, at)
	p.RunStatus = "partial"
	p.Title, p.Sentence = familyTitle, familySentence
	p.Overview = c.familyOverview(aliases, res)

	rows := map[string]blockapi.FamilyAccountOut{}
	for _, r := range res.Rows {
		rows[r.Alias] = r
	}
	reasons := map[string]string{}
	for _, x := range res.Excluded {
		reasons[x.Alias] = x.Reason
	}
	acc := &accountsBlock{Title: familyAccountsTitle, Meaning: familyAccountsMeaning, Rows: []accountRow{},
		Columns: []string{familyHeadName, familyHeadNet, familyHeadCash, familyHeadStock, familyHeadOptions, familyHeadDay}}
	detail := accountFigure{Gap: familyCellDetail}
	for i, a := range aliases {
		r, found := rows[a]
		row := accountRow{Name: familyAccountLabel(a), Scope: a, Whose: whoseOf(i), Stock: detail, Options: detail}
		reason := r.Reason
		if reason == "" {
			reason = reasons[a]
		}
		if !found && reason == "" {
			reason = "no_data"
		}
		if reason != "" {
			words := accountFigure{Gap: cellWords(reason)}
			row.NetLiquidation, row.Cash = words, words
		} else {
			row.NetLiquidation, row.Cash = familyFigureOf(r.Net), familyFigureOf(r.Cash)
			row.DataDay = r.Day
		}
		acc.Rows = append(acc.Rows, row)
	}
	p.Accounts = acc

	lines := familyNotesAndGaps(&p, res, newest)
	p.Gaps = append(lines, p.Gaps...)
	detailGaps(&p)
	if trend != nil {
		p.setTrend(*trend)
	}
	p.LeadNotes = append(p.LeadNotes, trendNotes...)
	p.Notes = append(p.Notes, familyCashNote)
	var oldestAt time.Time
	for _, alias := range res.Included {
		cp := caps[alias]
		if len(cp.values) == 0 || cp.values[0].CapturedAt.IsZero() {
			continue
		}
		t := cp.values[0].CapturedAt.In(c.Zone)
		p.DataTimes = append(p.DataTimes, sourceTime{Source: "account", DataTime: t.Format(timeLayout)})
		if oldestAt.IsZero() || t.Before(oldestAt) {
			oldestAt = t
		}
		if slices.Contains(c.Statement, alias) {
			p.Notes = append(p.Notes, textStatementFrom+cp.day+textStatementNote)
		}
	}
	if !oldestAt.IsZero() {
		p.Stale = staleLabel{DataTime: oldestAt.Format(timeLayout), StaleAfter: oldestAt.AddDate(0, 0, staleAfterDays).Format(timeLayout),
			Text: familyStaleText + res.OldestDay}
	}
	return p
}

// familyOverview is the block with the one total cell and the one row that counts the accounts in it; the
// row's note carries the cash and its share of the net when the figure gives them.
func (c Config) familyOverview(aliases []string, res blockapi.FamilyResult) moneyBlock {
	b := moneyBlock{Title: familyTitle, Meaning: familySentence, Rows: []moneyRow{}, Totals: []moneyCell{}}
	net, ok := cell(blockapi.MoneyLine{Amount: res.Net, Currency: currencyUSD})
	if !ok {
		reason := lineGap(blockapi.MoneyLine{Amount: res.Net, Currency: currencyUSD})
		b.Gap = &gapJSON{Reason: reason, Text: gapText(reason)}
		return b
	}
	row := moneyRow{Name: fmt.Sprintf(familyNetName, len(res.Included), len(aliases)), Amount: net}
	if cash, ok := cell(blockapi.MoneyLine{Amount: res.Cash, Currency: currencyUSD}); ok && res.CashShare.Gap == "" && shareText.MatchString(res.CashShare.Value) {
		row.Note = fmt.Sprintf(familyCashRow, cash.Text, shareLabel(res.CashShare.Value))
	}
	b.Rows = append(b.Rows, row)
	b.Totals = append(b.Totals, net)
	return b
}

// familyFigureOf is the cell of a USD figure, or the words of its gap.
func familyFigureOf(f compute.FigureValue) accountFigure {
	if c, ok := cell(blockapi.MoneyLine{Amount: f, Currency: currencyUSD}); ok {
		return accountFigure{Cell: &c}
	}
	return accountFigure{Gap: familyCellMissing}
}

// familyNotesAndGaps writes the lead_notes of p and returns the page gaps of the accounts left out.
func familyNotesAndGaps(p *pageJSON, res blockapi.FamilyResult, newest string) []gapJSON {
	var lines []gapJSON
	for _, x := range res.Excluded {
		who := familyAccountLabel(x.Alias)
		var text string
		switch {
		case x.Reason == "stale_input" && x.Day != "":
			text = fmt.Sprintf(familyLineStale, who, x.Day)
		case x.Reason == "read_error":
			text = fmt.Sprintf(familyLineReadError, who)
		case x.Reason == "unsupported_currency":
			text = fmt.Sprintf(familyLineCurrency, who)
		default:
			text = fmt.Sprintf(familyLineNoData, who)
		}
		p.LeadNotes = append(p.LeadNotes, text)
		reason := x.Reason
		if reason == "read_error" {
			reason = "no_data"
		}
		lines = append(lines, gapJSON{Reason: reason, Text: text})
	}
	for _, r := range res.Rows {
		if r.Reason == "" && r.Day != "" && r.Day != newest && slices.Contains(res.Included, r.Alias) {
			p.LeadNotes = append(p.LeadNotes, fmt.Sprintf(familyLineEarlierDay, familyAccountLabel(r.Alias), r.Day))
		}
	}
	if res.FxRate != "" {
		p.LeadNotes = append(p.LeadNotes, fmt.Sprintf(familyRateLine, res.FxRate, res.FxDay, res.FxSource))
	}

	return lines
}

// detailGaps makes the ten detail blocks of p the gap not_built and names each in the page gaps.
func detailGaps(p *pageJSON) {
	g := gapJSON{Reason: "not_built", Text: familyDetailGap}
	p.Composition = gapBlock(titleComposition, meaningComposition, g)
	p.Concentration = gapBlock(titleConcentration, meaningConcentration, g)
	p.Stress = gapBlock(titleStress, meaningStress, g)
	p.Income = gapBlock(titleIncome, meaningIncome, g)
	p.CashAfter = gapBlock(titleCashAfter, meaningCashAfter, g)
	p.Obligations = gapBlock(titleObligations, meaningObligations, g)
	p.Allocation = allocationGap(g)
	p.Movers = moversGap(g)
	p.Trend = trendBlock{Title: titleTrend, Meaning: meaningTrend, Rows: []trendPoint{}, Series: []trendSeries{}, Gap: gapOf(g)}
	p.MonthChange = waterfallBlock{Title: titleMonthChange, Meaning: meaningMonthChange, Rows: []waterfallStep{}, Gap: gapOf(g)}
	for _, title := range []string{titleComposition, titleMovers, titleTrend, titleStress, titleIncome,
		titleMonthChange, titleCashAfter, titleObligations, titleConcentration, titleAllocation} {
		p.Gaps = append(p.Gaps, gapJSON{Reason: g.Reason, Text: title + ": " + g.Text})
	}

}

// setTrend replaces the trend block of p and the page gap line that named it not_built; a block that is
// itself a gap names that gap instead.
func (p *pageJSON) setTrend(t trendBlock) {
	p.Trend = t
	p.Gaps = slices.DeleteFunc(p.Gaps, func(g gapJSON) bool { return g.Reason == "not_built" && strings.HasPrefix(g.Text, titleTrend+": ") })
	if t.Gap != nil {
		p.Gaps = append(p.Gaps, gapJSON{Reason: t.Gap.Reason, Text: titleTrend + ": " + t.Gap.Text})
	}
}
