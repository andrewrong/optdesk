package positions_page_test

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestJPYPositionCarriesAmountUnits(t *testing.T) {
	t.Parallel()
	c, _ := positionsConfig(pos("2026-11-10", "7203", "stock", "JPY", 1, "100", "12300"))
	m, _ := build(t, marksOf(c, midMark(1, "12450")), "main")
	row := []any{"views", 0, "stocks", "rows", 0}
	want := []struct {
		col, text, unit string
	}{
		{"cost", "¥12,300.00", "amount_per_share"},
		{"price", "¥12,450.00", "amount_per_share"},
		{"unrealised", "+¥15,000.00", "amount_signed"},
	}
	for _, w := range want {
		p := append(append([]any{}, row...), w.col)
		if v := at0(t, m, append(p, "text")...); v != w.text {
			t.Errorf("%s text = %v, want %s", w.col, v, w.text)
		}
		if v := at0(t, m, append(p, "unit")...); v != w.unit {
			t.Errorf("%s unit = %v, want %s", w.col, v, w.unit)
		}
		if v := at0(t, m, append(p, "currency")...); v != "JPY" {
			t.Errorf("%s currency = %v", w.col, v)
		}
	}
}

func TestFractionalStockQuantityIsListed(t *testing.T) {
	t.Parallel()
	c, _ := positionsConfig(
		pos("2026-11-10", "AAA", "stock", "USD", 1, "0.5", "10"),
		pos("2026-11-10", "BBB", "stock", "USD", 2, "10.250", "10"),
	)
	c.Ledgers = func() compute.OptionLedger { return openLegs(nil) }
	m, _ := build(t, marksOf(c, midMark(1, "11"), midMark(2, "11")), "main")
	for i, w := range []struct {
		value float64
		text  string
	}{{0.5, "0.5"}, {10.25, "10.25"}} {
		q := at0(t, m, "views", 0, "stocks", "rows", i, "quantity").(map[string]any)
		if q["value"] != w.value || q["text"] != w.text || q["unit"] != "shares" {
			t.Errorf("row %d quantity = %v, want %v %s", i, q, w.value, w.text)
		}
	}
	if n, ok := m["notes"]; ok && len(n.([]any)) != 0 {
		t.Errorf("notes = %v, want none", n)
	}
}

func TestFractionalOptionQuantityIsSkipped(t *testing.T) {
	t.Parallel()
	c, _ := positionsConfig(
		option(pos("2026-11-10", "XYZ", "option", "USD", 3, "0.5", "260"), "put", "2026-11-20", "100"),
		pos("2026-11-10", "AAA", "stock", "USD", 1, "1.0000001", "10"),
	)
	m, _ := build(t, c, "main")
	if n := len(at0(t, m, "views", 0, "stocks", "rows").([]any)); n != 0 {
		t.Errorf("stock rows = %d", n)
	}
	if n := len(at0(t, m, "views", 0, "options", "rows").([]any)); n != 0 {
		t.Errorf("option rows = %d", n)
	}
	notes, _ := m["notes"].([]any)
	if len(notes) != 1 || notes[0] != "Skipped: 2 rows whose quantity cannot be shown are not listed" {
		t.Errorf("notes = %v", notes)
	}
}

func TestFractionalStockFillIsShown(t *testing.T) {
	t.Parallel()
	exec := func(id, sym, sec, qty string) store.Execution {
		e := store.Execution{
			Alias: "main", ExecID: id, SessionDay: "2026-11-10", Symbol: sym, SecType: sec, Currency: "USD",
			Side: "BOT", Quantity: qty, Price: "10", ExecutedAt: time.Date(2026, 11, 10, 14, 30, 0, 0, time.UTC),
		}
		if sec == "option" {
			e.Expiry, e.Strike, e.Right = "2026-11-20", "100", "put"
		}
		return e
	}
	for _, c := range []struct {
		name string
		e    store.Execution
		text string
	}{
		{"stock", exec("s1", "AAA", "stock", "0.5"), "0.5 shares"},
		{"option", exec("o1", "XYZ", "option", "0.5"), ""},
	} {
		acc := account()
		acc.execs = []store.Execution{c.e}
		m, _ := build(t, config(acc), "main")
		q := at0(t, m, "views", 0, "trades", "rows", 0, "quantity").(map[string]any)
		if c.text != "" {
			if q["text"] != c.text || q["reason"] != nil {
				t.Errorf("%s: quantity = %v, want text %q", c.name, q, c.text)
			}
			continue
		}
		if q["reason"] != "missing_input" {
			t.Errorf("%s: quantity = %v, want gap missing_input", c.name, q)
		}
	}
}

func TestGoldenJPYFractional(t *testing.T) {
	t.Parallel()
	c, _ := positionsConfig(
		pos("2026-11-10", "7203", "stock", "JPY", 1, "100", "12300"),
		pos("2026-11-10", "AAA", "stock", "USD", 2, "0.5", "10"),
		pos("2026-11-10", "BBB", "stock", "USD", 3, "10.250", "10"),
	)
	_, b := build(t, marksOf(c, midMark(1, "12450"), midMark(2, "11"), midMark(3, "9")), "main")
	validate(t, b)
	checkGolden(t, b, "jpy_fractional.golden.json")
}
