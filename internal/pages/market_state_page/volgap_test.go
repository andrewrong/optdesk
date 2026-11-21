package market_state_page

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var fixedNow = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

type fakeIndicators struct{ compute.Indicators }

func parseJSON(b []byte) (any, error) {
	var v any
	err := json.Unmarshal(b, &v)
	return v, err
}

func ivReadings(t *testing.T, rows map[string][]contracts.ReadingRecord) compute.ReadingFunc {
	base := fakeReadings(t)
	return func(ctx context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
		if name == figIvRv20 {
			return rows[subject], nil
		}
		return base(ctx, name, kind, subject, from, to)
	}
}

func volRows(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, r := range list(obj(m["vol_gap"])["rows"]) {
		out = append(out, obj(r))
	}
	return out
}

func TestVolGapRows(t *testing.T) {
	t.Parallel()
	schema := compileSchema(t)
	stale := rec("2026-11-13", "0.01", "2026-11-13")
	stale.State = "missing_input"
	code := 3
	stale.ReasonCode = &code
	notOK := rec("2026-11-13", "0.01", "2026-11-13")
	notOK.State = "missing_input"
	rows := map[string][]contracts.ReadingRecord{
		// ok reading of an older day wins over a not-ok one of today
		"SPX": {rec("2026-11-12", "0.0349", "2026-11-12"), notOK},
		"NDX": {rec("2026-11-13", "-0.0212", "2026-11-13")},
		"RUT": nil,
	}
	c := fullConfig(t, chartResult())
	c.Readings = ivReadings(t, rows)
	b, err := Build(context.Background(), c, fakeSeries{}, testDay, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := parseJSON(b)
	if err := schema.Validate(v); err != nil {
		t.Fatal(err)
	}
	vr := volRows(decode(t, b))
	if len(vr) != 3 || vr[0]["id"] != "spx" || vr[1]["id"] != "ndx" || vr[2]["id"] != "rut" {
		t.Fatalf("rows %v", vr)
	}
	f := obj(vr[0]["figure"])
	if f["text"] != "+3.5 (11-12)" || f["unit"] != "vol_points" || f["value"] != json.Number("3.5") {
		t.Errorf("spx %v", f)
	}
	if vr[0]["day_text"] != volGapDayPrefix+"11-12" || obj(vr[0]["figure"])["text"] != "+3.5 (11-12)" {
		t.Errorf("day_text %v", vr[0]["day_text"])
	}
	if f := obj(vr[1]["figure"]); f["text"] != "−2.1" {
		t.Errorf("ndx %v", f)
	}
	if g := obj(vr[2]["figure"]); g["reason"] != "no_data" {
		t.Errorf("rut %v", g)
	}

	// missing, stale and old rows
	old := rec("2026-11-06", "0.02", "2026-11-06")
	bad := rec("2026-11-13", "", "2026-11-13")
	bad.Value = nil
	for name, tc := range map[string]struct {
		r      contracts.ReadingRecord
		reason string
	}{"stored stale": {stale, "stale_input"}, "old data day": {old, "stale_input"}, "no value": {bad, "missing_input"}} {
		got, _ := ivRvFigure("SPX", &tc.r, mustDay(testDay))
		if g, ok := got.(gap); !ok || g.Reason != tc.reason {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func mustDay(s string) time.Time {
	d, err := time.Parse(dayLayout, s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestVolGapWithoutReaderIsGaps(t *testing.T) {
	t.Parallel()
	c := fullConfig(t, marketstateapi.Result{})
	c.Readings = nil
	m := build(t, c)
	for _, r := range volRows(m) {
		if g := obj(r["figure"]); g["kind"] != "gap" {
			t.Errorf("%v", g)
		}
	}
}

// Decision 7: the day is shown only when it is not the page day. The schema requires a non-empty
// day_text, so on the page day it is a fixed text without the date and the figure text has no date.
func TestVolGapSameDayHidesDay(t *testing.T) {
	t.Parallel()
	r := rec("2026-11-13", "-0.0212", "2026-11-13")
	fig, dayText := ivRvFigure("NDX", &r, mustDay("2026-11-13"))
	if dayText != volGapSameDay || strings.Contains(dayText, "11-13") {
		t.Errorf("day_text %q on the page day", dayText)
	}
	if f := fig.(figure); strings.Contains(f.Text, "11-13") || f.Text != "−2.1" {
		t.Errorf("figure text %q", f.Text)
	}
	old := rec("2026-11-12", "-0.0212", "2026-11-12")
	fig, dayText = ivRvFigure("NDX", &old, mustDay("2026-11-13"))
	if dayText != volGapDayPrefix+"11-12" || fig.(figure).Text != "−2.1 (11-12)" {
		t.Errorf("off-day: %q %q", dayText, fig.(figure).Text)
	}
}
