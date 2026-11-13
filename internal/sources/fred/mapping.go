package fred

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	sourceName   = "fred"
	statusOK     = "present"
	statusNoVal  = "no_value"
	noValueToken = "." // FRED writes a missing observation as a dot.
)

type observationsResponse struct {
	Observations []struct {
		RealtimeStart string `json:"realtime_start"`
		RealtimeEnd   string `json:"realtime_end"`
		Date          string `json:"date"`
		Value         string `json:"value"`
	} `json:"observations"`
}

func parseDay(s string) (types.SerializableDate, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return types.SerializableDate{}, fmt.Errorf("bad date %q", s)
	}
	return types.SerializableDate{Time: t}, nil
}

// unitRow states how one configured series is stored.
type unitRow struct {
	unit       contracts.SeriesUnit
	currency   string // set for ccy_amount only
	shift      int    // decimal places the FRED value text moves right: millions 6, billions 9
	pinVintage bool   // realtime_start, realtime_end and source_revision become the observation date
}

const (
	ccyAmount   = contracts.SeriesUnit("ccy_amount")
	percent     = contracts.SeriesUnit("percent")
	indexPoints = contracts.SeriesUnit("index_points")
)

// seriesUnits states the unit of each configured series; a series absent here is refused.
// Each row is checked against recorded FRED metadata in testdata/series_units.json.
var seriesUnits = map[string]unitRow{
	"DGS10": {unit: percent},
	"DGS2":  {unit: percent},
	// Market series: FRED's units text is "Index" for these five and "Percent" for DGS3MO.
	"DGS3MO": {unit: percent},
	"SP500":  {unit: indexPoints},
	"VIXCLS": {unit: indexPoints},
	"VXNCLS": {unit: indexPoints},
	"RVXCLS": {unit: indexPoints},
	"VXVCLS": {unit: indexPoints},
	// Ease series: whole units of the currency, first value seen kept (ease-v2 Decision 3).
	"WALCL":        {unit: ccyAmount, currency: "USD", shift: 6, pinVintage: true},
	"WDTGAL":       {unit: ccyAmount, currency: "USD", shift: 6, pinVintage: true},
	"RRPONTSYD":    {unit: ccyAmount, currency: "USD", shift: 9, pinVintage: true},
	"DFF":          {unit: percent, pinVintage: true},
	"BAMLH0A0HYM2": {unit: percent, pinVintage: true},
	"BAMLC0A0CM":   {unit: percent, pinVintage: true},
	"BAA10Y":       {unit: percent, pinVintage: true},
	"SOFR":         {unit: percent, pinVintage: true},
	"WRESBAL":      {unit: ccyAmount, currency: "USD", shift: 6, pinVintage: true},
}

// shiftDecimalText moves the decimal point of a plain decimal text n places right, on the text,
// so that no float error enters: "0.123" shifted 9 is "123000000".
func shiftDecimalText(s string, n int) (string, error) {
	if n == 0 {
		return s, nil
	}
	sign := ""
	body := s
	if strings.HasPrefix(body, "-") || strings.HasPrefix(body, "+") {
		if body[0] == '-' {
			sign = "-"
		}
		body = body[1:]
	}
	whole, frac, _ := strings.Cut(body, ".")
	if whole == "" && frac == "" {
		return "", fmt.Errorf("bad decimal %q", s)
	}
	for _, r := range whole + frac {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("bad decimal %q", s)
		}
	}
	for len(frac) < n {
		frac += "0"
	}
	out := strings.TrimLeft(whole+frac[:n], "0")
	if out == "" {
		out = "0"
	}
	if rest := frac[n:]; rest != "" {
		out += "." + rest
	}
	return sign + out, nil
}

// mapObservations maps the body of a FRED series/observations response to records.
// publishedAt is the instant the response was fetched: FRED gives no publication instant.
func mapObservations(seriesID string, body []byte, publishedAt time.Time) ([]contracts.SeriesPoint, error) {
	row, ok := seriesUnits[seriesID]
	if !ok {
		return nil, fmt.Errorf("fred: no unit configured for series %s", seriesID)
	}
	var resp observationsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fred: undecodable response")
	}
	out := make([]contracts.SeriesPoint, 0, len(resp.Observations))
	for _, o := range resp.Observations {
		date, err := parseDay(o.Date)
		if err != nil {
			return nil, fmt.Errorf("fred: %w", err)
		}
		rs, err := parseDay(o.RealtimeStart)
		if err != nil {
			return nil, fmt.Errorf("fred: %w", err)
		}
		re, err := parseDay(o.RealtimeEnd)
		if err != nil {
			return nil, fmt.Errorf("fred: %w", err)
		}
		p := contracts.SeriesPoint{
			SeriesID:        seriesID,
			ObservationDate: date,
			RealtimeStart:   rs, // vintage: kept as FRED gives it
			RealtimeEnd:     re,
			PublishedAt:     publishedAt,
			Source:          sourceName,
			SourceRevision:  o.RealtimeStart,
			// Conversion boundary: FRED values are bare numbers; unit, currency and shift come from seriesUnits.
			Unit:     row.unit,
			Currency: row.currency,
		}
		if row.pinVintage {
			p.RealtimeStart, p.RealtimeEnd, p.SourceRevision = date, date, o.Date
		}
		if o.Value == noValueToken {
			p.Status = statusNoVal
		} else {
			text, err := shiftDecimalText(o.Value, row.shift)
			if err != nil {
				return nil, fmt.Errorf("fred: bad value for %s", o.Date)
			}
			v, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return nil, fmt.Errorf("fred: bad value for %s", o.Date)
			}
			p.Status = statusOK
			p.Value = &v
		}
		out = append(out, p)
	}
	return out, nil
}
