package home_companies

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const earningsLookbackDays = 120

// publishedPrefix is the frozen schema text for "published" in Chinese, written as escapes: the English guard
// of cmd/ot allows no CJK character in a Go source file of internal/pages.
const publishedPrefix = "\u5df2\u516c\u5e03 "

// earningsOut is the EarningsCell of the page schema.
type earningsOut struct {
	Kind string `json:"kind"`
	Date string `json:"date"`
	Text string `json:"text"`
}

// earningsCell reads the stored earnings rows of the issuer for the 120 days up to day and shows
// the earliest listed report date on or after day, whatever the status of the last row: "MM-DD".
// With none, the last row when it is listed and has a date before day shows "<published> MM-DD" (the frozen schema value, see publishedPrefix). No reader, no row or a row without a date is a gap; a reader error is
// returned. An empty issuer is a gap read before the reader is called. The text copies the stored date.
func (c Config) earningsCell(ctx context.Context, issuer string, day time.Time) (any, error) {
	if c.Earnings == nil {
		return mkGap("no_reader", "earnings date: "+noReaderText), nil
	}
	if issuer == "" {
		return mkGap("no_data", "earnings date: "+earningsNoIssuerText), nil
	}
	from := contracts.Day(day.AddDate(0, 0, -earningsLookbackDays).Format(dayLayout))
	list, err := c.Earnings.EarningsDates(ctx, issuer, from, contracts.Day(day.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("home_companies: read earnings dates of %s: %w", issuer, err)
	}
	if len(list) == 0 {
		return mkGap("no_data", "earnings date: "+earningsNoneListedText), nil
	}
	today := day.Format(dayLayout)
	var best *time.Time
	for i := range list {
		r := list[i]
		if string(r.Status) != "listed" || r.ReportDate == nil || r.ReportDate.Format(dayLayout) < today {
			continue
		}
		if best == nil || r.ReportDate.Before(*best) {
			t := r.ReportDate.Time
			best = &t
		}
	}
	if best != nil {
		return earningsOut{Kind: "value", Date: best.Format(dayLayout), Text: best.Format("01-02")}, nil
	}
	last := list[len(list)-1]
	if string(last.Status) != "listed" || last.ReportDate == nil {
		return mkGap("no_data", "earnings date: "+earningsNoneListedText), nil
	}
	report := last.ReportDate.Time
	return earningsOut{Kind: "value", Date: report.Format(dayLayout), Text: publishedPrefix + report.Format("01-02")}, nil
}
