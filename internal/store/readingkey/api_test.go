package readingkey_test

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/readingkey"
)

type key struct {
	name, kind, subject, day string
	version                  int
}

type permanentErr struct{ msg string }

func (e permanentErr) Error() string           { return e.msg }
func (permanentErr) Kind() contracts.ErrorKind { return contracts.Permanent }

// fake keeps the promises of ReadingUpserter: a differing reading is written, an equal one skipped, a key twice
// in one batch is a Permanent error before anything is stored.
type fake struct {
	stored map[key]contracts.ReadingRecord
}

var _ readingkey.ReadingUpserter = (*fake)(nil)

func keyOf(r contracts.ReadingRecord) key {
	return key{r.Name, r.SubjectKind, r.Subject, r.Day.String(), r.Version}
}

func (f *fake) UpsertReadings(_ context.Context, rows []contracts.ReadingRecord) (readingkey.WriteCount, error) {
	var c readingkey.WriteCount
	seen := map[key]bool{}
	for _, r := range rows {
		if seen[keyOf(r)] {
			return readingkey.WriteCount{}, permanentErr{"duplicate key in batch"}
		}
		seen[keyOf(r)] = true
	}
	if f.stored == nil {
		f.stored = map[key]contracts.ReadingRecord{}
	}
	for _, r := range rows {
		old, had := f.stored[keyOf(r)]
		if had && old.RowsSha256 == r.RowsSha256 {
			c.Skipped++
			continue
		}
		f.stored[keyOf(r)] = r
		c.Written++
	}
	return c, nil
}

func TestReadingUpserterContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var u readingkey.ReadingUpserter = &fake{}
	a := contracts.ReadingRecord{Name: "iv", Version: 1, SubjectKind: "instrument", Subject: "1", RowsSha256: "a"}

	got, err := u.UpsertReadings(ctx, nil)
	if err != nil || got != (readingkey.WriteCount{}) {
		t.Fatalf("empty: %+v, %v", got, err)
	}
	got, err = u.UpsertReadings(ctx, []contracts.ReadingRecord{a})
	if err != nil || got != (readingkey.WriteCount{Written: 1}) {
		t.Fatalf("first: %+v, %v", got, err)
	}
	b := a
	b.Subject = "2"
	got, err = u.UpsertReadings(ctx, []contracts.ReadingRecord{a, b})
	if err != nil || got != (readingkey.WriteCount{Written: 1, Skipped: 1}) {
		t.Fatalf("one written one skipped: %+v, %v", got, err)
	}
	_, err = u.UpsertReadings(ctx, []contracts.ReadingRecord{b, b})
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("key twice: %v", err)
	}
}
