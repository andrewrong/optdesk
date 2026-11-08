package store_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// memView is the implementing side: a fake ReadingViewReader over rows already held one per day.
type memView struct{ rows []contracts.ReadingRecord }

func (m *memView) WinnerReadings(_ context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	out := []contracts.ReadingRecord{}
	for _, r := range m.rows {
		if r.Name == name && r.SubjectKind == kind && r.Subject == subject && !r.Day.Before(string2time(from)) && !r.Day.After(string2time(to)) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day.Before(out[j].Day.Time) })
	return out, nil
}

func (m *memView) Close() error { return nil }

func day(d int) types.SerializableDate {
	return types.SerializableDate{Time: time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC)}
}

func string2time(d contracts.Day) time.Time {
	t, err := time.Parse("2006-01-02", string(d))
	if err != nil {
		panic(err)
	}
	return t
}

func TestReadingViewReaderContract(t *testing.T) {
	t.Parallel()
	open := store.OpenReadingViewFunc(func(_ context.Context, _ string) (store.ReadingViewReader, error) {
		return &memView{rows: []contracts.ReadingRecord{
			{Name: "n", SubjectKind: "k", Subject: "s", Day: day(5)},
			{Name: "n", SubjectKind: "k", Subject: "s", Day: day(4)},
		}}, nil
	})
	ctx := context.Background()
	r, err := open(ctx, "dsn")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	// The caller side: reads two days through the interface only.
	got, err := r.WinnerReadings(ctx, "n", "k", "s", "2026-11-04", "2026-11-05")
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v err %v", got, err)
	}
	if !got[0].Day.Equal(day(4).Time) || !got[1].Day.Equal(day(5).Time) {
		t.Fatalf("days not ascending: %+v", got)
	}

	none, err := r.WinnerReadings(ctx, "n", "k", "other", "2026-11-04", "2026-11-05")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("no rows = %#v err %v", none, err)
	}
}
