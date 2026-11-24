package fredseries

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// readerFake serves stored points per series id; err fails every read.
type readerFake struct {
	byID map[string][]contracts.SeriesPoint
	err  error
}

func (r *readerFake) SeriesPoints(_ context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	if r.err != nil {
		return nil, r.err
	}
	var out []contracts.SeriesPoint
	for _, p := range r.byID[id] {
		d := contracts.Day(p.ObservationDate.Format("2006-01-02"))
		if d >= from && d <= to {
			out = append(out, p)
		}
	}
	return out, nil
}

func pt(id, day string, valued bool) contracts.SeriesPoint {
	d, _ := time.Parse("2006-01-02", day)
	p := contracts.SeriesPoint{SeriesID: id, ObservationDate: types.SerializableDate{Time: d}, Status: "present"}
	if valued {
		v := 1.0
		p.Value = &v
	} else {
		p.Status = "no_value"
	}
	return p
}

func TestGapFrom(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		byID map[string][]contracts.SeriesPoint
		ids  []string
		want contracts.Day
	}{
		{"oldest of the newest", map[string][]contracts.SeriesPoint{
			"A": {pt("A", "2026-11-01", true), pt("A", "2026-11-08", true)},
			"B": {pt("B", "2026-11-05", true)},
		}, []string{"A", "B"}, "2026-11-06"},
		{"no point gives groupFrom", map[string][]contracts.SeriesPoint{"A": {pt("A", "2026-11-08", true)}}, []string{"A", "B"}, "2026-10-20"},
		{"unvalued point ignored", map[string][]contracts.SeriesPoint{"A": {pt("A", "2026-11-03", true), pt("A", "2026-11-09", false)}}, []string{"A"}, "2026-11-04"},
		{"not after day", map[string][]contracts.SeriesPoint{"A": {pt("A", "2026-11-10", true)}}, []string{"A"}, "2026-11-10"},
		{"not before groupFrom", map[string][]contracts.SeriesPoint{"A": {pt("A", "2026-10-01", true)}}, []string{"A"}, "2026-10-20"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := gapFrom(context.Background(), &readerFake{byID: c.byID}, c.ids, "2026-10-20", "2026-11-10")
			if err != nil || got != c.want {
				t.Fatalf("gapFrom = %s, %v; want %s", got, err, c.want)
			}
		})
	}
	if _, err := gapFrom(context.Background(), &readerFake{err: errors.New("x")}, []string{"A"}, "2026-10-20", "2026-11-10"); err == nil {
		t.Fatal("reader error not returned")
	}
}
