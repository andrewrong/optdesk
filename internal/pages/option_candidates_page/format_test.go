package option_candidates_page

import "testing"

func TestFixedText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in     string
		places int
		want   string
	}{
		{"0.5", 2, "0.50"}, {"-0.5594", 2, "-0.56"}, {"1.005", 2, "1.01"}, {"0.696", 3, "0.696"},
		{"-0.001", 2, "0.00"}, {"-1.005", 2, "-1.01"}, {"12", 1, "12.0"}, {"9.96", 1, "10.0"},
	} {
		got, ok := fixedText(c.in, c.places)
		if !ok || got != c.want {
			t.Errorf("fixedText(%q, %d) = %q, %v; want %q", c.in, c.places, got, ok, c.want)
		}
	}
	for _, bad := range []string{"abc", "1e-2", "", ".5", "1.", "0.1x", "1/2"} {
		if got, ok := fixedText(bad, 2); ok {
			t.Errorf("fixedText(%q) = %q, want ok false", bad, got)
		}
	}
}

func TestYieldCellUnreadable(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"abc", "1e-2"} {
		if g, ok := yieldCell(strp(bad)).(gap); !ok || g.Reason != "unreadable" {
			t.Errorf("yieldCell(%q) = %+v", bad, yieldCell(strp(bad)))
		}
	}
}

func TestMoneyTextCurrency(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ text, cur, want string }{
		{"1105.75", "USD", "$1,105.75"},
		{"1105.75", "", "$1,105.75"},
		{"8950", "JPY", "¥8,950"},
		{"8950.5", "JPY", "¥8,951"},
		{"1790000", "JPY", "¥1,790,000"},
		{"-1234.5", "USD", "-$1,234.50"},
		{"-1234", "JPY", "-¥1,234"},
		{"0.4", "JPY", "¥0"},
	} {
		got, ok := moneyText(c.text, c.cur)
		if !ok || got != c.want {
			t.Errorf("moneyText(%q, %q) = %q, %v; want %q", c.text, c.cur, got, ok, c.want)
		}
	}
	for _, cur := range []string{"", "JPY"} {
		if got, ok := moneyText("abc", cur); ok || got != "" {
			t.Errorf("moneyText(abc, %q) = %q, %v", cur, got, ok)
		}
	}
}

func TestTimesInt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		text string
		n    int
		want string
	}{
		{"17900", 100, "1790000"},
		{"17.5", 100, "1750.0"},
		{"0.05", 100, "5.00"},
		{"0", 100, "0"},
		{"12", 0, "0"},
	} {
		got, ok := timesInt(c.text, c.n)
		if !ok || got != c.want {
			t.Errorf("timesInt(%q, %d) = %q, %v; want %q", c.text, c.n, got, ok, c.want)
		}
	}
	for _, text := range []string{"-5", "x", "", "9999999999999999"} {
		if got, ok := timesInt(text, 100); ok {
			t.Errorf("timesInt(%q) = %q, want not ok", text, got)
		}
	}
	if _, ok := timesInt("5", -1); ok {
		t.Error("negative multiplier accepted")
	}
}
