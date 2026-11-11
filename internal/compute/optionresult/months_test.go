package optionresult

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func TestMonthsByClosedOn(t *testing.T) {
	t.Parallel()
	got := summarise(
		pos("3", "closed", "USD", "-1", "2", "7", "2026-02-03"),
		pos("1", "closed", "USD", "-1", "2", "10", "2026-01-31"),
		pos("2", "expired", "USD", "-1", "2", "5", "2026-01-02"),
	).Sections[0].Months
	if len(got) != 2 || got[0].Month != "2026-01" || got[0].Realized != "15.0000" || got[1].Month != "2026-02" || got[1].Realized != "7.0000" {
		t.Fatalf("months %+v", got)
	}
}

func TestRunningSum(t *testing.T) {
	t.Parallel()
	got := summarise(
		pos("1", "closed", "USD", "-1", "2", "10", "2026-01-31"),
		pos("2", "closed", "USD", "-1", "2", "-25", "2026-02-03"),
		pos("3", "closed", "USD", "-1", "2", "4", "2026-03-01"),
	)
	var run []string
	for _, m := range got.Sections[0].Months {
		run = append(run, m.Running)
	}
	if want := []string{"10.0000", "-15.0000", "-11.0000"}; !reflect.DeepEqual(run, want) {
		t.Fatalf("running %v", run)
	}
}

func TestOpenMonthNotOver(t *testing.T) {
	t.Parallel()
	in := compute.ResultInput{AsOf: "2026-02-10", Positions: []compute.LedgerPosition{
		pos("1", "closed", "USD", "-1", "2", "10", "2026-01-31"),
		pos("2", "closed", "USD", "-1", "2", "5", "2026-02-03"),
	}}
	m := New().Summarise(in).Sections[0].Months
	if !m[0].Over || m[1].Over {
		t.Fatalf("months %+v", m)
	}
	in.AsOf = "2026-03-01"
	if m = New().Summarise(in).Sections[0].Months; !m[1].Over {
		t.Fatalf("months %+v", m)
	}
}

func TestMonthsPremium(t *testing.T) {
	t.Parallel()
	sec := summarise(
		pos("1", "closed", "USD", "-1", "2", "10", "2026-01-05"),
		pos("2", "expired", "USD", "-2", "1.5", "5", "2026-01-20"),
		pos("3", "assigned", "USD", "-3", "1", "7", "2026-02-03"),
	).Sections[0]
	if len(sec.Months) != 2 || sec.Months[0].Premium != "500.0000" || sec.Months[1].Premium != "300.0000" {
		t.Fatalf("months %+v", sec.Months)
	}
	if sec.Premium != "800.0000" {
		t.Fatalf("section premium %q", sec.Premium)
	}
}

func TestMonthsPremiumSumsToSection(t *testing.T) {
	t.Parallel()
	for _, sec := range summarise(
		pos("1", "closed", "USD", "-1", "2", "10", "2026-01-05"),
		pos("2", "exercised", "USD", "-1", "3", "5", "2026-02-20"),
		pos("3", "closed", "USD", "1", "4", "-1", "2026-02-09"),
	).Sections {
		sum := new(big.Rat)
		for _, m := range sec.Months {
			v, ok := parse(m.Premium)
			if !ok {
				t.Fatalf("month premium %q", m.Premium)
			}
			sum.Add(sum, v)
		}
		if fixed(sum, 4) != sec.Premium {
			t.Fatalf("%s months sum %s, section %s", sec.Side, fixed(sum, 4), sec.Premium)
		}
	}
}

func TestMonthsPremiumBoughtPositive(t *testing.T) {
	t.Parallel()
	sec := summarise(pos("1", "closed", "USD", "1", "1.1", "-40", "2026-03-02")).Sections[0]
	if sec.Months[0].Premium != "110.0000" {
		t.Fatalf("months %+v", sec.Months)
	}
}
