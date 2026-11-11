package option_data_page

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	earningsLookbackDays = 120

	toneNone    = "none"
	toneOutside = "outside"
	toneInTerm  = "in_term"
)

// applyEarnings fills earnings_text and earnings_tone of u from the last earnings row of the issuer.
// With no reader or no issuer the fields stay absent. sessionDay is the day of the chain and rows
// its chain rows, whose latest expiry bounds "in term".
func applyEarnings(ctx context.Context, c Config, d time.Time, day contracts.Day, sessionDay string, rows []chainRow, u *underlying) error {
	if c.Earnings == nil || c.Issuer == "" {
		return nil
	}
	from := contracts.Day(d.AddDate(0, 0, -earningsLookbackDays).Format(dayLayout))
	list, err := c.Earnings.EarningsDates(ctx, c.Issuer, from, day)
	if err != nil {
		return fmt.Errorf("option_data_page: read earnings dates: %w", err)
	}
	u.EarningsTone = toneNone
	if len(list) == 0 {
		return nil
	}
	last := list[len(list)-1]
	if string(last.Status) != "listed" || last.ReportDate == nil {
		return nil
	}
	report := last.ReportDate.Format(dayLayout)
	if report < sessionDay {
		return nil
	}
	word := "vendor"
	if string(last.Source) == "owner" {
		word = "owner"
	}
	u.EarningsText = "E " + last.ReportDate.Format("01-02") + " · source " + word
	u.EarningsTone = toneOutside
	if expiry := latestExpiry(rows); expiry != "" && report <= expiry {
		u.EarningsTone = toneInTerm
	}
	return nil
}

// latestExpiry is the greatest expiry of the rows, "" when none has one.
func latestExpiry(rows []chainRow) string {
	var latest string
	for _, r := range rows {
		if r.Expiry != nil && *r.Expiry > latest {
			latest = *r.Expiry
		}
	}
	return latest
}
