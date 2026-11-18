package shared

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func at(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestPickNewestDayWins(t *testing.T) {
	t.Parallel()
	s := Source{
		Bars:   []Bar{{Day: "2026-11-17", Close: "367.71"}},
		Prints: []Print{{Day: "2026-11-18", Price: "373.80"}},
	}
	got, ok := Pick(s, "2026-11-18")
	if !ok || got != (Close{Day: "2026-11-18", Price: "373.80", Via: "option_eod"}) {
		t.Fatalf("got %+v %v", got, ok)
	}
}

func TestPickSameDayBarWins(t *testing.T) {
	t.Parallel()
	s := Source{
		Bars:   []Bar{{Day: "2026-11-17", Close: "10"}},
		Prints: []Print{{Day: "2026-11-17", Price: "11", At: at("2026-11-17T21:00:00Z")}},
	}
	got, ok := Pick(s, "2026-11-18")
	if !ok || got != (Close{Day: "2026-11-17", Price: "10", Via: "bar"}) {
		t.Fatalf("got %+v %v", got, ok)
	}
}

func TestPickLatestEventTimeInDay(t *testing.T) {
	t.Parallel()
	s := Source{Prints: []Print{
		{Day: "2026-11-17", Price: "1", At: at("2026-11-17T20:00:00Z")},
		{Day: "2026-11-17", Price: "2", At: at("2026-11-17T21:00:00Z")},
		{Day: "2026-11-17", Price: "3", At: at("2026-11-17T19:00:00Z")},
	}}
	got, ok := Pick(s, "2026-11-17")
	if !ok || got.Price != "2" {
		t.Fatalf("got %+v %v", got, ok)
	}
	// equal times go to the later row
	s.Prints = []Print{
		{Day: "2026-11-17", Price: "5", At: at("2026-11-17T20:00:00Z")},
		{Day: "2026-11-17", Price: "6", At: at("2026-11-17T20:00:00Z")},
	}
	if got, _ := Pick(s, "2026-11-17"); got.Price != "6" {
		t.Fatalf("equal At: got %+v", got)
	}
}

func TestPickNilEventTimeRanksLast(t *testing.T) {
	t.Parallel()
	s := Source{Prints: []Print{
		{Day: "2026-11-17", Price: "1", At: at("2026-11-17T20:00:00Z")},
		{Day: "2026-11-17", Price: "2"},
	}}
	if got, _ := Pick(s, "2026-11-17"); got.Price != "1" {
		t.Fatalf("got %+v", got)
	}
	s.Prints = []Print{{Day: "2026-11-17", Price: "7"}, {Day: "2026-11-17", Price: "8"}}
	if got, _ := Pick(s, "2026-11-17"); got.Price != "8" {
		t.Fatalf("both nil: got %+v", got)
	}
}

func TestPickLookbackLimit(t *testing.T) {
	t.Parallel()
	asOf := contracts.Day("2026-11-18")
	old := Source{Bars: []Bar{{Day: "2026-11-03", Close: "10"}}}
	if _, ok := Pick(old, asOf); ok {
		t.Fatal("15 days old bar taken")
	}
	edge := Source{Bars: []Bar{{Day: "2026-11-04", Close: "10"}}}
	if got, ok := Pick(edge, asOf); !ok || got.Price != "10" {
		t.Fatalf("14 days old bar not taken: %+v %v", got, ok)
	}
	future := Source{Bars: []Bar{{Day: "2026-11-19", Close: "10"}}}
	if _, ok := Pick(future, asOf); ok {
		t.Fatal("bar after asOf taken")
	}
}

func TestPickNonPositiveIgnored(t *testing.T) {
	t.Parallel()
	s := Source{
		Bars: []Bar{{Day: "2026-11-17", Close: "0"}, {Day: "2026-11-16", Close: "12"}},
		Prints: []Print{
			{Day: "2026-11-18", Price: "-3"},
			{Day: "2026-11-18", Price: "0.00"},
			{Day: "2026-11-18", Price: ""},
		},
	}
	got, ok := Pick(s, "2026-11-18")
	if !ok || got != (Close{Day: "2026-11-16", Price: "12", Via: "bar"}) {
		t.Fatalf("got %+v %v", got, ok)
	}
}

func TestPickNothing(t *testing.T) {
	t.Parallel()
	if got, ok := Pick(Source{}, "2026-11-18"); ok || got != (Close{}) {
		t.Fatalf("got %+v %v", got, ok)
	}
}
