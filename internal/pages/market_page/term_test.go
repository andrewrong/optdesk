package market_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func termEnv(day string, vix []contracts.SeriesPoint) env {
	e := allOK()
	e.day = day
	e.series = map[string][]contracts.SeriesPoint{seriesVIX: vix}
	return e
}

func cell(t *testing.T, m map[string]any, row int, k string) map[string]any {
	t.Helper()
	return at(t, m, "term", "points", row, k).(map[string]any)
}

func TestTermPicksStoredPoints(t *testing.T) {
	t.Parallel()
	e := termEnv("2026-11-07", []contracts.SeriesPoint{pt("2026-10-29", 18), pt("2026-11-06", 21.5), pt("2026-10-30", 19.25)})
	m := e.page(t)
	today, older := cell(t, m, 0, "today"), cell(t, m, 0, "five_days_ago")
	if today["text"] != "21.50" || today["unit"] != "index_points" || at(t, m, "term", "points", 0, "label") != "1M" {
		t.Errorf("today = %v", today)
	}
	if older["text"] != "19.25" {
		t.Errorf("five_days_ago = %v", older)
	}
	if g := cell(t, m, 1, "today"); g["reason"] != "no_data" {
		t.Errorf("3M without points = %v", g)
	}

	two := pt("2026-11-06", 20)
	newer := pt("2026-11-06", 22)
	newer.RealtimeStart = sdate("2026-11-07")
	future := pt("2026-11-06", 99)
	future.RealtimeStart = sdate("2026-11-08")
	m = termEnv("2026-11-07", []contracts.SeriesPoint{two, future, newer}).page(t)
	if got := cell(t, m, 0, "today")["text"]; got != "22.00" {
		t.Errorf("vintages: today = %v, want the newer vintage 22.00", got)
	}
}

func TestTermStaleIsGap(t *testing.T) {
	t.Parallel()
	m := termEnv("2026-11-07", []contracts.SeriesPoint{pt("2026-11-02", 21), pt("2026-10-27", 19)}).page(t)
	if g := cell(t, m, 0, "today"); g["kind"] != "gap" || g["reason"] != "stale" {
		t.Errorf("today = %v", g)
	}
	m = termEnv("2026-11-06", []contracts.SeriesPoint{pt("2026-11-02", 21)}).page(t)
	if g := cell(t, m, 0, "today"); g["kind"] != "value" {
		t.Errorf("4 days old: today = %v", g)
	}
	if g := cell(t, m, 0, "five_days_ago"); g["reason"] != "no_data" {
		t.Errorf("no older point: five_days_ago = %v", g)
	}
	// an older point more than 4 days before the target date is not used.
	m = termEnv("2026-11-06", []contracts.SeriesPoint{pt("2026-11-06", 21), pt("2026-10-25", 17)}).page(t)
	if g := cell(t, m, 0, "five_days_ago"); g["reason"] != "no_data" {
		t.Errorf("far older point: five_days_ago = %v", g)
	}
}

func TestTermAllGapIsOneGap(t *testing.T) {
	t.Parallel()
	e := allOK()
	e.series = nil
	m := e.page(t)
	g := at(t, m, "term").(map[string]any)
	if g["kind"] != "gap" || g["reason"] != "no_data" {
		t.Errorf("term = %v", g)
	}
	if m["run_status"] != "partial" {
		t.Errorf("run_status = %v", m["run_status"])
	}
}

func termSentenceEnv(ratioDay, pointDay string) env {
	e := allOK()
	e.day = pointDay
	e.readings[figVixTerm] = []contracts.ReadingRecord{rec(ratioDay, "ok", "0.905000", nil, ratioDay)}
	e.series = map[string][]contracts.SeriesPoint{
		seriesVIX: {pt(pointDay, 17.49)},
		seriesVXV: {pt(pointDay, 19.24)},
	}
	return e
}

func TestTermSentenceNamesRatioOnlyOfPointsDay(t *testing.T) {
	t.Parallel()
	m := termSentenceEnv("2026-11-10", "2026-11-11").page(t)
	if got, want := at(t, m, "term", "sentence"), "Term structure ratio: not stored for 11-11."; got != want {
		t.Errorf("older ratio: sentence = %v, want %q", got, want)
	}
	m = termSentenceEnv("2026-11-11", "2026-11-11").page(t)
	if got, want := at(t, m, "term", "sentence"), "VIX to 3-month VIX ratio 0.90"; got != want {
		t.Errorf("same day: sentence = %v, want %q", got, want)
	}
}
