// rows.go reads the stored rows of one company and passes them to Summarize. It computes nothing: it
// chooses which stored rows are passed on (bars of status ok, the newest listed report row) and copies the
// Summary; counts, the latest quarter and the reasons come only from Summarize.
package company_data_page

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

const (
	barBasis    = "trades"
	barWindow   = 400 // days; the window the bars job keeps
	reportAhead = 400 // days ahead searched for a listed report
	dayLayout   = "2006-01-02"
	sourceSEC   = "sec"
)

// companyRow reads what is stored for one company and returns its row. An error is the failure of a read.
func companyRow(ctx context.Context, in Input, u underlying.Row) (Company, error) {
	today, err := time.Parse(dayLayout, string(in.Today))
	if err != nil {
		return Company{}, err
	}
	si := companydataapi.SummaryInput{Market: u.Market, Issuer: u.Issuer, Fundamentals: u.Fundamentals, HasInstrument: u.ConID != 0}
	if si.HasInstrument {
		from := contracts.Day(today.AddDate(0, 0, -barWindow).Format(dayLayout))
		bars, err := in.Bars.PriceBars(ctx, u.ConID, barBasis, from, in.Today)
		if err != nil {
			return Company{}, err
		}
		si.BarDays = barDays(bars)
	}
	if strings.HasPrefix(u.Issuer, "cik:") {
		if si.Facts, err = in.Facts.FactsWithPeriodFiledBy(ctx, sourceSEC, u.Issuer, in.Today); err != nil {
			return Company{}, err
		}
	}
	if u.Issuer != "" {
		to := contracts.Day(today.AddDate(0, 0, reportAhead).Format(dayLayout))
		rows, err := in.Earnings.EarningsDates(ctx, u.Issuer, in.Today, to)
		if err != nil {
			return Company{}, err
		}
		si.NextReport = nextReport(rows, in.Today)
	}
	sum, err := in.Calc.Summarize(si)
	if err != nil {
		return Company{}, err
	}
	c := copySummary(u, sum)
	if si.NextReport != nil {
		d := string(*si.NextReport)
		c.NextReport = &d
	}
	return c, nil
}

// barDays lists the days of the bars to pass on. A stored bar row has no status column: every row the
// reader returns is a bar that was stored, so every day is passed.
func barDays(bars []contracts.PriceBar) []contracts.Day {
	out := make([]contracts.Day, 0, len(bars))
	for _, b := range bars {
		out = append(out, contracts.Day(b.SessionDay.Format(dayLayout)))
	}
	return out
}

// nextReport is the report date of the newest listed row on or after today; rows are ascending by day.
func nextReport(rows []contracts.EarningsDate, today contracts.Day) *contracts.Day {
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if r.Status != gen.EarningsDateStatusListed || r.ReportDate == nil {
			continue
		}
		d := contracts.Day(r.ReportDate.Format(dayLayout))
		if d >= today {
			return &d
		}
	}
	return nil
}

func copySummary(u underlying.Row, s companydataapi.Summary) Company {
	c := Company{Key: u.Key, Symbol: u.Symbol, Name: u.Name, Market: u.Market, Issuer: u.Issuer,
		BarCount: strconv.Itoa(s.BarCount), Statements: s.Statements, LatestQuarter: s.LatestLabel,
		EPSQuarters: strconv.Itoa(s.EPSQuarters), Missing: []Missing{}}
	if s.FiscalYearEndMonth >= 1 && s.FiscalYearEndMonth <= 12 {
		c.FiscalYearEnd = time.Month(s.FiscalYearEndMonth).String()
	}
	if s.LastBarDay != nil {
		d := string(*s.LastBarDay)
		c.PriceTo = &d
	}
	if c.LatestQuarter == "" && s.LatestEnd != nil {
		c.LatestQuarter = labelPrefix + string(*s.LatestEnd)
	}
	if s.Statements {
		c.QuartersPath = quartersBase + "?c=" + url.QueryEscape(u.Key)
	}
	for _, m := range s.Missing {
		c.Missing = append(c.Missing, Missing{Code: string(m.Reason), Text: m.Text})
	}
	return c
}
