package main

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// memReadings is a fake reading store: it answers LatestReadings from rows (the newest rev of each day, by name
// and subject, in the range) and keeps what InsertReadings is given.
type memReadings struct {
	mu     sync.Mutex
	rows   []contracts.ReadingRecord
	closed bool
}

func (m *memReadings) LatestReadings(_ context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	newest := map[string]contracts.ReadingRecord{}
	var days []string
	for _, r := range m.rows {
		d := r.Day.Format(dayLayout)
		if r.Name != name || r.SubjectKind != kind || r.Subject != subject || d < string(from) || d > string(to) {
			continue
		}
		if old, ok := newest[d]; !ok {
			days = append(days, d)
			newest[d] = r
		} else if r.Rev > old.Rev {
			newest[d] = r
		}
	}
	sort.Strings(days)
	out := make([]contracts.ReadingRecord, 0, len(days))
	for _, d := range days {
		out = append(out, newest[d])
	}
	return out, nil
}

func (m *memReadings) InsertReadings(_ context.Context, rows []contracts.ReadingRecord) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = append(m.rows, rows...)
	return len(rows), nil
}

func (m *memReadings) Close() error { m.closed = true; return nil }

var _ store.ReadingStore = (*memReadings)(nil)

func bandRow(symbol, day, value string, rev int, state string) contracts.ReadingRecord {
	d, _ := time.Parse(dayLayout, day)
	r := contracts.ReadingRecord{
		Name: "valuation.buy_band", Version: 1, SubjectKind: "underlying", Subject: symbol,
		Day: types.SerializableDate{Time: d}, Rev: rev, Currency: "USD", Origin: "compute_figures", Unit: "money",
	}
	r.State = "ok"
	if state == "ok" {
		r.Value = &value
	} else {
		r.State = "missing_input"
	}
	return r
}

func approveApp(t *testing.T, rd *memReadings) *app {
	t.Helper()
	a, _ := testApp(t, "OT_PG_DSN_READINGS=readings-dsn\n")
	a.deps.newClock = func() clock.Clock { return fakeClock{time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)} }
	a.deps.computeRules = newDeps().computeRules
	a.deps.openReading = func(context.Context, string) (store.ReadingStore, error) { return rd, nil }
	return a
}

func TestApproveBandWritesApproval(t *testing.T) {
	t.Parallel()
	rd := &memReadings{rows: []contracts.ReadingRecord{
		bandRow("AAA", "2026-11-04", "120.0000", 1, "ok"),
		bandRow("AAA", "2026-11-05", "142.5000", 1, "ok"),
		bandRow("BBB", "2026-11-05", "9.0000", 1, "ok"),
	}}
	a := approveApp(t, rd)
	out := &strings.Builder{}
	a.stdout = out
	if err := approveBand(a, []string{"--symbol", "AAA", "--day", "2026-11-06"}); err != nil {
		t.Fatal(err)
	}
	if err := approveBand(a, []string{"--symbol", "AAA"}); err != nil { // day defaults to today: 2026-11-06 in New York
		t.Fatal(err)
	}
	var got []contracts.ReadingRecord
	for _, r := range rd.rows {
		if r.Name == "valuation.band_approval" {
			got = append(got, r)
		}
	}
	if len(got) != 2 || !rd.closed {
		t.Fatalf("approvals %d, closed %v", len(got), rd.closed)
	}
	for i, r := range got {
		if msg := checkApproval(r, i+1); msg != "" {
			t.Errorf("approval %d: %s: %+v", i, msg, r)
		}
	}
	if !strings.Contains(out.String(), "approved AAA 2026-11-06 142.5000 rev 2") {
		t.Errorf("stdout %q", out)
	}
}

func TestApproveBandRefusesNoBand(t *testing.T) {
	t.Parallel()
	rd := &memReadings{rows: []contracts.ReadingRecord{
		bandRow("MISS", "2026-11-05", "", 1, "missing_input"),
		bandRow("LATE", "2026-11-09", "50.0000", 1, "ok"), // after the approval day
	}}
	for _, symbol := range []string{"NONE", "MISS", "LATE"} {
		a := approveApp(t, rd)
		err := approveBand(a, []string{"--symbol", symbol, "--day", "2026-11-06"})
		if err == nil || !strings.Contains(err.Error(), "no band") {
			t.Errorf("%s: err %v", symbol, err)
		}
	}
	if len(rd.rows) != 2 {
		t.Fatalf("rows written: %d", len(rd.rows)-2)
	}
	if err := approveBand(approveApp(t, rd), nil); err == nil {
		t.Error("no --symbol was accepted")
	}
}

// checkApproval names the first field of the approval of AAA on 2026-11-06 that is not as the band 142.5000 gives it.
func checkApproval(r contracts.ReadingRecord, rev int) string {
	switch {
	case r.Subject != "AAA" || r.SubjectKind != "underlying":
		return "subject"
	case r.Origin != "owner_command" || r.State != "ok":
		return "origin or state"
	case r.Value == nil || *r.Value != "142.5000":
		return "value"
	case r.Day.Format(dayLayout) != "2026-11-06":
		return "day"
	case r.Rev != rev:
		return "rev"
	case r.Currency != "USD" || r.Version != 1:
		return "currency or version"
	case len(r.RowsSha256) != 64 || len(r.Inputs) != 1:
		return "provenance"
	}
	return ""
}

func TestApproveBandWithholdsDSN(t *testing.T) {
	t.Parallel()
	a := approveApp(t, &memReadings{})
	a.deps.openReading = func(context.Context, string) (store.ReadingStore, error) {
		return nil, errors.New("dial readings-dsn failed")
	}
	err := approveBand(a, []string{"--symbol", "AAA"})
	if err == nil || strings.Contains(err.Error(), "readings-dsn") {
		t.Fatalf("err %v", err)
	}
}
