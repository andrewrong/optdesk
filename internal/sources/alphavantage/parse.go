package alphavantage

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

// minRows is the fewest data rows of a calendar the source accepts; a shorter body is refused.
const minRows = 500

// bom is the UTF-8 byte order mark a csv file may start with.
const bom = "\ufeff"

// parseCalendar turns a response body into listings. The columns are found by their header names.
// A JSON body is the vendor's notice (wrong key or used-up quota); it never becomes an empty result.
func parseCalendar(body []byte) ([]sources.EarningsListing, error) {
	trimmed := bytes.TrimSpace(body)
	if bytes.HasPrefix(trimmed, []byte("{")) {
		return nil, fail(contracts.Permanent, "rate_limited", "alphavantage: notice body (key or quota)")
	}
	r := csv.NewReader(bytes.NewReader(trimmed))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return nil, fail(contracts.Permanent, "not_csv", "alphavantage: body is not csv")
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.TrimSpace(strings.TrimPrefix(h, bom))] = i
	}
	si, ok1 := col["symbol"]
	ri, ok2 := col["reportDate"]
	if !ok1 || !ok2 {
		return nil, fail(contracts.Permanent, "not_csv", "alphavantage: header lacks symbol or reportDate")
	}
	field := func(rec []string, name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	var out []sources.EarningsListing
	rows := 0
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fail(contracts.Permanent, "not_csv", "alphavantage: csv row cannot be read")
		}
		rows++
		if si >= len(rec) || ri >= len(rec) {
			continue
		}
		sym, date := strings.TrimSpace(rec[si]), strings.TrimSpace(rec[ri])
		if sym == "" || date == "" {
			continue
		}
		out = append(out, sources.EarningsListing{
			Symbol:          sym,
			ReportDate:      contracts.Day(date),
			FiscalPeriodEnd: contracts.Day(field(rec, "fiscalDateEnding")),
			Estimate:        field(rec, "estimate"),
			TimeOfDay:       field(rec, "timeOfTheDay"),
		})
	}
	if rows < minRows {
		return nil, fail(contracts.Permanent, "short_calendar", "alphavantage: calendar has fewer than 500 rows")
	}
	return out, nil
}
