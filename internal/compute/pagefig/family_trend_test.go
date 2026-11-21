package pagefig_test

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func dv(day, v, cur string) blockapi.DayValue {
	return blockapi.DayValue{Day: contracts.Day(day), NetLiquidation: v, Currency: cur}
}

func days(ds ...string) []contracts.Day {
	var out []contracts.Day
	for _, d := range ds {
		out = append(out, contracts.Day(d))
	}
	return out
}

func pts(ps ...string) []blockapi.FamilyTrendPoint {
	var out []blockapi.FamilyTrendPoint
	for i := 0; i < len(ps); i += 2 {
		out = append(out, blockapi.FamilyTrendPoint{Day: contracts.Day(ps[i]), Net: ps[i+1]})
	}
	return out
}

func TestFamilyTrendSumsSameDay(t *testing.T) {
	t.Parallel()
	r := pagefig.NewFamilyTrend().FamilyTrend(blockapi.FamilyTrendIn{
		Accounts: []blockapi.FamilyTrendAccountIn{
			{Alias: "a", Days: []blockapi.DayValue{dv("2026-11-16", "100.005", "USD"), dv("2026-11-17", "200.015", "USD"), dv("2026-11-18", "300", "USD")}},
			{Alias: "b", Days: []blockapi.DayValue{dv("2026-11-16", "10.10", "USD"), dv("2026-11-17", "20.20", "USD"), dv("2026-11-18", "30.30", "USD")}},
		},
		Window: days("2026-11-16", "2026-11-17", "2026-11-18"),
	})
	wantA := pts("2026-11-16", "100.00", "2026-11-17", "200.02", "2026-11-18", "300.00")
	wantF := pts("2026-11-16", "110.10", "2026-11-17", "220.22", "2026-11-18", "330.30")
	if !reflect.DeepEqual(r.Family, wantF) || !reflect.DeepEqual(r.Accounts[0].Points, wantA) || r.Accounts[1].Alias != "b" ||
		r.LeftOut != 0 || r.YenUsed || r.Gap != "" {
		t.Fatalf("trend %+v", r)
	}
}

func TestFamilyTrendDropsDayWithMissingMember(t *testing.T) {
	t.Parallel()
	r := pagefig.NewFamilyTrend().FamilyTrend(blockapi.FamilyTrendIn{
		Accounts: []blockapi.FamilyTrendAccountIn{
			{Alias: "a", Days: []blockapi.DayValue{dv("2026-11-16", "1", "USD"), dv("2026-11-17", "2", "USD"), dv("2026-11-18", "3", "USD")}},
			{Alias: "b", Days: []blockapi.DayValue{dv("2026-11-16", "10", "USD"), dv("2026-11-18", "30", "USD")}},
		},
		Window: days("2026-11-16", "2026-11-17", "2026-11-18"),
	})
	if !reflect.DeepEqual(r.Family, pts("2026-11-16", "11.00", "2026-11-18", "33.00")) || r.LeftOut != 1 ||
		len(r.Accounts[0].Points) != 3 || len(r.Accounts[1].Points) != 2 || r.Gap != "" {
		t.Fatalf("trend %+v", r)
	}
}

func TestFamilyTrendYenUsesDayRate(t *testing.T) {
	t.Parallel()
	in := blockapi.FamilyTrendIn{
		Accounts: []blockapi.FamilyTrendAccountIn{
			{Alias: "u", Days: []blockapi.DayValue{dv("2026-11-16", "100", "USD"), dv("2026-11-17", "100", "USD"), dv("2026-11-18", "100", "USD")}},
			{Alias: "y", Days: []blockapi.DayValue{dv("2026-11-16", "15000", "JPY"), dv("2026-11-17", "15000", "JPY"), dv("2026-11-18", "15000", "JPY")}},
		},
		UsdJpy: map[contracts.Day]string{"2026-11-16": "150", "2026-11-18": "100", "2026-11-17": "0"},
		Window: days("2026-11-16", "2026-11-17", "2026-11-18"),
	}
	r := pagefig.NewFamilyTrend().FamilyTrend(in)
	if !reflect.DeepEqual(r.Accounts[1].Points, pts("2026-11-16", "100.00", "2026-11-18", "150.00")) ||
		!reflect.DeepEqual(r.Family, pts("2026-11-16", "200.00", "2026-11-18", "250.00")) || r.LeftOut != 1 || !r.YenUsed {
		t.Fatalf("trend %+v", r)
	}
	in.UsdJpy = nil
	r = pagefig.NewFamilyTrend().FamilyTrend(in)
	if r.YenUsed || len(r.Accounts[1].Points) != 0 || len(r.Family) != 0 || r.LeftOut != 3 {
		t.Fatalf("no rate %+v", r)
	}
}

func TestFamilyTrendBadValues(t *testing.T) {
	t.Parallel()
	r := pagefig.NewFamilyTrend().FamilyTrend(blockapi.FamilyTrendIn{
		Accounts: []blockapi.FamilyTrendAccountIn{{Alias: "a", Days: []blockapi.DayValue{
			dv("2026-11-16", "-1", "USD"), dv("2026-11-17", "", "USD"), dv("2026-11-18", "abc", "USD"),
			dv("2026-11-19", "5", "EUR"), dv("2026-11-20", "5", ""), dv("2026-11-21", "7", "USD"),
		}}},
		Window: days("2026-11-16", "2026-11-17", "2026-11-18", "2026-11-19", "2026-11-20", "2026-11-21"),
	})
	if !reflect.DeepEqual(r.Accounts[0].Points, pts("2026-11-21", "7.00")) || len(r.Family) != 1 || r.LeftOut != 5 {
		t.Fatalf("trend %+v", r)
	}
}

func TestFamilyTrendGap(t *testing.T) {
	t.Parallel()
	mk := func(n int) blockapi.FamilyTrendResult {
		d := []blockapi.DayValue{dv("2026-11-16", "1", "USD"), dv("2026-11-17", "2", "USD")}[:n]
		return pagefig.NewFamilyTrend().FamilyTrend(blockapi.FamilyTrendIn{
			Accounts: []blockapi.FamilyTrendAccountIn{{Alias: "a", Days: d}},
			Window:   days("2026-11-16", "2026-11-17"),
		})
	}
	if g := mk(1).Gap; g != "no_data" {
		t.Fatalf("1 point gap %q", g)
	}
	if g := mk(2).Gap; g != "" {
		t.Fatalf("2 points gap %q", g)
	}
	if g := pagefig.NewFamilyTrend().FamilyTrend(blockapi.FamilyTrendIn{}).Gap; g != "no_data" {
		t.Fatalf("empty gap %q", g)
	}
}
