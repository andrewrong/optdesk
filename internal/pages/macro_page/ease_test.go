package macro_page

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const easeDay = "2026-11-05"
const easeWeekAgo = "2026-10-29"

// easeRow is a stored macro.ease reading; an empty value makes it a missing_input row.
func easeRow(t *testing.T, day, value, band string) contracts.ReadingRecord {
	t.Helper()
	r := contracts.ReadingRecord{
		Name: "macro.ease", Version: 2, SubjectKind: "market", Subject: "WALCL",
		Day: date(t, day), Rev: 1, State: "ok", Value: &value,
	}
	if value == "" {
		r.State, r.Value = "missing_input", nil
		return r
	}
	params := `[{"param":"hy_oas.data_day","value_num":null,"value_tag":"` + day + `"},{"param":"dgs10.data_day","value_num":null,"value_tag":"2026-01-01"}`
	if band != "" {
		params += `,{"param":"band","value_num":null,"value_tag":"` + band + `"}`
	}
	if err := json.Unmarshal([]byte(params+"]"), &r.Parameters); err != nil {
		t.Fatal(err)
	}
	return r
}

func readerOf(rows ...contracts.ReadingRecord) func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
	return func(_ context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
		var out []contracts.ReadingRecord
		for _, r := range rows {
			d := contracts.Day(r.Day.Format(dayLayout))
			if r.Name == name && r.SubjectKind == kind && r.Subject == subject && d >= from && d <= to {
				out = append(out, r)
			}
		}
		return out, nil
	}
}

func easeBuild(t *testing.T, rows ...contracts.ReadingRecord) map[string]any {
	t.Helper()
	c := cfg("DGS10")
	c.Read = readerOf(rows...)
	return build(t, fakeReader{}, c, easeDay)
}

func fig(t *testing.T, m map[string]any, key string) (float64, string) {
	t.Helper()
	f, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %v", key, m[key])
	}
	return f["value"].(float64), f["text"].(string)
}

func TestEaseFromReading(t *testing.T) {
	t.Parallel()
	m := easeBuild(t, easeRow(t, easeDay, "56.42", "steady"), easeRow(t, easeWeekAgo, "56.62", "steady"))
	e := m["ease"].(map[string]any)
	if v, s := fig(t, e, "figure"); v != 56.4 || s != "56" {
		t.Errorf("figure %v %q", v, s)
	}
	if v, s := fig(t, e, "marker_percent"); v != 56.42 || s != "56" {
		t.Errorf("marker %v %q", v, s)
	}
	nb := e["neutral_band"].(map[string]any)
	if v, s := fig(t, nb, "low"); v != 33.5 || s != "33.5%" {
		t.Errorf("low %v %q", v, s)
	}
	if v, s := fig(t, nb, "high"); v != 66.5 || s != "66.5%" {
		t.Errorf("high %v %q", v, s)
	}
	if !reflect.DeepEqual(e["ticks"], []any{"0", "25", "50", "75", "100"}) {
		t.Errorf("ticks %v", e["ticks"])
	}
	if v, s := fig(t, e, "week_change"); v != -0.2 || s != "−0.2" {
		t.Errorf("week_change %v %q", v, s)
	}
	if e["reading_text"] != "Now 56, Steady; fixed scale 0 to 100." {
		t.Errorf("reading_text %v", e["reading_text"])
	}
	if e["aria_text"] != "Ease 56, Steady, Week change −0.2; fixed scale 0 to 100, steady band 33.5 to 66.5." {
		t.Errorf("aria_text %v", e["aria_text"])
	}
	dt := m["data_times"].([]any)[0].(map[string]any)
	if dt["source"] != "macro_ease" || dt["data_time"] != "2026-11-05T00:00:00+08:00" {
		t.Errorf("data_times %v", m["data_times"])
	}
	gaps := m["gaps"].([]any)
	if len(gaps) != 1 || gaps[0].(map[string]any)["text"] != "Event calendar: no data yet" {
		t.Errorf("gaps %v", gaps)
	}
}

func TestEaseWeekChangeSign(t *testing.T) {
	t.Parallel()
	m := easeBuild(t, easeRow(t, easeDay, "57.80", "steady"), easeRow(t, easeWeekAgo, "56.60", "steady"))
	if v, s := fig(t, m["ease"].(map[string]any), "week_change"); v != 1.2 || s != "+1.2" {
		t.Errorf("week_change %v %q", v, s)
	}
}

func TestEaseBandWords(t *testing.T) {
	t.Parallel()
	for band, word := range map[string]string{"tighter": "Tighter", "steady": "Steady", "looser": "Looser"} {
		m := easeBuild(t, easeRow(t, easeDay, "50.00", band))
		rt := m["ease"].(map[string]any)["reading_text"].(string)
		if !strings.Contains(rt, ", "+word+";") {
			t.Errorf("band %s: reading_text %q", band, rt)
		}
	}
}

func TestEaseWeekChangeGap(t *testing.T) {
	t.Parallel()
	m := easeBuild(t, easeRow(t, easeDay, "56.42", "steady"))
	wc := m["ease"].(map[string]any)["week_change"].(map[string]any)
	if wc["reason"] != "no_data" || wc["text"] != "Week change: no data yet" {
		t.Errorf("week_change %v", wc)
	}
}

func TestEaseMissingReadingKeepsGap(t *testing.T) {
	t.Parallel()
	for name, rows := range map[string][]contracts.ReadingRecord{
		"missing_input": {easeRow(t, easeDay, "", "")},
		"no row":        nil,
	} {
		m := easeBuild(t, rows...)
		b, _ := json.Marshal(m["ease"])
		if string(b) != `{"reason":"no_data","text":"Ease: no data yet"}` {
			t.Errorf("%s: ease %s", name, b)
		}
		if len(m["gaps"].([]any)) != 2 {
			t.Errorf("%s: gaps %v", name, m["gaps"])
		}
	}
}

func TestEaseNilReadIsUnchanged(t *testing.T) {
	t.Parallel()
	b, err := Build(t.Context(), goldenReader(t), goldenConfig(), "2026-11-05", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Errorf("bytes differ:\n%s", b)
	}
}

func TestEaseMarkerInsideBandIsSteady(t *testing.T) {
	t.Parallel()
	m := easeBuild(t, easeRow(t, easeDay, "33.70", "steady"))
	e := m["ease"].(map[string]any)
	if !strings.Contains(e["reading_text"].(string), "Steady") {
		t.Errorf("reading_text %v", e["reading_text"])
	}
	mv, _ := fig(t, e, "marker_percent")
	lo, _ := fig(t, e["neutral_band"].(map[string]any), "low")
	hi, _ := fig(t, e["neutral_band"].(map[string]any), "high")
	if mv != 33.7 || mv <= lo || mv >= hi {
		t.Errorf("marker %v not inside %v..%v", mv, lo, hi)
	}
}

func TestEaseHalfUpRounding(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]struct {
		fig  float64
		text string
	}{"56.45": {56.5, "56"}, "56.5": {56.5, "57"}} {
		e := easeBuild(t, easeRow(t, easeDay, value, "steady"))["ease"].(map[string]any)
		if v, s := fig(t, e, "figure"); v != want.fig || s != want.text {
			t.Errorf("%s: figure %v %q", value, v, s)
		}
	}
}

func TestEaseReaderErrorIsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	c := cfg("DGS10")
	c.Read = func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return nil, boom
	}
	if _, err := Build(t.Context(), fakeReader{}, c, easeDay, fixedAt); !errors.Is(err, boom) {
		t.Errorf("err %v", err)
	}
}

func TestEaseDataTimeIsNewestInputDataDay(t *testing.T) {
	t.Parallel()
	r := easeRow(t, easeDay, "56.42", "steady")
	older := "2026-11-03"
	r.Parameters[0].ValueTag = &older // hy_oas.data_day; dgs10.data_day is older still
	m := easeBuild(t, r)
	dt := m["data_times"].([]any)[0].(map[string]any)
	if dt["source"] != "macro_ease" || dt["data_time"] != "2026-11-03T00:00:00+08:00" {
		t.Fatalf("data_times[0] = %v", dt)
	}
}

func TestEaseWinnerOkOverNewerMissing(t *testing.T) {
	t.Parallel()
	ok := easeRow(t, easeDay, "56.42", "steady")
	missing := easeRow(t, easeDay, "", "")
	missing.Rev = 2
	e, isMap := easeBuild(t, ok, missing)["ease"].(map[string]any)
	if !isMap || e["figure"] == nil {
		t.Fatalf("winner ok rev 1 not shown: %v", e)
	}
}
