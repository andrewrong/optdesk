package barsrefresh

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
)

func TestWindowDays(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	ago := func(n int, src gen.PriceBarSource) contracts.PriceBar {
		r := stored(101, day.AddDate(0, 0, -n).Format(dayLayout), "10.0", 1)
		r.Source = src
		return r
	}
	cases := []struct {
		name string
		rows []contracts.PriceBar
		want int
	}{
		{"no bar", nil, 400},
		{"newest bar yesterday", []contracts.PriceBar{ago(1, "ibkr")}, 5},
		{"newest bar eight days back", []contracts.PriceBar{ago(30, "ibkr"), ago(8, "ibkr")}, 8},
		{"yfinance nine days back, newer ibkr", []contracts.PriceBar{ago(9, "yfinance"), ago(3, "ibkr"), ago(1, "ibkr")}, 10},
		{"gap longer than backfill", []contracts.PriceBar{ago(500, "ibkr")}, 400},
	}
	for _, c := range cases {
		if got := windowDays(c.rows, day, 5, 400); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestDailyComplete(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	row := func(n int, src gen.PriceBarSource) contracts.PriceBar {
		r := stored(101, day.AddDate(0, 0, -n).Format(dayLayout), "10.0", 1)
		r.Source = src
		return r
	}
	cases := []struct {
		name string
		rows []contracts.PriceBar
		want bool
	}{
		{"newest is the run day", []contracts.PriceBar{row(3, "ibkr"), row(0, "ibkr")}, true},
		{"newest a day before", []contracts.PriceBar{row(2, "ibkr"), row(1, "ibkr")}, false},
		{"yfinance row with the run day", []contracts.PriceBar{row(2, "yfinance"), row(0, "ibkr")}, false},
		{"no rows", nil, false},
	}
	for _, c := range cases {
		if got := dailyComplete(c.rows, day); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
