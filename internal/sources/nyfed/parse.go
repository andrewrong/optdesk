package nyfed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	sourceName = "nyfed"
	statusOK   = "present"
	percent    = contracts.SeriesUnit("percent")
)

type answer struct {
	RefRates []record `json:"refRates"`
}

type record struct {
	EffectiveDate *string         `json:"effectiveDate"`
	PercentRate   json.RawMessage `json:"percentRate"`
}

// parse maps the body of the NY Fed answer to points of seriesID. One bad record fails the whole
// answer, so nothing partial is returned. Points outside [from, to] are dropped (empty bound: open).
func parse(body []byte, seriesID string, from, to contracts.Day, now time.Time) ([]contracts.SeriesPoint, error) {
	var a answer
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, fmt.Errorf("nyfed: body does not parse")
	}
	out := make([]contracts.SeriesPoint, 0, len(a.RefRates))
	for i, r := range a.RefRates {
		if r.EffectiveDate == nil || *r.EffectiveDate == "" {
			return nil, fmt.Errorf("nyfed: record %d has no date", i)
		}
		t, err := time.Parse("2006-01-02", *r.EffectiveDate)
		if err != nil {
			return nil, fmt.Errorf("nyfed: record %d has a bad date", i)
		}
		v, err := parseRate(r.PercentRate)
		if err != nil {
			return nil, fmt.Errorf("nyfed: record %d: %w", i, err)
		}
		day := contracts.Day(*r.EffectiveDate)
		if (from != "" && day < from) || (to != "" && day > to) {
			continue
		}
		d := types.SerializableDate{Time: t}
		out = append(out, contracts.SeriesPoint{
			SeriesID:        seriesID,
			ObservationDate: d,
			Value:           &v,
			Unit:            percent,
			Status:          statusOK,
			Source:          sourceName,
			SourceRevision:  *r.EffectiveDate,
			RealtimeStart:   d,
			RealtimeEnd:     d,
			PublishedAt:     now,
		})
	}
	return out, nil
}

// parseRate reads a JSON number or a numeric string; absent, null, empty or other text is an error.
func parseRate(raw json.RawMessage) (float64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("rate is missing")
	}
	s := string(raw)
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0, fmt.Errorf("rate is not a number")
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, fmt.Errorf("rate is empty")
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("rate is not a number")
	}
	return v, nil
}
