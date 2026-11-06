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

// mapObservations maps the body of a FRED series/observations response to records.
// publishedAt is the instant the response was fetched: FRED gives no publication instant.
func mapObservations(seriesID string, body []byte, publishedAt time.Time) ([]contracts.SeriesPoint, error) {
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
			// Conversion boundary: FRED values are bare numbers; the unit is stated here and
			// nowhere else. The series of this client (DGS10) are quoted in percent.
			Unit: contracts.SeriesUnit("percent"),
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
