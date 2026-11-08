package alertrules_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/alertrules"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const today = contracts.Day("2026-11-06")

func sp(s string) *string { return &s }

func mkDay(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return types.SerializableDate{Time: d}
}

// row is an ok decimal reading of the day, data day dd ("" leaves the parameter out).
func row(t *testing.T, day, value, dd string) contracts.ReadingRecord {
	t.Helper()
	r := contracts.ReadingRecord{
		Day: mkDay(t, day), State: "ok", Value: sp(value),
		Unit: "decimal", Rev: 1,
	}
	if dd != "" {
		setDataDay(t, &r, dd)
	}
	return r
}

func setDataDay(t *testing.T, r *contracts.ReadingRecord, dd string) {
	t.Helper()
	b, _ := json.Marshal([]map[string]any{{"param": "data_day", "value_num": nil, "value_tag": dd}})
	if err := json.Unmarshal(b, &r.Parameters); err != nil {
		t.Fatal(err)
	}
}

func rule(op, thr string) compute.AlertRules {
	return compute.AlertRules{
		Rules: []compute.AlertRule{{
			ID: "r1", Title: "T", Figure: "market.vix_pctile_3y", FigureVersion: 1,
			SubjectKind: "market", Subject: "VIXCLS", Unit: "decimal", Op: op,
			Threshold: thr, Level: "w", MaxDataAgeDays: 6,
		}},
		PublicSubjects: []string{"VIXCLS"},
	}
}

func fixed(rows ...contracts.ReadingRecord) compute.ReadingFunc {
	return func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return rows, nil
	}
}

func one(t *testing.T, rules compute.AlertRules, read compute.ReadingFunc) compute.AlertResult {
	t.Helper()
	res, err := alertrules.Evaluate(context.Background(), rules, read, today)
	if err != nil || len(res) != 1 {
		t.Fatalf("err %v, results %d", err, len(res))
	}
	return res[0]
}

func TestEvalFires(t *testing.T) {
	t.Parallel()
	r := one(t, rule("ge", "0.90"), fixed(row(t, "2026-11-06", "0.95", "2026-11-05")))
	if r.State != "fired" || r.Reason != "" || r.Value != "0.95" || r.Day != "2026-11-06" || r.DataDay != "2026-11-05" {
		t.Fatalf("%+v", r)
	}
	if r.RuleID != "r1" || r.Level != "w" || r.Figure != "market.vix_pctile_3y" {
		t.Fatalf("%+v", r)
	}
}

func TestEvalClear(t *testing.T) {
	t.Parallel()
	r := one(t, rule("ge", "0.90"), fixed(row(t, "2026-11-06", "0.50", "2026-11-06")))
	if r.State != "clear" || r.Reason != "" {
		t.Fatalf("%+v", r)
	}
}

func TestEvalBoundary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		op, val string
		want    string
	}{
		{"gt", "0.90", "clear"}, {"ge", "0.90", "fired"}, {"lt", "0.90", "clear"}, {"le", "0.90", "fired"},
		{"gt", "0.91", "fired"}, {"lt", "0.89", "fired"},
	}
	for _, c := range cases {
		r := one(t, rule(c.op, "0.90"), fixed(row(t, "2026-11-06", c.val, "2026-11-06")))
		if r.State != c.want {
			t.Errorf("%s %s: %s, want %s", c.op, c.val, r.State, c.want)
		}
	}
}

func TestEvalMissingInputNoData(t *testing.T) {
	t.Parallel()
	m := row(t, "2026-11-06", "", "")
	m.State, m.Value = "missing_input", nil
	r := one(t, rule("ge", "0.9"), fixed(m))
	if r.State != "no_data" || r.Reason != "missing_input" || r.Day != "2026-11-06" {
		t.Fatalf("%+v", r)
	}
}

func TestEvalUnitMismatchNoData(t *testing.T) {
	t.Parallel()
	x := row(t, "2026-11-06", "0.95", "2026-11-06")
	x.Unit = "ratio"
	r := one(t, rule("ge", "0.9"), fixed(x))
	if r.State != "no_data" || r.Reason != "unit_mismatch" {
		t.Fatalf("%+v", r)
	}
}

func TestEvalOldRowNoData(t *testing.T) {
	t.Parallel()
	r := one(t, rule("ge", "0.9"), fixed(row(t, "2026-10-30", "0.95", "2026-10-30")))
	if r.State != "no_data" || r.Reason != "no_reading" {
		t.Fatalf("%+v", r)
	}
	r = one(t, rule("ge", "0.9"), fixed())
	if r.State != "no_data" || r.Reason != "no_reading" {
		t.Fatalf("%+v", r)
	}
}

func TestEvalStaleDataDayNoData(t *testing.T) {
	t.Parallel()
	r := one(t, rule("ge", "0.9"), fixed(row(t, "2026-11-06", "0.95", "2026-10-30")))
	if r.State != "no_data" || r.Reason != "stale_data" {
		t.Fatalf("%+v", r)
	}
}

func TestEvalMissingDataDayNoData(t *testing.T) {
	t.Parallel()
	for _, dd := range []string{"", "not a day", "2026-13-40"} {
		x := row(t, "2026-11-06", "0.95", "")
		if dd != "" {
			setDataDay(t, &x, dd)
		}
		r := one(t, rule("ge", "0.9"), fixed(x))
		if r.State != "no_data" || r.Reason != "no_data_day" {
			t.Errorf("%q: %+v", dd, r)
		}
	}
}

func TestEvalLateFredStillFreshEnough(t *testing.T) {
	t.Parallel()
	r := one(t, rule("ge", "0.9"), fixed(row(t, "2026-11-06", "0.95", "2026-10-31")))
	if r.State != "fired" || r.DataDay != "2026-10-31" {
		t.Fatalf("6 days old: %+v", r)
	}
}

func TestEvalDetailFixedWords(t *testing.T) {
	t.Parallel()
	rules := rule("ge", "0.90")
	rules.Rules[0].Title = "free text from the file"
	r := one(t, rules, fixed(row(t, "2026-11-06", "0.95", "2026-11-06")))
	if want := "market.vix_pctile_3y VIXCLS 0.95 at or above 0.90"; r.Detail != want {
		t.Fatalf("detail %q, want %q", r.Detail, want)
	}
	if strings.Contains(r.Detail, "free text") {
		t.Fatal("detail holds file text")
	}
	m := row(t, "2026-11-06", "", "")
	m.State, m.Value = "missing_input", nil
	if d := one(t, rules, fixed(m)).Detail; d != "" {
		t.Fatalf("no_data detail %q", d)
	}
}

func TestEvalExactDecimal(t *testing.T) {
	t.Parallel()
	// 0.1 + 0.2 style traps: a float compare of these decimals gets at least one wrong.
	r := one(t, rule("gt", "0.3"), fixed(row(t, "2026-11-06", "0.30000000000000004", "2026-11-06")))
	if r.State != "fired" {
		t.Fatalf("%+v", r)
	}
	r = one(t, rule("ge", "0.3"), fixed(row(t, "2026-11-06", "0.300000000000", "2026-11-06")))
	if r.State != "fired" {
		t.Fatalf("%+v", r)
	}
	r = one(t, rule("lt", "-0.05"), fixed(row(t, "2026-11-06", "-0.050000000001", "2026-11-06")))
	if r.State != "fired" {
		t.Fatalf("%+v", r)
	}
	r = one(t, rule("lt", "0"), fixed(row(t, "2026-11-06", "-0", "2026-11-06")))
	if r.State != "clear" {
		t.Fatalf("%+v", r)
	}
}

func TestEvalPublicSubject(t *testing.T) {
	t.Parallel()
	rules := rule("ge", "0.9")
	rd := fixed(row(t, "2026-11-06", "0.95", "2026-11-06"))
	if !one(t, rules, rd).Public {
		t.Fatal("VIXCLS is public")
	}
	rules.PublicSubjects = []string{"SPY"}
	if one(t, rules, rd).Public {
		t.Fatal("VIXCLS is not public")
	}
}

func TestEvalReaderErrorIsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	read := func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return nil, boom
	}
	res, err := alertrules.Evaluate(context.Background(), rule("ge", "0.9"), read, today)
	if !errors.Is(err, boom) || res != nil {
		t.Fatalf("res %v err %v", res, err)
	}
}

func TestEvalNewestRowAndWindow(t *testing.T) {
	t.Parallel()
	var gotFrom, gotTo contracts.Day
	older := row(t, "2026-11-04", "0.99", "2026-11-04")
	newer := row(t, "2026-11-05", "0.10", "2026-11-05")
	newer2 := row(t, "2026-11-05", "0.95", "2026-11-05")
	newer2.Rev = 2
	read := func(_ context.Context, name, kind, subj string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
		gotFrom, gotTo = from, to
		if name != "market.vix_pctile_3y" || kind != "market" || subj != "VIXCLS" {
			t.Errorf("read %s %s %s", name, kind, subj)
		}
		return []contracts.ReadingRecord{older, newer, newer2}, nil
	}
	r := one(t, rule("ge", "0.9"), read)
	if gotFrom != "2026-11-01" || gotTo != today {
		t.Fatalf("window %s..%s", gotFrom, gotTo)
	}
	if r.Day != "2026-11-05" || r.Value != "0.95" {
		t.Fatalf("%+v", r)
	}
}
