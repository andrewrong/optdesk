package reading

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/readingkey"
)

// upsertDB is a fake database whose upsert statement reports a canned count per call, in order.
type upsertDB struct {
	fakeDB
	upsertCounts []int64
	next         int
}

type upsertTx struct{ d *upsertDB }

func (t upsertTx) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	t.d.calls = append(t.d.calls, call{sql, args})
	if sql == upsertReadingSQL {
		n := t.d.upsertCounts[t.d.next]
		t.d.next++
		return n, nil
	}
	return 1, nil
}

func (d *upsertDB) InTx(_ context.Context, fn func(execer) error) error {
	d.events = append(d.events, "begin")
	if err := fn(upsertTx{d}); err != nil {
		d.events = append(d.events, "rollback")
		return err
	}
	d.events = append(d.events, "commit")
	return nil
}

func TestUpsertWritesChildrenOfWrittenReadingOnly(t *testing.T) {
	t.Parallel()
	d := &upsertDB{fakeDB: *newFake(), upsertCounts: []int64{1, 0}}
	a, b := rec(2, "ok"), rec(2, "ok")
	b.Subject = "OTHER"
	got, err := (&readingStore{db: d}).UpsertReadings(context.Background(), []contracts.ReadingRecord{a, b})
	if err != nil || got != (readingkey.WriteCount{Written: 1, Skipped: 1}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if strings.Join(d.events, ",") != "begin,commit" {
		t.Errorf("events = %v, want one transaction", d.events)
	}
	if count(&d.fakeDB, upsertReadingSQL) != 2 {
		t.Errorf("%d upserts, want one per reading", count(&d.fakeDB, upsertReadingSQL))
	}
	for _, sql := range []string{deleteInputKeySQL, deleteInputSQL, deleteParameterSQL, insertInputSQL, insertParameterSQL} {
		if count(&d.fakeDB, sql) != 1 {
			t.Errorf("%q ran %d times, want 1 (the written reading only)", sql, count(&d.fakeDB, sql))
		}
	}
	if count(&d.fakeDB, insertInputKeySQL) != 2 {
		t.Errorf("%d input keys written, want 2", count(&d.fakeDB, insertInputKeySQL))
	}
	// The delete of the children comes after the upsert and before their insert.
	order := ""
	for _, c := range d.calls[:6] {
		switch c.sql {
		case upsertReadingSQL:
			order += "u"
		case deleteInputKeySQL, deleteInputSQL, deleteParameterSQL:
			order += "d"
		default:
			order += "i"
		}
	}
	if order != "uddd"+"ii" {
		t.Errorf("statement order = %s", order)
	}
	if args := d.calls[0].args; args[5] != 2 {
		t.Errorf("rev sent = %v, want 2", args[5])
	}
}

func TestUpsertStatementShape(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"ON CONFLICT (name, version, subject_kind, subject, day) DO UPDATE", "IS DISTINCT FROM",
		"NOT (r.state = 'ok' AND EXCLUDED.state = 'missing_input')", "RETURNING 1",
	} {
		if !strings.Contains(upsertReadingSQL, want) {
			t.Errorf("upsert lacks %q", want)
		}
	}
}

func TestUpsertDuplicateKeyIsPermanentBeforeAnyStatement(t *testing.T) {
	t.Parallel()
	d := &upsertDB{fakeDB: *newFake()}
	_, err := (&readingStore{db: d}).UpsertReadings(context.Background(), []contracts.ReadingRecord{rec(1, "ok"), rec(2, "ok")})
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("error = %v, want Permanent", err)
	}
	if len(d.calls) != 0 || len(d.events) != 0 {
		t.Errorf("%d calls, events %v before the error", len(d.calls), d.events)
	}
}

func TestUpsertEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	d := &upsertDB{fakeDB: *newFake()}
	got, err := (&readingStore{db: d}).UpsertReadings(context.Background(), nil)
	if got != (readingkey.WriteCount{}) || err != nil || len(d.calls) != 0 || len(d.events) != 0 {
		t.Errorf("got %+v, %v, %d calls", got, err, len(d.calls))
	}
}

func TestUpsertDatabaseErrorIsRetryableAndCountsNothing(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.failExec = "readings.reading_parameter"
	d := &failTxDB{fakeDB: f}
	got, err := (&readingStore{db: d}).UpsertReadings(context.Background(), []contracts.ReadingRecord{rec(1, "ok")})
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable || got != (readingkey.WriteCount{}) {
		t.Errorf("got %+v, %v", got, err)
	}
}

// failTxDB runs the plain fake transaction, which returns 1 for every statement.
type failTxDB struct{ *fakeDB }

func TestUpsertSameCalendarDayInAnotherZoneIsPermanent(t *testing.T) {
	t.Parallel()
	d := &upsertDB{fakeDB: *newFake(), upsertCounts: []int64{1, 1}}
	a, b := rec(1, "ok"), rec(2, "ok")
	a.Day = types.SerializableDate{Time: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}
	b.Day = types.SerializableDate{Time: time.Date(2026, 11, 6, 0, 0, 0, 0, time.FixedZone("ET", -5*3600))}
	_, err := (&readingStore{db: d}).UpsertReadings(context.Background(), []contracts.ReadingRecord{a, b})
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent || len(d.calls) != 0 {
		t.Fatalf("err = %v, %d calls; want Permanent before any statement", err, len(d.calls))
	}
}
