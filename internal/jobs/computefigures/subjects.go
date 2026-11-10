// Package computefigures is the compute_figures job: it reads the stored inputs, asks the compute
// core for each figure and stores the answers as readings.
package computefigures

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	kindUnderlying = "underlying"
	kindMarket     = "market"
	chunkSize      = 500
	dayLayout      = time.DateOnly
)

// subject is one thing a figure is computed for: an underlying (id is its instrument id) or a market series.
type subject struct {
	kind string
	name string
	id   int64
}

// subjects lists the underlyings that have a row of option_eod on day itself, ordered by id, named by
// their symbol, or underlying:<id> when no name is stored.
func subjects(ctx context.Context, s ComputeStores, day contracts.Day) ([]subject, error) {
	out, _, err := listSubjects(ctx, s, day)
	return out, err
}

// listSubjects is subjects with the count of underlyings dropped because they have only later rows.
// Two ids with the same stored name are both keyed underlying:<id>, so a key names one underlying.
func listSubjects(ctx context.Context, s ComputeStores, day contracts.Day) (out []subject, dropped int, err error) {
	latest, err := s.Catalog.OptionEodLatest(ctx, day)
	if err != nil {
		return nil, 0, fmt.Errorf("computefigures: catalog: %w", err)
	}
	var ids []int64
	for _, l := range latest {
		rows, err := s.OptionEod.OptionEod(ctx, l.UnderlyingID, day, day)
		if err != nil {
			return nil, 0, fmt.Errorf("computefigures: option_eod of %d: %w", l.UnderlyingID, err)
		}
		if len(rows) > 0 {
			ids = append(ids, l.UnderlyingID)
		} else {
			dropped++
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	names := map[int64]string{}
	for _, c := range chunks(ids) {
		got, err := s.Names.InstrumentNames(ctx, c)
		if err != nil {
			return nil, 0, fmt.Errorf("computefigures: names: %w", err)
		}
		for _, n := range got {
			names[n.InstrumentID] = n.Name
		}
	}
	uses := map[string]int{}
	for _, id := range ids {
		uses[names[id]]++
	}
	out = make([]subject, 0, len(ids))
	for _, id := range ids {
		name := names[id]
		if name == "" || uses[name] > 1 {
			name = kindUnderlying + ":" + strconv.FormatInt(id, 10)
		}
		out = append(out, subject{kind: kindUnderlying, name: name, id: id})
	}
	return out, dropped, nil
}

// chunks splits ids into slices of at most chunkSize.
func chunks(ids []int64) [][]int64 {
	var out [][]int64
	for len(ids) > 0 {
		n := min(len(ids), chunkSize)
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	return out
}

func parseDay(d contracts.Day) (time.Time, error) {
	t, err := time.Parse(dayLayout, string(d))
	if err != nil {
		return time.Time{}, fmt.Errorf("computefigures: bad day %q", string(d))
	}
	return t, nil
}

// shiftDay is d moved by n calendar days; a day that does not parse is returned as it is.
func shiftDay(d contracts.Day, n int) contracts.Day {
	t, err := parseDay(d)
	if err != nil {
		return d
	}
	return contracts.Day(t.AddDate(0, 0, n).Format(dayLayout))
}

// daysBetween is the calendar days from a to b.
func daysBetween(a, b contracts.Day) int {
	ta, err1 := parseDay(a)
	tb, err2 := parseDay(b)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(tb.Sub(ta).Hours() / 24)
}

func dayOf(d types.SerializableDate) contracts.Day { return contracts.Day(d.Format(dayLayout)) }
