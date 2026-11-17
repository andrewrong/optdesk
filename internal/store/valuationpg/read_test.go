package valuationpg

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// day0 is the fixed base of every time in these tests; the numbers are made up.
var day0 = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

func at(minutes int) time.Time { return day0.Add(time.Duration(minutes) * time.Minute) }

// reviewRow is one row of rd.valuation_review_r1 in the order of reviewColumns; effMin < 0 means no effective_at.
func reviewRow(id int64, reviewID, company, typ, status, versionDate string, effMin int) []driver.Value {
	row := make([]driver.Value, 30)
	row[0], row[1], row[2], row[3], row[4] = id, reviewID, company, typ, versionDate
	row[7], row[8], row[9], row[10] = status, "owner", "10", "3"
	row[28] = at(0)
	if effMin >= 0 {
		row[29] = at(effMin)
	}
	return row
}

func profileRow(id int64, key, name string, setMin int) []driver.Value {
	return []driver.Value{id, key, name, "SYM", "USD", "US", "in_pool", nil, "", at(setMin)}
}

// views answers each view by the name in the statement.
type views struct {
	profiles, closing, company, rvers, rtable, margin [][]driver.Value
}

func (v views) answer(q string, _ []driver.NamedValue) [][]driver.Value {
	switch {
	case strings.Contains(q, "rd.valuation_company_profile_r1"):
		return v.profiles
	case strings.Contains(q, "rd.valuation_review_r1") && strings.Contains(q, "status IN"):
		return v.closing
	case strings.Contains(q, "rd.valuation_review_r1"):
		return v.company
	case strings.Contains(q, "rd.valuation_r_version_r1"):
		return v.rvers
	case strings.Contains(q, "rd.valuation_r_table_r1"):
		return v.rtable
	case strings.Contains(q, "rd.valuation_margin_table_r1"):
		return v.margin
	}
	return nil
}

func current(t *testing.T, v views) valuation.State {
	t.Helper()
	rec := &recorder{answer: v.answer}
	st, err := rec.store().Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func currentID(t *testing.T, v views) int64 {
	t.Helper()
	st := current(t, v)
	if len(st.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(st.Entries))
	}
	if st.Entries[0].Current == nil {
		return 0
	}
	return st.Entries[0].Current.ID
}

func TestCurrentValuationRule(t *testing.T) {
	t.Parallel()
	prof := [][]driver.Value{profileRow(1, "US:AAA", "Aaa", 0)}

	// The later effective_at wins whatever the version dates say.
	v := views{profiles: prof, closing: [][]driver.Value{
		reviewRow(1, "a", "US:AAA", "first", "effective", "2026-02-01", 10),
		reviewRow(2, "b", "US:AAA", "event", "effective", "2025-01-01", 20),
	}}
	if got := currentID(t, v); got != 2 {
		t.Errorf("later effective_at, earlier version date: current = %d, want 2", got)
	}

	// Equal effective_at: the later id.
	v.closing = [][]driver.Value{
		reviewRow(5, "a", "US:AAA", "first", "effective", "2026-02-01", 10),
		reviewRow(6, "b", "US:AAA", "event", "effective", "2026-03-01", 10),
	}
	if got := currentID(t, v); got != 6 {
		t.Errorf("equal effective_at: current = %d, want the later id 6", got)
	}

	// A draft is never current: the closing statement never returns one, and a draft that slipped in is ignored.
	v.closing = [][]driver.Value{reviewRow(7, "d", "US:AAA", "first", "draft", "2026-02-01", -1)}
	if got := currentID(t, v); got != 0 {
		t.Errorf("draft only: current = %d, want none", got)
	}

	// An effective review with a void row is not current; the earlier effective review is.
	v.closing = [][]driver.Value{
		reviewRow(1, "a", "US:AAA", "first", "effective", "2026-02-01", 10),
		reviewRow(2, "b", "US:AAA", "event", "effective", "2026-02-02", 20),
		reviewRow(3, "b", "US:AAA", "event", "void", "2026-02-02", -1),
	}
	if got := currentID(t, v); got != 1 {
		t.Errorf("latest voided: current = %d, want the earlier effective review 1", got)
	}

	// Every effective review voided: none.
	v.closing = append(v.closing, reviewRow(4, "a", "US:AAA", "first", "void", "2026-02-01", -1))
	if got := currentID(t, v); got != 0 {
		t.Errorf("all voided: current = %d, want none", got)
	}

	// Reviews of another company do not count.
	v.closing = [][]driver.Value{reviewRow(9, "z", "US:ZZZ", "first", "effective", "2026-02-01", 30)}
	if got := currentID(t, v); got != 0 {
		t.Errorf("review of another company: current = %d, want none", got)
	}
}

func TestBackfillNeverCurrent(t *testing.T) {
	t.Parallel()
	v := views{
		profiles: [][]driver.Value{profileRow(1, "US:AAA", "Aaa", 0)},
		closing: [][]driver.Value{
			reviewRow(1, "a", "US:AAA", "first", "effective", "2026-02-01", 10),
			reviewRow(2, "b", "US:AAA", "backfill", "effective", "2027-01-01", 99),
		},
	}
	if got := currentID(t, v); got != 1 {
		t.Errorf("late backfill: current = %d, want 1", got)
	}
	v.closing = v.closing[1:]
	if got := currentID(t, v); got != 0 {
		t.Errorf("backfill only: current = %d, want none", got)
	}
}

func TestNewestTableVersionWins(t *testing.T) {
	t.Parallel()
	rt := func(date string, setMin int, yield string) []driver.Value {
		return []driver.Value{date, at(setMin), yield, "1.1", "1.2", "1.3"}
	}
	mt := func(date string, setMin int, pFrom string) []driver.Value {
		return []driver.Value{date, at(setMin), pFrom, "0.3", "0.1", "0.2", "0.4"}
	}
	rv := func(id int64, date string, setMin int, ru string) []driver.Value {
		return []driver.Value{id, date, "4.0", ru, "1.2", "1.3", false, "", "", at(setMin)}
	}
	v := views{
		// the newer effective date wins even when inserted earlier; its rows keep insertion order
		rtable: [][]driver.Value{
			rt("2026-01-01", 50, "3.0"), rt("2026-01-01", 50, ""),
			rt("2026-02-01", 10, "3.5"), rt("2026-02-01", 10, "4.5"), rt("2026-02-01", 10, ""),
		},
		// same effective date: the later set_at wins
		margin: [][]driver.Value{
			mt("2026-02-01", 10, "1"), mt("2026-02-01", 10, "3"),
			mt("2026-02-01", 20, "1"), mt("2026-02-01", 20, "2"), mt("2026-02-01", 20, "3"),
		},
		rvers: [][]driver.Value{
			rv(1, "2026-01-01", 5, "1.01"), rv(2, "2026-02-01", 6, "1.02"), rv(3, "2026-02-01", 9, "1.03"),
		},
	}
	st := current(t, v)
	if st.RTable == nil || st.RTable.EffectiveDate != "2026-02-01" || len(st.RTable.Rows) != 3 ||
		st.RTable.Rows[0].YieldUpTo != "3.5" || st.RTable.Rows[2].YieldUpTo != "" || !st.RTable.SetAt.Equal(at(10)) {
		t.Errorf("r table = %+v", st.RTable)
	}
	if st.Margin == nil || len(st.Margin.Rows) != 3 || !st.Margin.SetAt.Equal(at(20)) || st.Margin.Rows[1].PFrom != "2" {
		t.Errorf("margin table = %+v", st.Margin)
	}
	if st.RVersion == nil || st.RVersion.ID != 3 || st.RVersion.RUS != "1.03" {
		t.Errorf("r version = %+v", st.RVersion)
	}
	rec := &recorder{answer: v.answer}
	all, err := rec.store().RVersions(context.Background())
	if err != nil || len(all) != 3 || all[0].ID != 3 || all[1].ID != 2 || all[2].ID != 1 {
		t.Errorf("RVersions = %+v, %v; want newest first 3, 2, 1", all, err)
	}

}

func TestNoRowsGiveNilVersionsAndNonNilSlices(t *testing.T) {
	t.Parallel()
	empty := current(t, views{})
	if empty.RTable != nil || empty.Margin != nil || empty.RVersion != nil || empty.Entries == nil {
		t.Errorf("empty state = %+v; versions must be nil and Entries non-nil", empty)
	}
	rec := &recorder{}
	none, err := rec.store().RVersions(context.Background())
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("RVersions on no rows = %v, %v; want a non-nil empty slice", none, err)
	}
}

func TestProfilesNewestPerKey(t *testing.T) {
	t.Parallel()
	v := views{profiles: [][]driver.Value{
		profileRow(1, "US:BBB", "Bbb old", 5),
		profileRow(2, "US:AAA", "Aaa old", 5),
		profileRow(3, "US:BBB", "Bbb new", 9),
		profileRow(4, "US:AAA", "Aaa same time, later id", 5),
		profileRow(5, "US:AAA", "Aaa older time, latest id", 1),
	}}
	st := current(t, v)
	if len(st.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(st.Entries))
	}
	a, b := st.Entries[0], st.Entries[1]
	if a.Profile.CompanyKey != "US:AAA" || a.Profile.Name != "Aaa same time, later id" {
		t.Errorf("first entry = %+v, want the newest AAA row by set_at then id", a.Profile)
	}
	if b.Profile.CompanyKey != "US:BBB" || b.Profile.Name != "Bbb new" {
		t.Errorf("second entry = %+v, want the newest BBB row", b.Profile)
	}
	if a.Current != nil || b.Current != nil {
		t.Errorf("no review rows, but a company has a current valuation")
	}
}

func TestReviewsDecodesAndNeverNil(t *testing.T) {
	t.Parallel()
	row := reviewRow(8, "a", "US:AAA", "first", "draft", "2026-02-01", -1)
	row[17] = `[{"percent":"5","reason":"x"}]`
	row[18] = `[{"label":"base","probability":"0.5","eps":"2","multiple":"10"}]`
	row[16] = `["a","b"]`
	row[25] = `{"p":"steady"}`
	rec := &recorder{answer: func(string, []driver.NamedValue) [][]driver.Value { return [][]driver.Value{row} }}
	got, err := rec.store().Reviews(context.Background(), "US:AAA")
	if err != nil || len(got) != 1 {
		t.Fatalf("reviews = %+v, %v", got, err)
	}
	r := got[0]
	if r.VersionDate != contracts.Day("2026-02-01") || len(r.MethodResults) != 2 || r.Adjustments[0].Percent != "5" ||
		r.Scenarios[0].Label != "base" || r.Reasons["p"] != "steady" || r.EffectiveAt != nil {
		t.Errorf("review = %+v", r)
	}
	rec = &recorder{}
	none, err := rec.store().Reviews(context.Background(), "US:NONE")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("reviews of no company = %v, %v; want a non-nil empty slice", none, err)
	}
}
