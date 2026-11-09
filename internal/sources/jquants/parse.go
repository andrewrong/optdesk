package jquants

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

// body is the J-Quants earnings-date answer; only the fields the source uses are decoded.
type body struct {
	Data          []row           `json:"data"`
	PaginationKey json.RawMessage `json:"pagination_key"`
}

type row struct {
	SchDate string `json:"SchDate"`
	FYE     string `json:"FYE"`
	Code    string `json:"Code"`
}

// parseBody decodes one answer. A pagination_key is an error, never a silent truncation. A row with an
// empty or malformed SchDate is undecided and yields no listing, which the job records as absent.
func parseBody(raw []byte) ([]sources.EarningsListing, error) {
	var b body
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fail(contracts.Permanent, "not_json", "jquants: body is not JSON")
	}
	if len(b.PaginationKey) > 0 && string(b.PaginationKey) != "null" && string(b.PaginationKey) != `""` {
		return nil, fail(contracts.Permanent, "unexpected_pagination", "jquants: unexpected pagination_key")
	}
	var out []sources.EarningsListing
	for _, r := range b.Data {
		if !isDay(r.SchDate) {
			continue
		}
		l := sources.EarningsListing{Symbol: fourDigits(r.Code), ReportDate: contracts.Day(r.SchDate)}
		if isDay(r.FYE) {
			l.FiscalPeriodEnd = contracts.Day(r.FYE)
		}
		out = append(out, l)
	}
	return out, nil
}

func isDay(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// fourDigits cuts the vendor's 5-digit code (the 4 digits plus "0") to 4.
func fourDigits(code string) string {
	code = strings.TrimSpace(code)
	if len(code) == 5 {
		return code[:4]
	}
	return code
}
