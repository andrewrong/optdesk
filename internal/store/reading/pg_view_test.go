//go:build pg

package reading

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/readingkey"
)

func dataDayRec(rev int, state, version string) contracts.ReadingRecord {
	r := rec(rev, state)
	r.Parameters[0].Param, r.Parameters[0].ValueTag = "data_day", strPtr(version)
	if state != "ok" {
		r.Value = nil
	}
	return r
}

func TestViewAgainstRoPages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := Open(ctx, connURL(t, name, "ot_w_readings"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	// ok rev 1 stays when a missing_input rev 2 of the same key is written: the upsert skips it.
	if _, err := w.InsertReadings(ctx, []contracts.ReadingRecord{dataDayRec(1, "ok", "2026-11-05")}); err != nil {
		t.Fatal(err)
	}
	mustUpsert(t, w, readingkey.WriteCount{Skipped: 1}, dataDayRec(2, "missing_input", "2026-11-04"))
	v, err := OpenView(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = v.Close() }()
	got, err := v.WinnerReadings(ctx, "market.ivp_own", "market", "VIXCLS", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 1 || got[0].Rev != 1 || string(got[0].State) != "ok" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if len(got[0].Parameters) != 1 || *got[0].Parameters[0].ValueTag != "2026-11-05" || len(got[0].Inputs) != 0 {
		t.Errorf("parameters %+v inputs %+v", got[0].Parameters, got[0].Inputs)
	}
	// A version bump leaves one row for the day: version 2.
	bumped := dataDayRec(1, "ok", "2026-11-06")
	bumped.Version = 2
	if _, err := w.InsertReadings(ctx, []contracts.ReadingRecord{bumped}); err != nil {
		t.Fatal(err)
	}
	got, err = v.WinnerReadings(ctx, "market.ivp_own", "market", "VIXCLS", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 1 || got[0].Version != 2 || *got[0].Parameters[0].ValueTag != "2026-11-06" {
		t.Fatalf("after bump got %+v, %v", got, err)
	}
	// The same reader works as ot_w_readings, and a table stays closed to the page role.
	wv, err := OpenView(ctx, connURL(t, name, "ot_w_readings"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wv.Close() }()
	if got, err := wv.WinnerReadings(ctx, "market.ivp_own", "market", "VIXCLS", "2026-11-01", "2026-11-30"); err != nil || len(got) != 1 {
		t.Errorf("as ot_w_readings: %+v, %v", got, err)
	}
	if _, err := pgxAs(t, name, "ot_ro_pages").Exec(ctx, `SELECT 1 FROM readings.reading_parameter`); err == nil {
		t.Error("ot_ro_pages may read the parameter table")
	}
}

func TestViewParameterDataDay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := Open(ctx, connURL(t, name, "ot_w_readings"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if _, err := w.InsertReadings(ctx, []contracts.ReadingRecord{dataDayRec(1, "ok", "2026-11-05")}); err != nil {
		t.Fatal(err)
	}
	v, err := OpenView(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = v.Close() }()
	got, err := v.WinnerReadings(ctx, "market.ivp_own", "market", "VIXCLS", "2026-11-06", "2026-11-06")
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	p := got[0].Parameters
	if len(p) != 1 || p[0].Param != "data_day" || p[0].ValueTag == nil || *p[0].ValueTag != "2026-11-05" {
		t.Errorf("parameters = %+v", p)
	}
}
