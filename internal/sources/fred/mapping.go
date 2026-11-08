package fred

import (
	"encoding/json"
	"fmt"
	"strconv"
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

// seriesUnits states the unit of each configured series; a series absent here is refused.
// Each row is checked against recorded FRED metadata in testdata/series_units.json.
var seriesUnits = map[string]contracts.SeriesUnit{
	"DGS10": contracts.SeriesUnit("percent"),
	"DGS2":  "percent",
	// Market series: FRED's units text is "Index" for these five and "Percent" for DGS3MO.
	"DGS3MO": "percent",
	"SP500":  "index_points",
	"VIXCLS": "index_points",
	"VXNCLS": "index_points",
	"RVXCLS": "index_points",
	"VXVCLS": "index_points",
}

// mapObservations maps the body of a FRED series/observations response to records.
// publishedAt is the instant the response was fetched: FRED gives no publication instant.
func mapObservations(seriesID string, body []byte, publishedAt time.Time) ([]contracts.SeriesPoint, error) {
	unit, ok := seriesUnits[seriesID]
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
			// Conversion boundary: FRED values are bare numbers; the unit comes from seriesUnits.
			Unit: unit,
		}
		if o.Value == noValueToken {
			p.Status = statusNoVal
		} else {
			v, err := strconv.ParseFloat(o.Value, 64)
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
