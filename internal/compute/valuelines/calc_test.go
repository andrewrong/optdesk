package valuelines

import (
	"errors"
	"sort"
	"strconv"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

var testMargin = []valuelinesapi.MarginRow{
	{PFrom: "1", M: "0.30", W: "0.10", H: "0.40", S: "0.30"},
	{PFrom: "3", M: "0.18", W: "0.07", H: "0.30", S: "0.22"},
	{PFrom: "4", M: "0.12", W: "0.05", H: "0.20", S: "0.15"},
}

func baseInput(close string) valuelinesapi.Input {
	return valuelinesapi.Input{
		V: "100", P: "3", Market: valuelinesapi.MarketUS,
		R:      valuelinesapi.RValues{US: "0.9", JP: "0.8", HKCN: "0.7"},
		Margin: testMargin, Close: close,
	}
}

func TestLinesNoRounding(t *testing.T) {
	t.Parallel()
	l, err := New().Lines(baseInput(""))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"C": {l.C, "90"}, "BuyHigh": {l.BuyHigh, "73.8"}, "BuyLow": {l.BuyLow, "69.3"},
		"Heavy": {l.Heavy, "63"}, "FairLow": {l.FairLow, "83.7"}, "FairHigh": {l.FairHigh, "96.3"},
		"Trim": {l.Trim, "109.8"}, "Clear": {l.Clear, "120"}, "Core": {l.Core, "150"},
	}
	for k, v := range want {
		if v[0] != v[1] {
			t.Errorf("%s = %s, want %s", k, v[0], v[1])
		}
	}
	in := baseInput("")
	in.V = "100.07"
	l, err = New().Lines(in)
	if err != nil {
		t.Fatal(err)
	}
	// 100.07*0.9 = 90.063; *0.82 = 73.85166; *1.22 = 109.87686; V*1.2 = 120.084
	if l.C != "90.063" || l.BuyHigh != "73.85166" || l.Trim != "109.87686" || l.Clear != "120.084" || l.Core != "150.105" {
		t.Errorf("exact products lost: %+v", l)
	}
}

func TestZoneFlagsJudgedAlone(t *testing.T) {
	t.Parallel()
	in := baseInput("")
	in.R.US = "1"
	in.Margin = []valuelinesapi.MarginRow{{PFrom: "1", M: "0.2", W: "0.1", H: "0.3", S: "0.5"}}
	// C 100, Trim 150, Clear 120, Core 150
	in.Close = "130"
	l, err := New().Lines(in)
	if err != nil {
		t.Fatal(err)
	}
	if l.Flags.AtTrim || !l.Flags.AtClear || l.Zone != "clear" {
		t.Errorf("close 130: %+v zone %s", l.Flags, l.Zone)
	}
	in.Close = "125"
	in.Margin[0].S = "0.1" // trim 110 below clear 120
	l, _ = New().Lines(in)
	if !l.Flags.AtTrim || !l.Flags.AtClear || l.Zone != "clear" {
		t.Errorf("close 125: %+v zone %s", l.Flags, l.Zone)
	}
	in.Margin[0].S = "0.5" // trim 150 above clear 120
	in.Close = "125"
	l, _ = New().Lines(in)
	if l.Flags.AtTrim || !l.Flags.AtClear {
		t.Errorf("between: %+v", l.Flags)
	}
	in.Close = "160"
	l, _ = New().Lines(in)
	if !l.Flags.AtTrim || !l.Flags.AtClear || !l.Flags.AtCore || l.Zone != "core" {
		t.Errorf("above all: %+v zone %s", l.Flags, l.Zone)
	}
	in.Margin[0].S = "2" // trim 300, close 160 above clear and core only
	l, _ = New().Lines(in)
	if l.Flags.AtTrim || !l.Flags.AtClear || l.Zone != "core" {
		t.Errorf("trim above core: %+v zone %s", l.Flags, l.Zone)
	}
	in.Margin[0].S = "0.1"
	in.Close = "115" // trim only (110), clear 120 not reached
	l, _ = New().Lines(in)
	if l.Zone != "trim" || l.Flags.AtClear {
		t.Errorf("trim only: %+v zone %s", l.Flags, l.Zone)
	}
	in.Close = "95" // fair 90..110
	l, _ = New().Lines(in)
	if l.Zone != "fair" || !l.Flags.InFair {
		t.Errorf("fair: %+v zone %s", l.Flags, l.Zone)
	}
}

func TestLineBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		close, zone string
		heavy, buy  bool
	}{
		{"66", "buy", false, true},
		{"63", "heavy", true, true},
		{"50", "heavy", true, true},
		{"69.3", "buy", false, true},
		{"73.8", "buy", false, true},
		{"73.80001", "between", false, false},
		{"80", "between", false, false},
		{"83.7", "fair", false, false},
		{"96.3", "fair", false, false},
		{"109.8", "trim", false, false},
		{"120", "clear", false, false},
		{"150", "core", false, false},
	}
	for _, c := range cases {
		l, err := New().Lines(baseInput(c.close))
		if err != nil {
			t.Fatal(err)
		}
		if l.Zone != c.zone || l.Flags.AtHeavy != c.heavy || l.Flags.InBuy != c.buy {
			t.Errorf("close %s: zone %s flags %+v", c.close, l.Zone, l.Flags)
		}
	}
}

func TestHKCNHasNoZone(t *testing.T) {
	t.Parallel()
	in := baseInput("70")
	in.Market = valuelinesapi.MarketHKCN
	l, err := New().Lines(in)
	if err != nil {
		t.Fatal(err)
	}
	if l.C != "70" || l.Zone != "" || l.Flags != nil || l.ZoneNote != "no_zone_market" {
		t.Errorf("%+v", l)
	}
}

func TestNoCloseNoZone(t *testing.T) {
	t.Parallel()
	l, err := New().Lines(baseInput(""))
	if err != nil {
		t.Fatal(err)
	}
	if l.ZoneNote != "no_close" || l.Zone != "" || l.Flags != nil {
		t.Errorf("%+v", l)
	}
}

func TestBadInput(t *testing.T) {
	t.Parallel()
	mod := map[string]func(*valuelinesapi.Input){
		"v0":       func(i *valuelinesapi.Input) { i.V = "0" },
		"vneg":     func(i *valuelinesapi.Input) { i.V = "-1" },
		"p0.5":     func(i *valuelinesapi.Input) { i.P = "0.5" },
		"p5.5":     func(i *valuelinesapi.Input) { i.P = "5.5" },
		"nomargin": func(i *valuelinesapi.Input) { i.Margin = testMargin[1:]; i.P = "2" },
		"market":   func(i *valuelinesapi.Input) { i.Market = "XX" },
		"text":     func(i *valuelinesapi.Input) { i.V = "abc" },
	}
	for name, f := range mod {
		in := baseInput("")
		f(&in)
		if _, err := New().Lines(in); !errors.Is(err, valuelinesapi.ErrBadInput) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

func TestMarginRowChosenByP(t *testing.T) {
	t.Parallel()
	for p, m := range map[string]string{"1": "0.3", "2.5": "0.3", "3": "0.18", "3.5": "0.18", "4": "0.12", "5": "0.12"} {
		in := baseInput("")
		in.P = p
		l, err := New().Lines(in)
		if err != nil {
			t.Fatal(err)
		}
		if l.M != m {
			t.Errorf("P %s: M %s, want %s", p, l.M, m)
		}
	}
}

func TestMarketColumnChosen(t *testing.T) {
	t.Parallel()
	for mk, c := range map[string]string{"US": "90", "JP": "80", "HKCN": "70"} {
		in := baseInput("")
		in.Market = mk
		l, err := New().Lines(in)
		if err != nil {
			t.Fatal(err)
		}
		if l.C != c {
			t.Errorf("%s: C %s", mk, l.C)
		}
	}
}

func TestBandPositions(t *testing.T) {
	t.Parallel()
	l, _ := New().Lines(baseInput("80"))
	b := New().Band(l, "80")
	if len(b.Marks) != 9 {
		t.Fatalf("marks %d", len(b.Marks))
	}
	checkBand(t, b, 63, 150)

	// trim above clear keeps both marks
	in := baseInput("80")
	in.Margin = []valuelinesapi.MarginRow{{PFrom: "1", M: "0.2", W: "0.1", H: "0.3", S: "2"}}
	l, _ = New().Lines(in)
	b = New().Band(l, "80")
	checkBand(t, b, 63, 270)
	names := map[string]bool{}
	for _, m := range b.Marks {
		names[m.Name] = true
	}
	if !names["trim"] || !names["clear"] {
		t.Errorf("marks lost: %v", names)
	}
}

func checkBand(t *testing.T, b valuelinesapi.Band, lo, hi float64) {
	t.Helper()
	mn, _ := strconv.ParseFloat(b.Min, 64)
	mx, _ := strconv.ParseFloat(b.Max, 64)
	if !(mn < lo) || !(mx > hi) {
		t.Errorf("band %s..%s does not hold %v..%v", b.Min, b.Max, lo, hi)
	}
	var vals []float64
	for _, m := range b.Marks {
		v, _ := strconv.ParseFloat(m.Value, 64)
		p, _ := strconv.ParseFloat(m.Pos, 64)
		if p < 0 || p > 1 {
			t.Errorf("%s pos %s", m.Name, m.Pos)
		}
		vals = append(vals, v)
	}
	if !sort.Float64sAreSorted(vals) {
		t.Errorf("marks out of order: %v", vals)
	}
}

func TestBandCloseOutside(t *testing.T) {
	t.Parallel()
	l, _ := New().Lines(baseInput(""))
	b := New().Band(l, "10")
	checkBand(t, b, 10, 150)
	if b.Marks[0].Name != "close" {
		t.Errorf("first mark %s", b.Marks[0].Name)
	}
}

func TestChangePercent(t *testing.T) {
	t.Parallel()
	got, err := New().ChangePercent("100", "88")
	if err != nil || got != "-12" {
		t.Errorf("%q %v", got, err)
	}
	if _, err := New().ChangePercent("0", "5"); !errors.Is(err, valuelinesapi.ErrBadInput) {
		t.Errorf("err %v", err)
	}
}
