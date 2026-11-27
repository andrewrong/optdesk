package household_page

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// maxTrendAccounts is the most accounts that get a line of their own: the schema holds 4 series, the family
// line and three accounts.
const maxTrendAccounts = 3

// familyTrend is the trend block of the family page and the lead note that goes with it. It is nil when
// Config.FamilyTrend is nil (the block stays the gap not_built). The sums are those of the figure; this
// function reads the history of the counted accounts, the yen rate of each day a counted yen account has a
// value, and copies the text of the result.
func (c Config) familyTrend(ctx context.Context, res blockapi.FamilyResult, caps map[string]capture, sessions []contracts.Day) (*trendBlock, []string) {
	if c.FamilyTrend == nil {
		return nil, nil
	}
	b := trendBlock{Title: titleTrend, Meaning: meaningTrend, Rows: []trendPoint{}, Series: []trendSeries{}}
	noData := func() (*trendBlock, []string) {
		b.Gap = new(gapJSON{Reason: "no_data", Text: familyTrendNoData})
		return &b, nil
	}
	if len(res.Included) == 0 {
		return noData()
	}
	in := blockapi.FamilyTrendIn{Window: sessions, UsdJpy: map[contracts.Day]string{}}
	for _, alias := range res.Included {
		days := slices.Clone(caps[alias].history) // newest first as read; the figure takes ascending days
		slices.Reverse(days)
		in.Accounts = append(in.Accounts, blockapi.FamilyTrendAccountIn{Alias: alias, Days: days})
		for _, d := range days {
			if d.Currency != currencyJPY || c.Fx == nil {
				continue
			}
			if _, done := in.UsdJpy[d.Day]; done {
				continue
			}
			if r, found, err := c.Fx.LatestFxRate(ctx, pairUsdJpy, d.Day); err == nil && found {
				in.UsdJpy[d.Day] = r.Rate
			}
		}
	}
	in.Window = sinceFirstRecord(sessions, in.Accounts)
	out := c.FamilyTrend().FamilyTrend(in)
	if out.Gap != "" || len(out.Family) == 0 {
		return noData()
	}
	rows := trendRows(out.Family)
	b.Rows = rows
	b.Window = familyTrendWindow(out)
	b.Series = []trendSeries{{Scope: familyAliasKey, Label: familyTrendLabel, Rows: rows, Window: b.Window}}
	var notes []string
	if len(res.Included) > maxTrendAccounts {
		notes = append(notes, familyTrendMany)
	} else {
		for _, a := range out.Accounts {
			b.Series = append(b.Series, trendSeries{Scope: a.Alias, Label: familyAccountLabel(a.Alias), Rows: trendRows(a.Points)})
		}
	}
	return &b, notes
}

func trendRows(points []blockapi.FamilyTrendPoint) []trendPoint {
	rows := make([]trendPoint, 0, len(points))
	for _, p := range points {
		rows = append(rows, trendPoint{Day: string(p.Day), Value: usdFigure{
			Value: json.Number(p.Net), Text: moneyText("$", p.Net), Unit: "USD"}})
	}
	return rows
}

// familyTrendWindow names the first day, the last day and the number of family points, then the days left
// out when there are any, then the yen sentence when a yen value was converted.
func familyTrendWindow(out blockapi.FamilyTrendResult) string {
	first, last := out.Family[0].Day, out.Family[len(out.Family)-1].Day
	s := fmt.Sprintf(familyTrendWindowText, first, last, len(out.Family))
	if out.LeftOut > 0 {
		s += fmt.Sprintf(familyTrendLeftOut, strconv.Itoa(out.LeftOut))
	}
	if out.YenUsed {
		s += familyTrendYen
	}
	return s
}

// sinceFirstRecord cuts the window to the days from the first day on which any counted account has a
// record, so the days before every record are not counted as left out. With no record it is returned whole.
func sinceFirstRecord(window []contracts.Day, accounts []blockapi.FamilyTrendAccountIn) []contracts.Day {
	first := contracts.Day("")
	for _, a := range accounts {
		for _, d := range a.Days {
			if first == "" || d.Day < first {
				first = d.Day
			}
		}
	}
	if first == "" {
		return window
	}
	i, _ := slices.BinarySearch(window, first)
	return window[i:]
}
