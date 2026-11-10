package option_data_page

import (
	"bytes"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// vlSpec is one option of a vol-line test: instrument id, right, strike, expiry and stored iv (nil: null).
type vlSpec struct {
	id     int
	right  string
	strike string
	expiry contracts.Day
	iv     *float64
}

// vlCfg builds a config with the underlying price 100 and one row per spec.
func vlCfg(t *testing.T, price *float64, specs ...vlSpec) Config {
	t.Helper()
	var rows []contracts.OptionEod
	var terms []store.OptionTerms
	for _, s := range specs {
		r := eodRow(t, s.id, f64(float64(10-len(rows))), "2026-11-06T21:00:00Z")
		r.IvDecimal, r.UnderlyingPrice = s.iv, price
		rows = append(rows, r)
		terms = append(terms, store.OptionTerms{InstrumentID: int64(s.id), Right: s.right, Strike: s.strike, ExpirationDate: s.expiry})
	}
	c := cfg(&fakeEod{rows: rows}, &fakeQuotes{})
	c.Terms = &fakeTerms{rows: terms}
	c.VolLine = true
	return c
}

func volLine(t *testing.T, c Config) string {
	t.Helper()
	u := under(buildChecked(t, c))
	s, ok := u["vol_line"].(string)
	if !ok {
		t.Fatalf("vol_line missing: %v", u)
	}
	return s
}

func TestVolLineAtTheMoney(t *testing.T) {
	t.Parallel()
	c := vlCfg(t, f64(100),
		vlSpec{1, "C", "100", "2026-12-18", f64(0.5)},
		vlSpec{2, "C", "100", "2026-11-20", f64(0.241)},
		vlSpec{3, "C", "95", "2026-11-20", f64(0.3)},
		vlSpec{4, "P", "110", "2026-11-20", f64(0.4)},
		vlSpec{5, "C", "100", "2026-11-05", f64(0.9)}, // expired before the session day
	)
	if got, want := volLine(t, c), "IV 24.1% · HV30 缺 · IVP 缺"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestVolLineTieBreaks(t *testing.T) {
	t.Parallel()
	c := vlCfg(t, f64(100),
		vlSpec{1, "C", "105", "2026-11-20", f64(0.5)},
		vlSpec{2, "C", "95", "2026-11-20", f64(0.2)},
	)
	if got, want := volLine(t, c), "IV 20.0% · HV30 缺 · IVP 缺"; got != want {
		t.Fatalf("lower strike: got %q, want %q", got, want)
	}
	c = vlCfg(t, f64(100),
		vlSpec{1, "P", "100", "2026-11-20", f64(0.3)},
		vlSpec{2, "C", "100", "2026-11-20", f64(0.2)},
	)
	if got, want := volLine(t, c), "IV 20.0% · HV30 缺 · IVP 缺"; got != want {
		t.Fatalf("call first: got %q, want %q", got, want)
	}
}

func TestVolLineMissingIV(t *testing.T) {
	t.Parallel()
	c := vlCfg(t, f64(100),
		vlSpec{1, "C", "100", "2026-11-20", nil},
		vlSpec{2, "C", "101", "2026-11-20", f64(0.256)},
	)
	if got, want := volLine(t, c), "IV 25.6% · HV30 缺 · IVP 缺"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	c = vlCfg(t, f64(100), vlSpec{1, "C", "100", "2026-11-20", nil})
	if got, want := volLine(t, c), "IV 缺 · HV30 缺 · IVP 缺"; got != want {
		t.Fatalf("all null: got %q, want %q", got, want)
	}
}

func TestVolLineNoTerms(t *testing.T) {
	t.Parallel()
	const want = "IV 缺 · HV30 缺 · IVP 缺"
	c := vlCfg(t, f64(100), vlSpec{1, "C", "100", "2026-11-20", f64(0.241)})
	c.Terms = nil
	if got := volLine(t, c); got != want {
		t.Fatalf("nil terms: got %q, want %q", got, want)
	}
	c = vlCfg(t, f64(100), vlSpec{1, "C", "100", "2026-11-20", f64(0.241)})
	c.Terms = &fakeTerms{}
	if got := volLine(t, c); got != want {
		t.Fatalf("empty terms: got %q, want %q", got, want)
	}
	c = vlCfg(t, nil, vlSpec{1, "C", "100", "2026-11-20", f64(0.241)})
	if got := volLine(t, c); got != want {
		t.Fatalf("no price: got %q, want %q", got, want)
	}
}

func TestVolLineAbsentWithoutRows(t *testing.T) {
	t.Parallel()
	c := cfg(&fakeEod{}, &fakeQuotes{})
	c.VolLine = true
	if _, ok := under(buildChecked(t, c))["vol_line"]; ok {
		t.Fatal("vol_line present on an empty page")
	}

	// VolLine false: the bytes are those of a build with the same Config before the change,
	// which never wrote the field, so they hold no vol_line and equal a second build.
	c = vlCfg(t, f64(100), vlSpec{1, "C", "100", "2026-11-20", f64(0.241)})
	c.VolLine = false
	a, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(a, []byte("vol_line")) {
		t.Fatalf("vol_line present with VolLine false: %s", a)
	}
	if err := validateBytes(t, a); err != nil {
		t.Fatal(err)
	}
}
