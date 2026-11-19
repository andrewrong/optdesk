package home_household

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	pairUsdJpy     = "USDJPY"
	currencyJPY    = "JPY"
	familyAliasKey = "household" // the key of the family view; never an account
)

// buildFamily is the card of a book with two or more aliases: the four tiles are the family sums of
// blockapi.FamilyFigure, copied as they are. The first alias is the main account: its reader error is an
// error of the build; any other alias that fails is left out with the reason read_error.
func (c Config) buildFamily(ctx context.Context, d time.Time, at time.Time) (doc, error) {
	if c.Family == nil {
		return doc{}, fmt.Errorf("home_household: no family figure")
	}
	if c.Aliases[0] == familyAliasKey {
		return gapDoc(c, at, "no_reader", textNoReader), nil
	}
	sessions, err := c.sessionDays(ctx, d)
	if err != nil {
		return doc{}, err
	}
	newest := ""
	if len(sessions) > 0 {
		newest = string(sessions[len(sessions)-1])
	}
	in := blockapi.FamilyIn{Accounts: make([]blockapi.FamilyAccountIn, 0, len(c.Aliases))}
	caps := map[string]capture{}
	var counted []string // data days of the accounts that carry values
	jpy := false
	for i, alias := range c.Aliases {
		a := blockapi.FamilyAccountIn{Alias: alias}
		if alias == familyAliasKey {
			a.Reason = "read_error"
			in.Accounts = append(in.Accounts, a)
			continue
		}
		snap, idx, err := c.captureOf(ctx, alias, sessions)
		switch {
		case err != nil && i == 0:
			return doc{}, err
		case err != nil:
			a.Reason = "read_error"
		case snap.day == "":
			a.Reason = "no_data"
		case len(sessions)-1-idx > staleSessions:
			a.Reason, a.Day = "stale_input", snap.day
		default:
			a.Day = snap.day
			a.Values, a.Prior, a.MonthStart = valuesIn(snap.values), valuesIn(snap.prior), valuesIn(snap.monthStart)
			caps[alias] = snap
			counted = append(counted, snap.day)
			for _, v := range snap.values {
				jpy = jpy || v.Currency == currencyJPY
			}
		}
		in.Accounts = append(in.Accounts, a)
	}
	if jpy && c.Fx != nil && len(counted) > 0 {
		if err := c.readRate(ctx, &in, counted); err != nil {
			return doc{}, err
		}
	}
	res := c.Family().Family(in)
	return c.assembleFamily(in, res, caps, newest, at), nil
}

// assembleFamily writes the card of a family result. Every figure is copied from res.
func (c Config) assembleFamily(in blockapi.FamilyIn, res blockapi.FamilyResult, caps map[string]capture, newest string, at time.Time) doc {
	d := base(c, at)
	oldest := res.OldestDay
	net := blockapi.MoneyLine{Amount: res.Net, Currency: res.Currency}
	cash := blockapi.MoneyLine{Amount: res.Cash, Currency: res.Currency, Share: res.CashShare}
	d.Card.Tiles = tiles{
		NetWorth:    netWorthTile(net, oldest),
		Cash:        cashTile(cash, net),
		DayChange:   moveTile(labelDayChange, explainDayChange, res.DayAmount, res.DayPct, res.Currency),
		MonthToDate: moveTile(labelMonthToDate, explainMonthToDate, res.MonthAmount, res.MonthPct, res.Currency),
	}
	d.Card.Tiles.DayChange = sayFamilyGap(d.Card.Tiles.DayChange, res.DayAmount.Gap, in, res.Included, familyNeedsPrior,
		func(a blockapi.FamilyAccountIn) bool { return len(a.Prior) == 0 })
	d.Card.Tiles.MonthToDate = sayFamilyGap(d.Card.Tiles.MonthToDate, res.MonthAmount.Gap, in, res.Included, familyNeedsMonthStart,
		func(a blockapi.FamilyAccountIn) bool { return len(a.MonthStart) == 0 })

	d.RunStatus = "ok"
	for _, t := range []struct {
		label string
		gap   *gapJSON
	}{
		{labelNetWorth, d.Card.Tiles.NetWorth.gap()}, {labelCash, d.Card.Tiles.Cash.gap()},
		{labelDayChange, d.Card.Tiles.DayChange.gap()}, {labelMonthToDate, d.Card.Tiles.MonthToDate.gap()},
	} {
		if t.gap != nil {
			d.RunStatus = "partial"
			d.Card.Gaps = append(d.Card.Gaps, gapJSON{Reason: t.gap.Reason, Text: t.label + ": " + t.gap.Text})
		}
	}
	if len(res.Included) == 0 {
		d.RunStatus = "failed"
	}

	shown := oldest
	if shown == "" {
		shown = familyNoDay
	}
	d.Card.Subtitle = fmt.Sprintf(familySubtitle, len(res.Included), len(c.Aliases), shown)
	d.Card.DataTimesText = textDataDay + shown
	d.DataTimes = []sourceTime{}
	var oldestAt time.Time
	for _, alias := range res.Included {
		cp := caps[alias]
		if len(cp.values) == 0 || cp.values[0].CapturedAt.IsZero() {
			continue
		}
		t := cp.values[0].CapturedAt.In(c.Zone)
		d.DataTimes = append(d.DataTimes, sourceTime{Source: "account", DataTime: t.Format(timeLayout)})
		if oldestAt.IsZero() || t.Before(oldestAt) {
			oldestAt = t
		}
	}
	if !oldestAt.IsZero() {
		d.StaleAfter = oldestAt.AddDate(0, 0, staleAfterDays).Format(timeLayout)
		d.Card.Stale = staleLabel{DataTime: oldestAt.Format(timeLayout), StaleAfter: d.StaleAfter, Text: textStaleLabel + oldest}
	}
	d.Card.Notes = familyNotes(res, newest)
	return d
}

// sayFamilyGap words the gap of a change tile of the family: missing_input names the counted accounts that
// lack the earlier capture, stale_input of the day tile is cardDayMixed. Any other tile is returned as it is.
func sayFamilyGap(t tile, reason string, in blockapi.FamilyIn, included []string, format string, lacks func(blockapi.FamilyAccountIn) bool) tile {
	var text string
	switch reason {
	case "stale_input":
		text = cardDayMixed
	case "missing_input":
		var names []string
		for _, a := range in.Accounts {
			if lacks(a) && contains(included, a.Alias) {
				names = append(names, fmt.Sprintf(familyAccountLabel, a.Alias))
			}
		}
		if len(names) == 0 {
			return t
		}
		text = fmt.Sprintf(format, strings.Join(names, familyListSeparator))
	default:
		return t
	}
	g := gapJSON{Reason: reason, Text: text}
	t.Figure, t.Caption = g, g
	return t
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// familyNotes is one line per left-out alias, one per counted alias from an earlier day, the rate line
// when a rate was used and the cash line.
func familyNotes(res blockapi.FamilyResult, newest string) []string {
	notes := []string{}
	for _, x := range res.Excluded {
		who := fmt.Sprintf(familyAccountLabel, x.Alias)
		switch {
		case x.Reason == "stale_input" && x.Day != "":
			notes = append(notes, fmt.Sprintf(familyLineStale, who, x.Day))
		case x.Reason == "read_error":
			notes = append(notes, fmt.Sprintf(familyLineReadError, who))
		case x.Reason == "unsupported_currency":
			notes = append(notes, fmt.Sprintf(familyLineCurrency, who))
		default:
			notes = append(notes, fmt.Sprintf(familyLineNoData, who))
		}
	}
	for _, r := range res.Rows {
		if r.Reason == "" && r.Day != "" && r.Day != newest && contains(res.Included, r.Alias) {
			notes = append(notes, fmt.Sprintf(familyLineEarlierDay, fmt.Sprintf(familyAccountLabel, r.Alias), r.Day))
		}
	}
	if res.FxRate != "" {
		notes = append(notes, fmt.Sprintf(familyRateLine, res.FxRate, res.FxDay, res.FxSource))
	}
	return append(notes, familyCashLine)
}

var _ store.AccountReader // the readers are those of Config

// readRate sets the USDJPY rate of the newest data day among days, with its day and its source; no rate
// stored leaves the input without one.
func (c Config) readRate(ctx context.Context, in *blockapi.FamilyIn, days []string) error {
	last := days[0]
	for _, day := range days {
		if day > last {
			last = day
		}
	}
	r, found, err := c.Fx.LatestFxRate(ctx, pairUsdJpy, contracts.Day(last))
	if err != nil {
		return fmt.Errorf("home_household: read fx rate: %w", err)
	}
	if found {
		in.UsdJpy, in.RateDay, in.RateSource = r.Rate, r.RateDay.Format(dayLayout), string(r.Source)
	}
	return nil
}
