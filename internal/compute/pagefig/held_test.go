package pagefig

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func hv(s string) compute.FigureValue { return compute.FigureValue{Value: s} }

func hline(alias, sym, v, day string) blockapi.HeldLineIn {
	return blockapi.HeldLineIn{Alias: alias, Underlying: sym, Currency: "USD", Value: hv(v), MarkDay: contracts.Day(day)}
}

func TestHeldMergesAccounts(t *testing.T) {
	t.Parallel()
	r := NewHeld().HeldCompanies(blockapi.HeldIn{
		Lines:    []blockapi.HeldLineIn{hline("wife", "AAA", "10.50", "2026-11-20"), hline("main", "AAA", "20", "2026-11-18")},
		TotalUSD: hv("1000"),
	})
	if len(r.Rows) != 1 {
		t.Fatalf("rows %+v", r.Rows)
	}
	w := r.Rows[0]
	if len(w.Aliases) != 2 || w.Aliases[0] != "wife" || w.Aliases[1] != "main" || w.Value.Value != "30.50" || w.OldestMark != "2026-11-18" {
		t.Fatalf("row %+v", w)
	}
}

func TestHeldGapLine(t *testing.T) {
	t.Parallel()
	r := NewHeld().HeldCompanies(blockapi.HeldIn{
		Lines: []blockapi.HeldLineIn{
			hline("main", "AAA", "10", "2026-11-20"),
			{Alias: "main", Underlying: "BBB", Currency: "USD", Value: compute.FigureValue{Gap: "missing_quote"}},
			hline("wife", "BBB", "5", "2026-11-20"),
		},
		TotalUSD: hv("100"),
	})
	b := r.Rows[1]
	if b.Underlying != "BBB" || b.Value.Gap != "missing_quote" || b.Share.Gap != "missing_quote" {
		t.Fatalf("gap row %+v", b)
	}
	if r.RowsSum.Value != "10.00" {
		t.Fatalf("sum %+v", r.RowsSum)
	}
}

func TestHeldShare(t *testing.T) {
	t.Parallel()
	in := blockapi.HeldIn{
		Lines: []blockapi.HeldLineIn{
			hline("main", "AAA", "25", "2026-11-20"),
			hline("main", "SHORT", "-5", "2026-11-20"),
			{Alias: "main", Underlying: "GAP", Currency: "USD", Value: compute.FigureValue{Gap: "missing_quote"}},
		},
		TotalUSD: hv("100"),
	}
	r := NewHeld().HeldCompanies(in)
	if r.Rows[0].Share.Value != "25.00" || r.Rows[1].Underlying != "SHORT" || r.Rows[1].Value.Value != "-5.00" ||
		r.Rows[1].Share.Value != "-5.00" || r.Rows[2].Underlying != "GAP" {
		t.Fatalf("rows %+v", r.Rows)
	}
	in.TotalUSD = compute.FigureValue{Gap: "no_data"}
	for _, w := range NewHeld().HeldCompanies(in).Rows[:2] {
		if w.Share.Gap != "no_data" || w.Share.Value != "" {
			t.Fatalf("share without total %+v", w)
		}
	}
}

func TestHeldEarnings(t *testing.T) {
	t.Parallel()
	row := func(st, rep, obs string) blockapi.HeldEarningsRow {
		return blockapi.HeldEarningsRow{Status: st, Report: contracts.Day(rep), ObservedOn: contracts.Day(obs)}
	}
	ok := func(e blockapi.HeldEarningsIn) blockapi.HeldCompanyIn {
		return blockapi.HeldCompanyIn{Underlying: "AAA", Currency: "USD", Resolve: "ok", Earnings: e}
	}
	cases := []struct {
		name   string
		c      blockapi.HeldCompanyIn
		next   string
		days   int
		reason string
	}{
		{"listed", ok(blockapi.HeldEarningsIn{Read: true, Rows: []blockapi.HeldEarningsRow{row("listed", "2026-11-25", "2026-11-20")}}), "2026-11-25", 4, ""},
		{"today", ok(blockapi.HeldEarningsIn{Read: true, Rows: []blockapi.HeldEarningsRow{row("listed", "2026-11-21", "2026-11-20")}}), "2026-11-21", 0, ""},
		{"newest wins", ok(blockapi.HeldEarningsIn{Read: true, Rows: []blockapi.HeldEarningsRow{row("listed", "2026-11-25", "2026-11-19"), row("absent", "", "2026-11-20")}}), "", 0, "no_row"},
		{"tie keeps later", ok(blockapi.HeldEarningsIn{Read: true, Rows: []blockapi.HeldEarningsRow{row("listed", "2026-11-25", "2026-11-20"), row("error", "", "2026-11-20")}}), "", 0, "row_error"},
		{"override", ok(blockapi.HeldEarningsIn{Read: true, Override: "2026-12-01", Rows: []blockapi.HeldEarningsRow{row("listed", "2026-11-25", "2026-11-20")}}), "2026-12-01", 10, ""},
		{"past", ok(blockapi.HeldEarningsIn{Read: true, Rows: []blockapi.HeldEarningsRow{row("listed", "2026-11-20", "2026-11-20")}}), "", 0, "past_date"},
		{"past override", ok(blockapi.HeldEarningsIn{Read: true, Override: "2026-11-01"}), "", 0, "past_date"},
		{"absent", ok(blockapi.HeldEarningsIn{Read: true, Rows: []blockapi.HeldEarningsRow{row("absent", "", "2026-11-20")}}), "", 0, "no_row"},
		{"no rows", ok(blockapi.HeldEarningsIn{Read: true}), "", 0, "no_row"},
		{"unsupported", ok(blockapi.HeldEarningsIn{Read: true, Rows: []blockapi.HeldEarningsRow{row("unsupported", "", "2026-11-20")}}), "", 0, "unsupported"},
		{"no reader", ok(blockapi.HeldEarningsIn{}), "", 0, "no_reader"},
		{"no underlying", blockapi.HeldCompanyIn{Underlying: "AAA", Currency: "USD", Resolve: "no_underlying", Earnings: blockapi.HeldEarningsIn{Read: true}}, "", 0, "no_underlying"},
		{"no issuer", blockapi.HeldCompanyIn{Underlying: "AAA", Currency: "USD", Resolve: "no_issuer", Earnings: blockapi.HeldEarningsIn{Read: true}}, "", 0, "no_issuer"},
		{"other resolve", blockapi.HeldCompanyIn{Underlying: "AAA", Currency: "USD", Resolve: "?", Earnings: blockapi.HeldEarningsIn{Read: true}}, "", 0, "no_underlying"},
	}
	for _, tc := range cases {
		r := NewHeld().HeldCompanies(blockapi.HeldIn{
			Lines:     []blockapi.HeldLineIn{hline("main", "AAA", "10", "2026-11-20")},
			Companies: []blockapi.HeldCompanyIn{tc.c},
			TotalUSD:  hv("100"), Today: "2026-11-21",
		})
		w := r.Rows[0]
		if string(w.Next) != tc.next || w.DaysTo != tc.days || w.EarnReason != tc.reason {
			t.Errorf("%s: got %q %d %q", tc.name, w.Next, w.DaysTo, w.EarnReason)
		}
	}
}

func TestHeldSortOrder(t *testing.T) {
	t.Parallel()
	r := NewHeld().HeldCompanies(blockapi.HeldIn{
		Lines: []blockapi.HeldLineIn{
			hline("main", "ZZZ", "10", "2026-11-20"),
			{Alias: "main", Underlying: "AAG", Currency: "USD", Value: compute.FigureValue{Gap: "missing_quote"}},
			hline("main", "BBB", "10", "2026-11-20"),
			hline("main", "CCC", "50", "2026-11-20"),
			{Alias: "main", Underlying: "AAH", Currency: "USD", Value: compute.FigureValue{Gap: "missing_quote"}},
		},
		TotalUSD: hv("100"),
	})
	var got []string
	for _, w := range r.Rows {
		got = append(got, w.Underlying)
	}
	want := []string{"CCC", "BBB", "ZZZ", "AAG", "AAH"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order %v want %v", got, want)
		}
	}
}
