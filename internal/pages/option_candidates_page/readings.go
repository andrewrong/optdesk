package option_candidates_page

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const readingSubjectKind = "underlying"

// Ids of the stored name-level readings (Decision 11).
const (
	idVolForecast    = "market.vol_forecast"
	idVolForecastErr = "market.vol_forecast_err"
	idIVP            = "market.ivp_ibkr"
	idVolEdge        = "market.vol_edge"
)

// reading is one name-level reading: the stored text and its data day, or, with no ok reading, the
// reason word of the newest missing one.
type reading struct {
	text, day, word string
}

// nameReadings are the four readings of one name; noReader is "" when they were read, else the gap
// text of the missing reader or ticker.
type nameReadings struct {
	forecast, forecastErr, ivp, volEdge reading
	noReader                            string
}

// readingsFor reads the four readings of subject over the 7 days up to day. The ok reading with the
// newest day is used; with none, the newest missing reading gives the reason word, no_input when there is none.
func readingsFor(ctx context.Context, rd store.ReadingViewReader, subject string, day contracts.Day) (nameReadings, error) {
	from, err := daysBack(day, lookbackDays)
	if err != nil {
		return nameReadings{}, fmt.Errorf("option_candidates_page: bad day %q: %w", day, err)
	}
	var out nameReadings
	for _, f := range []struct {
		id  string
		dst *reading
	}{{idVolForecast, &out.forecast}, {idVolForecastErr, &out.forecastErr}, {idIVP, &out.ivp}, {idVolEdge, &out.volEdge}} {
		recs, err := rd.WinnerReadings(ctx, f.id, readingSubjectKind, subject, from, day)
		if err != nil {
			return nameReadings{}, fmt.Errorf("option_candidates_page: readings of %s: %w", f.id, err)
		}
		*f.dst = pickReading(recs, from, day)
	}
	return out, nil
}

func pickReading(recs []contracts.ReadingRecord, from, to contracts.Day) reading {
	ok, missing := reading{}, reading{word: readNoInput}
	haveOK, missingDay := false, ""
	for _, rec := range recs {
		d := rec.Day.Format(dayLayout)
		if d < string(from) || d > string(to) {
			continue
		}
		if rec.State == "ok" && rec.Value != nil {
			if !haveOK || d >= ok.day {
				ok, haveOK = reading{text: *rec.Value, day: d}, true
			}
		} else if d >= missingDay {
			missingDay, missing = d, reading{word: missingWord(rec)}
		}
	}
	if haveOK {
		return ok
	}
	return missing
}

func missingWord(rec contracts.ReadingRecord) string {
	if rec.ReasonCode != nil {
		if s, ok := readingReasons[*rec.ReasonCode]; ok {
			return s
		}
	}
	return readNoInput
}

// tickersOf is the ticker of each distinct underlying of rows, by underlying id; an id with no name is absent.
func tickersOf(ctx context.Context, names store.InstrumentNameReader, rows []store.Candidate) (map[int]string, error) {
	out := map[int]string{}
	if names == nil {
		return out, nil
	}
	var ids []int64
	for _, r := range rows {
		if r.UnderlyingID > 0 {
			if _, seen := out[r.UnderlyingID]; !seen {
				out[r.UnderlyingID] = ""
				ids = append(ids, int64(r.UnderlyingID))
			}
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	got, err := names.InstrumentNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("option_candidates_page: read names: %w", err)
	}
	for _, n := range got {
		out[int(n.InstrumentID)] = n.Name
	}
	return out, nil
}
