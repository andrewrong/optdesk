package macro_page

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

const eventsDay = "2026-11-05"

var (
	fredKinds = []string{"cpi", "employment_situation", "pce", "gdp", "ppi", "retail_sales", "jolts", "h41"}
	fileKinds = []string{"fomc", "boj", "quarter_end", "us_tax"}
)

type fakeEvents struct {
	events []macroevent.Event
	kinds  []macroevent.Kind
	err    error
}

func (f *fakeEvents) Between(_ context.Context, from, to contracts.Day) ([]macroevent.Event, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := []macroevent.Event{}
	for _, e := range f.events {
		if e.Date >= from && e.Date <= to {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeEvents) Kinds(context.Context) ([]macroevent.Kind, error) {
	return f.kinds, f.err
}

// fileRows is one file row per kind in 2026 and in 2027, both outside the 60 days of eventsDay (to 2027-01-04
// except 2026-01-02, which is before it), so every file kind counts as entered for both years.
func fileRows() []macroevent.Event {
	var out []macroevent.Event
	for _, k := range fileKinds {
		out = append(out,
			macroevent.Event{Kind: k, Source: "file", Date: "2026-01-02", RunDay: "2026-10-01"},
			macroevent.Event{Kind: k, Source: "file", Date: "2027-06-02", RunDay: "2026-10-01"})
	}
	return out
}

// healthy is a reader with no gap: every FRED kind fetched the day before with dates to the end of 2027, and
// every file kind entered for 2026 and 2027.
func healthy() *fakeEvents {
	f := &fakeEvents{events: fileRows()}
	for _, k := range fredKinds {
		f.kinds = append(f.kinds, macroevent.Kind{Kind: k, Source: "fred", LastRunDay: "2026-11-04", LastDate: "2027-12-31", Rows: 5})
	}
	for _, k := range fileKinds {
		f.kinds = append(f.kinds, macroevent.Kind{Kind: k, Source: "file", LastRunDay: "2026-10-01", LastDate: "2027-12-31", Rows: 5})
	}
	return f
}

func buildEvents(t *testing.T, f *fakeEvents) map[string]any {
	t.Helper()
	c := cfg("A")
	if f != nil {
		c.Events = f
	}
	return build(t, fakeReader{}, c, eventsDay)
}

func gapTexts(m map[string]any) []string {
	var out []string
	for _, g := range m["gaps"].([]any) {
		out = append(out, g.(map[string]any)["text"].(string))
	}
	return out
}

func gapWithPrefix(m map[string]any, prefix string) (string, bool) {
	for _, g := range gapTexts(m) {
		if strings.HasPrefix(g, prefix) {
			return g, true
		}
	}
	return "", false
}

func eventRows(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, e := range m["events"].([]any) {
		out = append(out, e.(map[string]any))
	}
	return out
}

func TestEventsConventionTexts(t *testing.T) {
	t.Parallel()
	f := healthy()
	f.events = append(f.events,
		macroevent.Event{Kind: "cpi", Source: "fred", Date: "2026-11-10", RunDay: "2026-11-04"},
		macroevent.Event{Kind: "jolts", Source: "fred", Date: "2026-11-11", RunDay: "2026-11-04"},
		macroevent.Event{Kind: "h41", Source: "fred", Date: "2026-11-12", RunDay: "2026-11-04"},
		macroevent.Event{Kind: "ppi", Source: "fred", Date: "2026-11-13", RunDay: "2026-11-04"},
		macroevent.Event{Kind: "fomc", Source: "file", Date: "2026-12-09", RunDay: "2026-10-01"},
		macroevent.Event{Kind: "boj", Source: "file", Date: "2026-12-19", RunDay: "2026-10-02"},
		macroevent.Event{Kind: "quarter_end", Source: "file", Date: "2026-12-31", RunDay: "2026-10-03"},
		macroevent.Event{Kind: "us_tax", Source: "file", Date: "2026-12-15", RunDay: "2026-10-04"},
	)
	got := map[string][2]string{}
	for _, r := range eventRows(buildEvents(t, f)) {
		got[r["date"].(string)] = [2]string{r["name_text"].(string), r["when_text"].(string)}
		if strings.Contains(r["name_text"].(string), "high") {
			t.Errorf("importance tag in name %q", r["name_text"])
		}
		if p, isGap := r["previous"].(map[string]any); isGap && p["reason"] == "not_built" {
			t.Errorf("previous %v", p)
		}
	}
	want := map[string][2]string{
		"2026-11-10": {"CPI", "usually 8:30 ET, high"},
		"2026-11-11": {"JOLTS", "usually 10:00 ET"},
		"2026-11-12": {"H.4.1", "usually 4:30 pm ET"},
		"2026-11-13": {"PPI", "usually 8:30 ET"},
		"2026-12-09": {"FOMC decision", "usually 2:00 pm ET, high, read 2026-10-01"},
		"2026-12-19": {"BoJ decision", "usually around midday JST, date in JST, high, read 2026-10-02"},
		"2026-12-31": {"Quarter-end", "date only, read 2026-10-03"},
		"2026-12-15": {"US corporate tax date", "date only, read 2026-10-04"},
	}
	for d, w := range want {
		if got[d] != w {
			t.Errorf("%s: got %q want %q", d, got[d], w)
		}
	}
}

func TestEventsWindow60AndCap(t *testing.T) {
	t.Parallel()
	f := healthy()
	f.events = append(f.events,
		macroevent.Event{Kind: "cpi", Source: "fred", Date: "2027-01-04"}, // day + 60
		macroevent.Event{Kind: "ppi", Source: "fred", Date: "2027-01-05"}, // day + 61
	)
	m := buildEvents(t, f)
	r := eventRows(m)
	if len(r) != 1 || r[0]["date"] != "2027-01-04" {
		t.Fatalf("rows %v", r)
	}
	if _, ok := gapWithPrefix(m, "Events after"); ok {
		t.Error("truncation gap with one row")
	}

	f = healthy()
	for i := 0; i < 21; i++ {
		d := contracts.Day(fmt.Sprintf("2026-12-%02d", 1+i))
		f.events = append(f.events, macroevent.Event{Kind: "h41", Source: "fred", Date: d, RunDay: "2026-11-04"})
	}
	m = buildEvents(t, f)
	if n := len(eventRows(m)); n != 20 {
		t.Fatalf("rows %d", n)
	}
	want := fmt.Sprintf(gapEventsNotShown, "2026-12-20", 20, 21)
	found := false
	for _, g := range gapTexts(m) {
		found = found || g == want
	}
	if !found {
		t.Errorf("truncation gap %q not in %v", want, gapTexts(m))
	}
}

func TestEventsNilReaderKeepsGap(t *testing.T) {
	t.Parallel()
	m := buildEvents(t, nil)
	if m["events_title"] != "This week and next" {
		t.Errorf("title %v", m["events_title"])
	}
	if m["sentence"] != "Macro indicators come from stored series; ease and events: no data yet." {
		t.Errorf("sentence %v", m["sentence"])
	}
	if _, ok := gapWithPrefix(m, "Event calendar: no data yet"); !ok {
		t.Errorf("gaps %v", gapTexts(m))
	}
	if len(eventRows(m)) != 0 {
		t.Errorf("rows %v", eventRows(m))
	}
}

func TestEventsReaderDropsOldGap(t *testing.T) {
	t.Parallel()
	m := buildEvents(t, healthy())
	if _, ok := gapWithPrefix(m, "Event calendar: no data yet"); ok {
		t.Errorf("old gap kept: %v", gapTexts(m))
	}
	if m["events_title"] != "Next 60 days, dates in the publisher's own zone (ET, JST)" {
		t.Errorf("title %v", m["events_title"])
	}
	if m["sentence"] != "Macro indicators come from stored series; ease: no data yet." {
		t.Errorf("sentence %v", m["sentence"])
	}
	if got := gapTexts(m); len(got) != 1 || got[0] != "Ease: no data yet" {
		t.Errorf("gaps %v", got)
	}
}

func TestEventsStalePerKind(t *testing.T) {
	t.Parallel()
	f := healthy()
	f.kinds[0].LastRunDay = "2026-10-20" // cpi, 16 days
	f.kinds[1].LastRunDay = "2026-10-26" // exactly 10 days: not stale
	m := buildEvents(t, f)
	g, ok := gapWithPrefix(m, gapPrefixStale)
	if !ok || g != gapPrefixStale+"CPI (fetched 2026-10-20)" {
		t.Errorf("stale gap %q %v", g, ok)
	}
}

func TestEventsNoDatesStored(t *testing.T) {
	t.Parallel()
	f := healthy()
	f.kinds = f.kinds[1:]              // cpi has no row at all
	f.kinds[0].LastDate = "2026-11-05" // employment situation has none after the day
	m := buildEvents(t, f)
	g, ok := gapWithPrefix(m, gapPrefixNone)
	if !ok || g != gapPrefixNone+"CPI, Employment situation" {
		t.Errorf("gap %q %v", g, ok)
	}
}

func TestEventsHeldGap(t *testing.T) {
	t.Parallel()
	f := healthy()
	f.kinds[0].HeldDay = "2026-11-03"
	f.kinds[1].HeldDay = "2026-11-03"
	f.kinds[1].LastDate = "2026-11-05" // held but no row after the day: only the no-dates line
	m := buildEvents(t, f)
	g, ok := gapWithPrefix(m, gapPrefixHeld)
	if !ok || g != gapPrefixHeld+"CPI (2026-11-03)" {
		t.Errorf("gap %q %v", g, ok)
	}
}

func TestEventsGapsMerged(t *testing.T) {
	t.Parallel()
	f := healthy()
	for i := 0; i < 5; i++ {
		f.kinds[i].LastRunDay = "2026-10-01"
	}
	m := buildEvents(t, f)
	n := 0
	for _, g := range gapTexts(m) {
		if strings.HasPrefix(g, gapPrefixStale) {
			n++
			if utf8.RuneCountInString(g) > 200 {
				t.Errorf("line of %d characters", utf8.RuneCountInString(g))
			}
			if !strings.Contains(g, "PPI (fetched 2026-10-01)") {
				t.Errorf("line %q", g)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d stale lines", n)
	}

	for i := range f.kinds[:8] { // all eight FRED kinds: more than 200 characters
		f.kinds[i].LastRunDay = "2026-10-01"
	}
	m = buildEvents(t, f)
	g, _ := gapWithPrefix(m, gapPrefixStale)
	cut := strings.LastIndex(g, gapMoreAfterList[:strings.Index(gapMoreAfterList, "%d")])
	if n := utf8.RuneCountInString(g); n > 200 || cut < 0 || !utf8.ValidString(g) {
		t.Errorf("cut line %q (%d)", g, n)
	}
	var more int
	if _, err := fmt.Sscanf(g[cut:], gapMoreAfterList, &more); err != nil || more < 1 {
		t.Errorf("ending %q: %v", g[cut:], err)
	}
}

func TestEventsNotEnteredGap(t *testing.T) {
	t.Parallel()
	f := healthy()
	var keep []macroevent.Event
	for _, e := range f.events {
		if e.Kind == "fomc" && e.Date < "2027-01-01" {
			continue
		}
		if e.Kind == "boj" && e.Date >= "2027-01-01" {
			continue
		}
		keep = append(keep, e)
	}
	f.events = keep
	m := buildEvents(t, f)
	g, ok := gapWithPrefix(m, gapPrefixNotEntered)
	if !ok || g != gapPrefixNotEntered+"FOMC 2026, BoJ 2027" {
		t.Errorf("gap %q %v", g, ok)
	}
}

func TestEventsReaderErrorFailsBuild(t *testing.T) {
	t.Parallel()
	c := cfg("A")
	c.Events = &fakeEvents{err: errors.New("boom")}
	if _, err := Build(t.Context(), fakeReader{}, c, eventsDay, fixedAt); err == nil {
		t.Fatal("want an error")
	}
}

func windowBuild(t *testing.T, ev *fakeEvents, w WindowFunc) map[string]any {
	t.Helper()
	c := cfg("A")
	c.Events, c.Window = ev, w
	return buildWith(t, fakeReader{}, c)
}

func TestEventWindowFlag(t *testing.T) {
	t.Parallel()
	ev := healthy()
	ev.events = append(ev.events,
		eventOn("cpi", "2026-11-04"), eventOn("cpi", "2026-11-05"), eventOn("pce", "2026-12-20"), eventOn("gdp", "2026-12-21"))
	rows := eventRows(windowBuild(t, ev, windowOf45(true)))
	got := map[string][2]any{}
	for _, r := range rows {
		got[r["date"].(string)] = [2]any{r["days_to"], r["in_window"]}
	}
	want := map[string][2]any{
		"2026-11-04": {float64(-1), false}, "2026-11-05": {float64(0), true}, "2026-12-20": {float64(45), true}, "2026-12-21": {float64(46), false},
	}
	for d, w := range want {
		if got[d] != w {
			t.Errorf("%s: got %v want %v", d, got[d], w)
		}
	}
	// a window of 46 days takes the last in
	rows = eventRows(windowBuild(t, ev, func(context.Context, time.Time) (int, bool) { return 46, true }))
	for _, r := range rows {
		if r["date"] == "2026-12-21" && r["in_window"] != true {
			t.Errorf("46-day window: %v", r)
		}
	}
	// no Window: no new field
	c := cfg("A")
	c.Events = ev
	for _, r := range eventRows(buildWith(t, fakeReader{}, c)) {
		if _, ok := r["days_to"]; ok {
			t.Fatalf("days_to without Window: %v", r)
		}
		if _, ok := r["in_window"]; ok {
			t.Fatalf("in_window without Window: %v", r)
		}
	}
}

func TestWindowSourceWords(t *testing.T) {
	t.Parallel()
	ev := healthy()
	ev.events = append(ev.events, eventOn("fomc", "2026-11-20"))
	cases := []struct {
		name string
		w    WindowFunc
		want string
	}{
		{"stored", func(context.Context, time.Time) (int, bool) { return 30, true }, "the 30-day option window (your max_dte setting)"},
		{"none stored", windowOf45(false), "the 45-day option window (default, none set)"},
		{"failed read", func(context.Context, time.Time) (int, bool) { return 0, true }, "the 45-day option window (default, none set)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			text := leadRows(t, windowBuild(t, ev, tc.w))[2]["text"].(string)
			if !strings.HasSuffix(text, tc.want) || !strings.HasPrefix(text, "FOMC decision, 2026-11-20, in 15 days, inside ") {
				t.Fatalf("text %q", text)
			}
		})
	}
}

func TestMacroGapTextsAreChinese(t *testing.T) {
	t.Parallel()
	f := healthy()
	f.kinds[0].LastRunDay = "2026-10-20" // cpi stale
	f.kinds[1].LastDate = "2026-11-05"   // employment situation: no date after the day
	f.events = append(f.events, macroevent.Event{Kind: "cpi", Source: "fred", Date: "2026-11-12", RunDay: "2026-11-04"})
	for i := 0; i < 21; i++ {
		d := contracts.Day(fmt.Sprintf("2026-12-%02d", 1+i))
		f.events = append(f.events, macroevent.Event{Kind: "h41", Source: "fred", Date: d, RunDay: "2026-11-04"})
	}
	c := cfg("U")
	c.Events = f
	c.Liquidity = fixedFunc(fullResult())
	r := fakeReader{points: map[string][]contracts.SeriesPoint{"U": {pt(t, "2026-11-04", "2026-11-04", "index", f64(1))},
		"CPIAUCNS": monthlySeries(t, "CPIAUCNS", "index_points", "2025-01-01", 21, yoyIndex)}}
	m := buildWith(t, r, c)

	var texts []string
	texts = append(texts, gapTexts(m)...)
	for _, row := range eventRows(m) {
		if a, ok := row["actual"].(map[string]any); ok && a["reason"] != nil {
			texts = append(texts, a["text"].(string))
		}
	}
	if latest, ok := indicator(m, 0)["latest"].(map[string]any); ok && latest["reason"] != nil {
		texts = append(texts, latest["text"].(string))
	}
	for _, want := range []string{gapPrefixStale, gapPrefixNone, gapNotYetReleased, gapUnitUnknown} {
		found := false
		for _, s := range texts {
			found = found || strings.Contains(s, want)
		}
		if !found {
			t.Errorf("no text holds %q in %v", want, texts)
		}
	}
	old := []string{"Dates older", "No dates stored", "Stored dates kept", "Not entered", "and 1 more", "not shown", "not published",
		"not stored", "not readable", "earlier estimate", "not yet", "unit unknown"}
	for _, s := range texts {
		if !strings.ContainsFunc(s, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			// the "no data yet" family stays English by the issue; only the texts of items 1 to 5 are checked
			if !strings.Contains(s, "no data yet") && !strings.Contains(s, "Data day") && !strings.Contains(s, "no stored point") && !strings.Contains(s, "latest stored") {
				t.Errorf("no Chinese character: %q", s)
			}
		}
		for _, o := range old {
			if strings.Contains(s, o) {
				t.Errorf("old English %q in %q", o, s)
			}
		}
	}
}
