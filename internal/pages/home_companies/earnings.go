package home_companies

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const earningsLookbackDays = 120

// earningsOut is the EarningsCell of the page schema.
type earningsOut struct {
	Kind string `json:"kind"`
	Date string `json:"date"`
	Text string `json:"text"`
}

// earningsCell reads the stored earnings rows of the issuer for the 120 days up to day and shows
// the report date of the last row when it is listed and has one: "MM-DD" on or after day,
// "已公布 MM-DD" before it. No reader, no row or a row without a date is a gap; a reader error is
// returned. The text copies the stored date.
func (c Config) earningsCell(ctx context.Context, issuer string, day time.Time) (any, error) {
	if c.Earnings == nil {
		return mkGap("no_reader", "earnings date: "+noReaderText), nil
	}
	from := contracts.Day(day.AddDate(0, 0, -earningsLookbackDays).Format(dayLayout))
	list, err := c.Earnings.EarningsDates(ctx, issuer, from, contracts.Day(day.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("home_companies: read earnings dates of %s: %w", issuer, err)
	}
	if len(list) == 0 {
		return mkGap("no_data", "earnings date: none listed"), nil
	}
	last := list[len(list)-1]
	if string(last.Status) != "listed" || last.ReportDate == nil {
		return mkGap("no_data", "earnings date: none listed"), nil
	}
	report := last.ReportDate.Time
	text := report.Format("01-02")
	if report.Format(dayLayout) < day.Format(dayLayout) {
		text = "已公布 " + text
	}
	return earningsOut{Kind: "value", Date: report.Format(dayLayout), Text: text}, nil
}
