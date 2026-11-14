package macroevents

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestHandRejectsNoURL(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/hand_bad_no_url.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseHand(data); err == nil || !strings.Contains(err.Error(), "url") {
		t.Fatalf("err = %v", err)
	}
	noDay := "[[source]]\nkind = \"boj\"\nyear = 2026\nurl = \"https://x\"\ndates = []\n"
	if _, err := parseHand([]byte(noDay)); err == nil || !strings.Contains(err.Error(), "read_day") {
		t.Fatalf("err = %v", err)
	}
}

func TestHandRowsHaveURLAndReadDay(t *testing.T) {
	t.Parallel()
	srcs, err := parseHand(handBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 8 {
		t.Fatalf("sources = %d", len(srcs))
	}
	for _, s := range srcs {
		if s.URL == "" || s.ReadDay == "" {
			t.Errorf("%s %d: url %q read_day %q", s.Kind, s.Year, s.URL, s.ReadDay)
		}
	}
}

func TestHandCountPerKindAndYear(t *testing.T) {
	t.Parallel()
	srcs, err := parseHand(handBytes)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"fomc": 8, "boj": 8, "quarter_end": 4, "us_tax": 4}
	years := map[int]map[string]int{}
	for _, s := range srcs {
		if years[s.Year] == nil {
			years[s.Year] = map[string]int{}
		}
		years[s.Year][s.Kind] += len(s.Dates)
		for _, d := range s.Dates {
			if !strings.HasPrefix(d, strconv.Itoa(s.Year)+"-") {
				t.Errorf("%s %d: date %s is in another year", s.Kind, s.Year, d)
			}
		}
	}
	if len(years) == 0 {
		t.Fatal("no year")
	}
	for y, kinds := range years {
		for k, n := range want {
			if kinds[k] != n {
				t.Errorf("%d %s: %d dates, want %d", y, k, kinds[k], n)
			}
		}
		if len(kinds) != len(want) {
			t.Errorf("%d: kinds %v", y, kinds)
		}
	}
}

func TestMinutesAre21DaysAfter(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]contracts.Day{
		"2026-01-28": "2026-02-18",
		"2026-12-09": "2026-12-30",
		"2027-11-17": "2027-12-08",
		"2026-12-20": "2027-01-10",
	} {
		if got := minutesDay(contracts.Day(in)); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
	evs, err := fileEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(evs["fomc_minutes"]) != len(evs["fomc"]) || evs["fomc_minutes"][0].Date != "2026-02-18" {
		t.Fatalf("minutes = %v", evs["fomc_minutes"])
	}
}
