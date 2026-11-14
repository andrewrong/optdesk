package household_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	calendarID         = "XNYS"
	dayLayout          = "2006-01-02"
	timeLayout         = "2006-01-02T15:04:05-07:00"
	sessionLookback    = 150 // calendar days of session days to read
	historyDays        = 90  // session days of the trend
	staleSessions      = 3   // an input older than this many session days is a stale_input gap
	markMaxAgeSessions = 5   // LedgerParams default of structures_page; the replay needs it for marks this page never reads
	staleAfterDays     = 4
)

var scopeKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// capture is what the account holds on its newest day with a capture.
type capture struct {
	day        string
	values     []store.AccountValue
	monthStart []store.AccountValue
	history    []blockapi.DayValue
}

// Build writes the household_page of alias for day; at is generated_at. alias "" is the main account.
func Build(ctx context.Context, c Config, alias string, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("household_page: no zone")
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("household_page: bad day %q: %w", day, err)
	}
	at = at.In(c.Zone)
	if alias == "" && len(c.Aliases) > 0 {
		alias = c.Aliases[0]
	}
	i := slices.Index(c.Aliases, alias)
	if i < 0 {
		return marshal(gapPage(c, alias, nil, at, "no_data", textUnknown))
	}
	tag := accountTag{Label: labelAccount + alias + labelClose, Whose: whoseOther}
	if i == 0 {
		tag.Whose = whoseMain
	}
	scopes := []accountScope{{Key: alias, Label: tag.Label, Accounts: []accountTag{tag}}}
	if c.Account == nil || c.Ledger == nil || c.Sessions == nil || c.Ledgers == nil || c.Figures == nil {
		return marshal(gapPage(c, alias, scopes, at, "no_reader", textNoReader))
	}
	sessions, err := c.sessionDays(ctx, d)
	if err != nil {
		return nil, err
	}
	snap, err := c.capture(ctx, alias, sessions)
	if err != nil {
		return nil, err
	}
	if snap.day == "" {
		return marshal(gapPage(c, alias, scopes, at, "no_data", textNoCapture))
	}
	if lag := len(sessions) - 1 - slices.Index(sessions, contracts.Day(snap.day)); lag > staleSessions {
		return marshal(gapPage(c, alias, scopes, at, "stale_input", textStale))
	}
	obligations, ledgerPuts, ledgerAt, err := c.obligations(ctx, alias, sessions, contracts.Day(snap.day))
	if err != nil {
		return nil, err
	}
	h, err := c.readHeld(ctx, alias, snap)
	if err != nil {
		return nil, err
	}
	mv, err := c.movers(ctx, alias, tag, d, sessions, snap, h)
	if err != nil {
		return nil, err
	}
	res := c.Figures().Household(blockapi.HouseholdInput{
		Alias: alias, Whose: tag.Whose, Day: contracts.Day(snap.day),
		Values: valuesIn(snap.values), MonthStart: valuesIn(snap.monthStart),
		History: snap.history, Obligations: obligations,
		UsdJpy: h.usdJpy, Positions: h.positionsIn(), PositionsRead: c.Marks != nil,
	})
	return marshal(c.assemble(alias, tag, scopes, snap, ledgerAt, res, c.holdings(alias, snap, h), mv, missingFromLedger(h.positions, ledgerPuts), at))
}

func marshal(p pageJSON) ([]byte, error) { return json.Marshal(p) }

func valuesIn(vs []store.AccountValue) []blockapi.ValueIn {
	out := make([]blockapi.ValueIn, 0, len(vs))
	for _, v := range vs {
		out = append(out, blockapi.ValueIn{Tag: v.Tag, Value: v.Value, Currency: v.Currency})
	}
	return out
}

// sessionDays returns the session days up to and including d, ascending, at most historyDays.
func (c Config) sessionDays(ctx context.Context, d time.Time) ([]contracts.Day, error) {
	from := contracts.Day(d.AddDate(0, 0, -sessionLookback).Format(dayLayout))
	rows, err := c.Sessions.SessionDays(ctx, calendarID, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("household_page: read session days: %w", err)
	}
	out := make([]contracts.Day, 0, len(rows))
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	slices.Sort(out)
	if len(out) > historyDays {
		out = out[len(out)-historyDays:]
	}
	return out, nil
}

// capture reads the values of every session day, newest first: the newest day with rows is the data
// day, the last session day of the month before it is the month start, the net liquidation of each
// day is the history.
func (c Config) capture(ctx context.Context, alias string, sessions []contracts.Day) (capture, error) {
	var cp capture
	byDay := map[string][]store.AccountValue{}
	for i := len(sessions) - 1; i >= 0; i-- {
		rows, err := c.Account.Values(ctx, alias, string(sessions[i]))
		if err != nil {
			return cp, fmt.Errorf("household_page: read values: %w", err)
		}
		if len(rows) == 0 {
			continue
		}
		byDay[string(sessions[i])] = rows
		if cp.day == "" {
			cp.day = string(sessions[i])
		}
		for _, v := range rows {
			if v.Tag == "NetLiquidation" {
				cp.history = append(cp.history, blockapi.DayValue{Day: sessions[i], NetLiquidation: v.Value, Currency: v.Currency})
			}
		}
	}
	if cp.day == "" {
		return cp, nil
	}
	cp.values = byDay[cp.day]
	for i := len(sessions) - 1; i >= 0; i-- {
		if s := string(sessions[i]); s < cp.day[:7] {
			cp.monthStart = byDay[s]
			break
		}
	}
	return cp, nil
}

// obligations replays the ledger of alias and returns its open short puts and the newest record time.
func (c Config) obligations(ctx context.Context, alias string, sessions []contracts.Day,
	day contracts.Day) ([]blockapi.ObligationIn, []string, time.Time, error) {
	var newest time.Time
	rows, err := c.Ledger.FlexTradeRows(ctx, alias)
	if err != nil {
		return nil, nil, newest, fmt.Errorf("household_page: read ledger: %w", err)
	}
	trades := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		attrs := map[string]string{}
		if err := json.Unmarshal([]byte(r.Attributes), &attrs); err != nil {
			return nil, nil, newest, fmt.Errorf("household_page: ledger row %s: %w", r.TransactionID, err)
		}
		trades = append(trades, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: attrs})
		if r.RecordedAt.After(newest) {
			newest = r.RecordedAt
		}
	}
	res := c.Ledgers().Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: sessions, AsOf: day},
		compute.LedgerParams{MarkMaxAgeSessions: markMaxAgeSessions})
	var out []blockapi.ObligationIn
	var keys []string
	for _, p := range res.Positions {
		if p.Status != "open" || p.Right != "P" || !strings.HasPrefix(p.Quantity, "-") {
			continue
		}
		keys = append(keys, putKey(p.Underlying, p.Strike, p.Expiry))
		out = append(out, blockapi.ObligationIn{
			Underlying: p.Underlying, Currency: p.Currency, Strike: p.Strike, Multiplier: p.Multiplier,
			Contracts: strings.TrimPrefix(p.Quantity, "-"), PremiumIncome: p.PremiumIncome,
		})
	}
	return out, keys, newest, nil
}

const (
	secTypeOption    = "option"
	maxListedPuts    = 5
	maxBlockGapRunes = 170 // the page list repeats a block gap under its title (at most 23 characters); both stay under 200
)

// putKey identifies a put by underlying, strike and expiry; the expiry loses its dashes and the strike
// its trailing zeros, so the stored and the ledger spelling meet.
func putKey(underlying, strike, expiry string) string {
	if strings.Contains(strike, ".") {
		strike = strings.TrimSuffix(strings.TrimRight(strike, "0"), ".")
	}
	return underlying + "|" + strike + "|" + strings.ReplaceAll(expiry, "-", "")
}

// missingFromLedger lists the stored short puts (symbol, strike, expiry) that no open short put of the
// ledger matches; each ledger put matches one stored put.
func missingFromLedger(positions []store.Position, ledgerKeys []string) []string {
	left := map[string]int{}
	for _, k := range ledgerKeys {
		left[k]++
	}
	var missing []string
	for _, p := range positions {
		if p.SecType != secTypeOption || p.Right != "put" || !strings.HasPrefix(p.Quantity, "-") {
			continue
		}
		if k := putKey(p.Symbol, p.Strike, p.Expiry); left[k] > 0 {
			left[k]--
			continue
		}
		missing = append(missing, strings.TrimSpace(p.Symbol+" P"+p.Strike+" "+p.Expiry))
	}
	return missing
}

// withLedgerGap marks the obligations, income and cash after blocks as gaps when stored short puts
// are missing from the ledger; the rows stay as they are.
func withLedgerGap(p *pageJSON, missing []string) {
	if len(missing) == 0 {
		return
	}
	listed := missing
	if len(listed) > maxListedPuts {
		listed = listed[:maxListedPuts]
	}
	text := ""
	for ; len(listed) > 0; listed = listed[:len(listed)-1] {
		text = fmt.Sprintf(textNotInLedger, len(missing), strings.Join(listed, ", "))
		if len([]rune(text)) <= maxBlockGapRunes {
			break
		}
	}
	if r := []rune(text); len(r) > maxBlockGapRunes {
		text = string(r[:maxBlockGapRunes])
	}
	g := gapJSON{Reason: "no_data", Text: text}
	p.Obligations.Gap, p.Income.Gap, p.CashAfter.Gap = &g, &g, &g
}

// assemble writes the page from the figures; without hb the mark blocks are the gap missing_quote,
// movers is the block of the two mark days. The run is ok when no block has a gap.
func (c Config) assemble(alias string, tag accountTag, scopes []accountScope, cp capture, ledgerAt time.Time,
	res blockapi.HouseholdResult, hb *holdingBlocks, mv moversPart, missing []string, at time.Time) pageJSON {
	p := base(c, scopes, alias, at)
	p.RunStatus = "partial"
	p.Title, p.Sentence = pageTitle, pageSentence
	p.Overview = moneyOf(titleOverview, meaningOverview, res.Overview)
	p.CashAfter = moneyOf(titleCashAfter, meaningCashAfter, res.CashAfter)
	p.Stress = moneyOf(titleStress, meaningStress, res.Stress)
	p.Obligations = moneyOf(titleObligations, meaningObligations, res.Obligations)
	p.Income = moneyOf(titleIncome, meaningIncome, res.Income)
	p.MonthChange = waterfallOf(res.MonthChange)
	if len(cp.monthStart) == 0 && cp.day != "" {
		p.MonthChange = waterfallBlock{Title: titleMonthChange, Meaning: meaningMonthChange, Rows: []waterfallStep{},
			Gap: &gapJSON{Reason: "missing_input", Text: fmt.Sprintf(textNoMonthStart, cp.day[:7]+"-01")}}
	}
	withLedgerGap(&p, missing)
	p.Trend = trendOf(alias, tag.Label, res.Trend)
	p.Accounts = accountsOf(alias, tag.Label, tag.Whose, cp.day, res.Accounts)
	if hb != nil {
		p.Composition, p.Concentration, p.Allocation = hb.composition, hb.concentration, hb.allocation
	} else {
		p.Composition = gapBlock(titleComposition, meaningComposition, missingQuote())
		p.Concentration = gapBlock(titleConcentration, meaningConcentration, missingQuote())
		p.Allocation = allocationGap(missingQuote())
	}
	p.Movers = mv.block
	if slices.Contains(c.Statement, alias) {
		p.Notes = append(p.Notes, textStatementFrom+cp.day+textStatementNote)
	}

	captured := at
	if len(cp.values) > 0 && !cp.values[0].CapturedAt.IsZero() {
		captured = cp.values[0].CapturedAt.In(c.Zone)
	}
	p.DataTimes = []sourceTime{{Source: "account", DataTime: captured.Format(timeLayout)}}
	if !ledgerAt.IsZero() {
		p.DataTimes = append(p.DataTimes, sourceTime{Source: "option_ledger", DataTime: ledgerAt.In(c.Zone).Format(timeLayout)})
	}
	p.DataTimes = append(p.DataTimes, mv.times...)
	p.Stale = staleLabel{
		DataTime: captured.Format(timeLayout), StaleAfter: captured.AddDate(0, 0, staleAfterDays).Format(timeLayout),
		Text: textStaleLabel + cp.day,
	}

	blocks := []struct {
		title string
		gap   *gapJSON
	}{
		{titleOverview, p.Overview.Gap}, {titleComposition, p.Composition.Gap}, {titleMovers, p.Movers.Gap},
		{titleTrend, p.Trend.Gap}, {titleStress, p.Stress.Gap}, {titleIncome, p.Income.Gap},
		{titleMonthChange, p.MonthChange.Gap}, {titleCashAfter, p.CashAfter.Gap}, {titleObligations, p.Obligations.Gap},
		{titleConcentration, p.Concentration.Gap}, {titleAllocation, p.Allocation.Gap},
	}
	for _, b := range blocks {
		if b.gap != nil {
			p.Gaps = append(p.Gaps, gapJSON{Reason: b.gap.Reason, Text: b.title + ": " + b.gap.Text})
		}
	}
	if len(p.Gaps) == 0 {
		p.RunStatus = "ok"
	}
	return p
}

func gapOf(g gapJSON) *gapJSON { return &g }

func allocationGap(g gapJSON) allocationBlock {
	return allocationBlock{Title: titleAllocation, Meaning: meaningAllocation, Rows: []allocationRow{}, Totals: []moneyCell{}, Gap: gapOf(g)}
}

func moversGap(g gapJSON) moversBlock {
	return moversBlock{
		Title: titleMovers, Meaning: meaningMovers, Rows: []moneyRow{}, Totals: []moneyCell{},
		Stocks: []moverRow{}, Options: []moverRow{}, Gap: gapOf(g),
	}
}

// base is the envelope with every block empty.
func base(c Config, scopes []accountScope, alias string, at time.Time) pageJSON {
	scope := alias
	if !scopeKey.MatchString(scope) {
		scope = "unknown"
	}
	if scopes == nil {
		scopes = []accountScope{}
	}
	return pageJSON{
		Schema: "household_page", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: scope,
		GeneratedAt: at.Format(timeLayout), DataTimes: []sourceTime{}, Notes: []string{}, Gaps: []gapJSON{}, Scopes: scopes,
		Stale: staleLabel{DataTime: at.Format(timeLayout), StaleAfter: at.Format(timeLayout), Text: textStaleLabel + "none"},
	}
}

// gapPage is the page of an alias that has nothing to show: every block is the one gap.
func gapPage(c Config, alias string, scopes []accountScope, at time.Time, reason, text string) pageJSON {
	p := base(c, scopes, alias, at)
	g := gapJSON{Reason: reason, Text: text}
	p.RunStatus = "failed"
	p.Title, p.Sentence = pageTitle, text
	p.Overview = gapBlock(titleOverview, meaningOverview, g)
	p.Composition = gapBlock(titleComposition, meaningComposition, g)
	p.Stress = gapBlock(titleStress, meaningStress, g)
	p.Income = gapBlock(titleIncome, meaningIncome, g)
	p.CashAfter = gapBlock(titleCashAfter, meaningCashAfter, g)
	p.Obligations = gapBlock(titleObligations, meaningObligations, g)
	p.Concentration = gapBlock(titleConcentration, meaningConcentration, g)
	p.Allocation = allocationGap(g)
	p.Movers = moversGap(g)
	p.Trend = trendBlock{Title: titleTrend, Meaning: meaningTrend, Rows: []trendPoint{}, Series: []trendSeries{}, Gap: gapOf(g)}
	p.MonthChange = waterfallBlock{Title: titleMonthChange, Meaning: meaningMonthChange, Rows: []waterfallStep{}, Gap: gapOf(g)}
	p.Gaps = []gapJSON{g}
	return p
}
