//go:build pg

package optionopenpg

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// listStore makes a database, applies 0034_option_list.sql and the band columns of 0035 to it and opens the writer on it.
func listStore(t *testing.T) openquote.OptionListStore {
	t.Helper()
	ctx := context.Background()
	name := newDatabase(t)
	raw, err := os.ReadFile("../../../migrations/0034_option_list.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0034: %v", err)
	}
	// 0035 also makes ops and rd objects this database lacks: only its first statement, the band columns, applies here.
	raw, err = os.ReadFile("../../../migrations/0035_option_list_band_run_view.sql")
	if err != nil {
		t.Fatal(err)
	}
	band, _, _ := strings.Cut(string(raw), "-- ops.option_run_reason:")
	if _, err := c.Exec(ctx, strings.Replace(band, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0035 band columns: %v", err)
	}
	w, err := Open(ctx, connURL(t, name, "ot_w_option_open"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	ls, ok := w.(openquote.OptionListStore)
	if !ok {
		t.Fatal("the store is not an OptionListStore")
	}
	return ls
}

func TestListRowsKeepListingOnShortAndRunExpectedGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := listStore(t)
	at := time.Date(2026, 11, 12, 11, 20, 0, 0, time.UTC)
	later := at.Add(24 * time.Hour)
	rec := openquote.ListRecord{UnderlyingID: 1, Expiry: "2026-12-18", Class: "AAPL", ListedAt: at, ListedSession: "2026-11-12",
		Count: 516, TriedAt: at, Outcome: "listed"}
	if err := s.PutLists(ctx, []openquote.ListRecord{rec}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutLists(ctx, []openquote.ListRecord{{UnderlyingID: 1, Expiry: "2026-12-18", Class: "AAPL", TriedAt: later, Outcome: "short"}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Lists(ctx, 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	want := rec
	want.TriedAt, want.Outcome = later, "short"
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("got %+v want %+v", got[0], want)
	}
	// A first attempt that is short has no listing.
	if err := s.PutLists(ctx, []openquote.ListRecord{{UnderlyingID: 1, Expiry: "2026-12-18", Class: "2AAPL", TriedAt: at, Outcome: "error"}}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Lists(ctx, 1)
	if len(got) != 2 || !got[0].ListedAt.IsZero() || got[0].ListedSession != "" {
		t.Errorf("never listed row = %+v", got)
	}

	run := func(exp []string, listed int, reason string) openquote.ListRunRecord {
		return openquote.ListRunRecord{UnderlyingID: 1, Session: "2026-11-12", Expected: exp, Listed: listed, Reason: reason, UpdatedAt: at}
	}
	steps := []struct {
		write openquote.ListRunRecord
		want  []string
	}{
		{run(nil, 0, "not_finished"), nil},
		{run([]string{}, 0, "no_expiry_in_window"), []string{}},
		{run([]string{"2026-12-18|AAPL"}, 0, "not_finished"), []string{"2026-12-18|AAPL"}},
		{run([]string{"2027-01-15|AAPL"}, 1, "none"), []string{"2026-12-18|AAPL"}},
		{run(nil, 1, "none"), []string{"2026-12-18|AAPL"}},
	}
	for i, st := range steps {
		if err := s.PutListRuns(ctx, []openquote.ListRunRecord{st.write}); err != nil {
			t.Fatal(err)
		}
		rows, err := s.ListRuns(ctx, "2026-11-12")
		if err != nil || len(rows) != 1 {
			t.Fatalf("step %d: %+v, %v", i, rows, err)
		}
		if !reflect.DeepEqual(rows[0].Expected, st.want) || rows[0].Reason != st.write.Reason {
			t.Errorf("step %d: expected %#v reason %s, want %#v %s", i, rows[0].Expected, rows[0].Reason, st.want, st.write.Reason)
		}
	}
}

func TestRunListedEqualsCountOfListRowsOfSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := listStore(t)
	at := time.Date(2026, 11, 12, 11, 20, 0, 0, time.UTC)
	var rows []openquote.ListRecord
	for _, e := range []string{"2026-12-11", "2026-12-18"} {
		rows = append(rows, openquote.ListRecord{UnderlyingID: 1, Expiry: contracts.Day(e), Class: "AAPL",
			ListedAt: at, ListedSession: "2026-11-12", Count: 10, TriedAt: at, Outcome: "listed"})
	}
	rows = append(rows, openquote.ListRecord{UnderlyingID: 1, Expiry: "2026-12-24", Class: "AAPL", ListedAt: at, ListedSession: "2026-11-11",
		Count: 10, TriedAt: at, Outcome: "listed"})
	if err := s.PutLists(ctx, rows); err != nil {
		t.Fatal(err)
	}
	listed := 0
	got, _ := s.Lists(ctx, 1)
	for _, r := range got {
		if r.ListedSession == "2026-11-12" {
			listed++
		}
	}
	if err := s.PutListRuns(ctx, []openquote.ListRunRecord{{UnderlyingID: 1, Session: "2026-11-12", Expected: []string{"a|b"},
		Listed: listed, Reason: "some_short", UpdatedAt: at}}); err != nil {
		t.Fatal(err)
	}
	runs, err := s.ListRuns(ctx, "2026-11-12")
	if err != nil || len(runs) != 1 || runs[0].Listed != listed || listed != 2 {
		t.Fatalf("runs %+v listed %d, %v", runs, listed, err)
	}
	if other, _ := s.ListRuns(ctx, "2026-11-11"); len(other) != 0 {
		t.Errorf("other session: %+v", other)
	}
}

func TestBandRoundTripAndShortKeepsItPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := listStore(t)
	at := time.Date(2026, 11, 12, 11, 20, 0, 0, time.UTC)
	rec := openquote.ListRecord{UnderlyingID: 1, Expiry: "2026-12-18", Class: "AAPL", ListedAt: at, ListedSession: "2026-11-12",
		Count: 516, TriedAt: at, Outcome: "listed", BandLo: "180.50", BandHi: "220", Sigma: 0.31, SigmaSource: "iv", InBand: 120}
	if err := s.PutLists(ctx, []openquote.ListRecord{rec}); err != nil {
		t.Fatal(err)
	}
	failed := openquote.ListRecord{UnderlyingID: 1, Expiry: "2026-12-18", Class: "AAPL", TriedAt: at.Add(time.Hour), Outcome: "error"}
	if err := s.PutLists(ctx, []openquote.ListRecord{failed}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Lists(ctx, 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	want := rec
	want.TriedAt, want.Outcome = at.Add(time.Hour), "error"
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("got %+v want %+v", got[0], want)
	}
	// A listing with an inverted band is refused by the check.
	bad := rec
	bad.Class, bad.BandLo, bad.BandHi = "BAD", "300", "200"
	if err := s.PutLists(ctx, []openquote.ListRecord{bad}); err == nil {
		t.Error("band_lo > band_hi was stored")
	}
}
