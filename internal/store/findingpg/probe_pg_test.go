//go:build pg

package findingpg

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

// probeTables replace the empty stub views of newDatabase with tables that have the columns the probe reads.
var probeTables = map[string]string{
	"reading_r1":          `name text, version int, subject_kind text, subject text, day date, state text, value numeric`,
	"option_open_r1":      `session_day date, currency text, received_at timestamptz`,
	"option_candidate_r1": `session_day date, run_id text, kind text, computed_at timestamptz, rule_version text`,
	"rule_param_r1":       `set_at timestamptz`,
	"option_exclusion_r1": `set_at timestamptz`,
	"job_gap_r1":          `job_id text, dataset_id text, day date, reason text, first_seen_at timestamptz, cleared_at timestamptz`,
}

// openProbe returns a probe over a fresh database and an administrator connection that fills the tables.
func openProbe(t *testing.T) (finding.Probe, *pgx.Conn) {
	t.Helper()
	ctx := context.Background()
	name := newDatabase(t)
	admin, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(ctx) })
	for v, cols := range probeTables {
		for _, q := range []string{
			`DROP VIEW rd.` + v,
			`CREATE TABLE rd.` + v + ` (` + cols + `)`,
			`GRANT SELECT ON rd.` + v + ` TO ot_w_datacheck`,
		} {
			if _, err := admin.Exec(ctx, q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
	}
	p, err := OpenProbe(ctx, connURL(t, name, "ot_w_datacheck"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, admin
}

func exec(t *testing.T, c *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func reading(t *testing.T, c *pgx.Conn, name string, version int, subject, day, state string, value any) {
	t.Helper()
	if f, ok := value.(float64); ok {
		value = strconv.FormatFloat(f, 'f', -1, 64)
	}
	exec(t, c, `INSERT INTO rd.reading_r1 VALUES ($1, $2, 'underlying', $3, $4::text::date, $5, $6::text::numeric)`, name, version, subject, day, state, value)
}

func TestRepeatedReadingsHighestVersion(t *testing.T) {
	t.Parallel()
	p, c := openProbe(t)
	// Day 1: version 1 values 1,2,3; day 2: version 1 repeats all three, version 2 repeats only one.
	for i, s := range []string{"a", "b", "c"} {
		v := float64(i + 1)
		reading(t, c, "market.rv20", 1, s, "2026-11-23", "ok", v)
		reading(t, c, "market.rv20", 1, s, "2026-11-24", "ok", v)
	}
	reading(t, c, "market.rv20", 2, "a", "2026-11-24", "ok", 1.0)
	reading(t, c, "market.rv20", 2, "b", "2026-11-24", "ok", 9.0)
	reading(t, c, "other", 1, "a", "2026-11-24", "ok", 1.0)

	got, err := p.RepeatedReadings(context.Background(), []string{"market.rv20"}, "2026-11-24", "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	// Version 2 is the highest of 24th: a repeats day 23 (version 1) by value, b does not.
	want := []finding.ReadingRepeat{{Name: "market.rv20", Day: "2026-11-24", OK: 2, Same: 1}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRepeatedReadingsSkipsMissingInput(t *testing.T) {
	t.Parallel()
	p, c := openProbe(t)
	reading(t, c, "market.rv20", 1, "a", "2026-11-20", "ok", 5.0)
	reading(t, c, "market.rv20", 1, "a", "2026-11-23", "missing_input", nil)
	reading(t, c, "market.rv20", 1, "a", "2026-11-24", "ok", 5.0)
	reading(t, c, "market.rv20", 1, "b", "2026-11-24", "missing_input", nil)

	got, err := p.RepeatedReadings(context.Background(), []string{"market.rv20"}, "2026-11-23", "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	// The 23rd has no ok row; the 24th compares a with the 20th, the previous day that has an ok row; b is not counted.
	if len(got) != 1 || got[0] != (finding.ReadingRepeat{Name: "market.rv20", Day: "2026-11-24", OK: 1, Same: 1}) {
		t.Fatalf("got %+v", got)
	}
	none, err := p.RepeatedReadings(context.Background(), nil, "2026-11-23", "2026-11-24")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("no names: %v, %v", none, err)
	}
}

func TestLateOpenQuotesZones(t *testing.T) {
	t.Parallel()
	p, c := openProbe(t)
	ins := `INSERT INTO rd.option_open_r1 VALUES ($1::text::date, $2, $3::text::timestamptz)`
	// JPY session 2026-11-24, received 09:17 Tokyo the next day (00:17 UTC): late.
	exec(t, c, ins, "2026-11-24", "JPY", "2026-11-25 09:17:00+09")
	// JPY received the same Tokyo day: not late.
	exec(t, c, ins, "2026-11-24", "JPY", "2026-11-24 15:00:00+09")
	// USD received 22:00 New York the same day (03:00 UTC next day): not late.
	exec(t, c, ins, "2026-11-24", "USD", "2026-11-24 22:00:00-05")
	// EUR has no zone: counted, never late.
	exec(t, c, ins, "2026-11-24", "EUR", "2026-11-30 10:00:00+00")

	got, err := p.LateOpenQuotes(context.Background(), "2026-11-24", "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	want := []finding.LateQuotes{
		{Day: "2026-11-24", Currency: "EUR", Rows: 1, Late: 0},
		{Day: "2026-11-24", Currency: "JPY", Rows: 2, Late: 1},
		{Day: "2026-11-24", Currency: "USD", Rows: 1, Late: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}

func TestNewestCandidateRuns(t *testing.T) {
	t.Parallel()
	p, c := openProbe(t)
	ins := `INSERT INTO rd.option_candidate_r1 VALUES ($1::text::date, $2, $3, $4::text::timestamptz, $5)`
	exec(t, c, ins, "2026-11-24", "r1", "put", "2026-11-24 21:00:00+00", "put_v1")
	exec(t, c, ins, "2026-11-24", "r2", "put", "2026-11-24 22:00:00+00", "put_v2")
	exec(t, c, ins, "2026-11-24", "r2", "put", "2026-11-24 22:00:00+00", "put_v3")
	exec(t, c, ins, "2026-11-24", "r1", "call", "2026-11-24 21:00:00+00", "put_v2")
	exec(t, c, ins, "2026-11-20", "r0", "put", "2026-11-20 21:00:00+00", "put_v1")

	got, err := p.NewestCandidateRuns(context.Background(), "2026-11-24", "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	want := []finding.CandidateRun{
		{Day: "2026-11-24", Kind: "call", Rows: 1, ComputedAt: at("2026-11-24T21:00:00Z"), RuleVersion: "put_v2"},
		{Day: "2026-11-24", Kind: "put", Rows: 2, ComputedAt: at("2026-11-24T22:00:00Z"), RuleVersion: ""},
	}
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].Day != want[i].Day || got[i].Kind != want[i].Kind || got[i].Rows != want[i].Rows || got[i].RuleVersion != want[i].RuleVersion || !got[i].ComputedAt.Equal(want[i].ComputedAt) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}

func TestSettingsChangedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, c := openProbe(t)
	if _, ok, err := p.SettingsChangedAt(ctx); err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}
	exec(t, c, `INSERT INTO rd.rule_param_r1 VALUES ('2026-11-20 10:00:00+00')`)
	exec(t, c, `INSERT INTO rd.option_exclusion_r1 VALUES ('2026-11-22 10:00:00+00')`)
	at, ok, err := p.SettingsChangedAt(ctx)
	if err != nil || !ok || !at.Equal(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("at=%v ok=%v err=%v", at, ok, err)
	}
}

func TestOpenGapsOpenOnlyAscending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, c := openProbe(t)
	empty, err := p.OpenGaps(ctx)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty: %v, %v", empty, err)
	}
	ins := `INSERT INTO rd.job_gap_r1 VALUES ($1, $2, $3::text::date, 'missing', $4::text::timestamptz, $5::text::timestamptz)`
	exec(t, c, ins, "j2", "ds2", "2026-11-23", "2026-11-25 00:00:00+00", nil)
	exec(t, c, ins, "j1", "ds1", "2026-11-22", "2026-11-24 00:00:00+00", nil)
	exec(t, c, ins, "j3", "ds3", "2026-11-21", "2026-11-23 00:00:00+00", "2026-11-24 00:00:00+00")

	got, err := p.OpenGaps(ctx)
	if err != nil || len(got) != 2 || got[0].JobID != "j1" || got[1].JobID != "j2" ||
		got[0].Dataset != "ds1" || got[0].Day != contracts.Day("2026-11-22") || got[0].Reason != "missing" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
