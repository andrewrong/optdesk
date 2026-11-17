package company_quarters_page

import "testing"

func TestGroupThousandsExact(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"1234567.80":                         "1,234,567.80",
		"123":                                "123",
		"1000":                               "1,000",
		"123456789012345678901234567890.123": "123,456,789,012,345,678,901,234,567,890.123",
		"n/a":                                "n/a",
		"1e5":                                "1e5",
	}
	for in, want := range cases {
		if got := group(in); got != want {
			t.Errorf("group(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMinusIsTypographic(t *testing.T) {
	t.Parallel()
	if got := group("-20.20"); got != "−20.20" {
		t.Fatalf("got %q", got)
	}
	if got := group("-1234.5"); got != "−1,234.5" {
		t.Fatalf("got %q", got)
	}
}

func TestEPSKeepsFiledDecimals(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"1.46", "0.100", "-0.5"} {
		got := group(s)
		if len(got) < len(s) {
			t.Errorf("group(%q) lost digits: %q", s, got)
		}
	}
	if got := group("1.46"); got != "1.46" {
		t.Fatalf("got %q", got)
	}
}
