package valuelines

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

func goodCheck() valuelinesapi.CheckInput {
	return valuelinesapi.CheckInput{Type: "first", VersionDate: "2026-01-02", V: "100", P: "3.5", Margin: testMargin}
}

func TestCheckRequiredAndRanges(t *testing.T) {
	t.Parallel()
	if got := New().Check(goodCheck()); len(got) != 0 {
		t.Fatalf("good: %v", got)
	}
	cases := []struct {
		field string
		mod   func(*valuelinesapi.CheckInput)
	}{
		{"type", func(c *valuelinesapi.CheckInput) { c.Type = "" }},
		{"version_date", func(c *valuelinesapi.CheckInput) { c.VersionDate = "" }},
		{"v", func(c *valuelinesapi.CheckInput) { c.V = "" }},
		{"p", func(c *valuelinesapi.CheckInput) { c.P = "" }},
		{"v", func(c *valuelinesapi.CheckInput) { c.V = "0" }},
		{"p", func(c *valuelinesapi.CheckInput) { c.P = "3.3" }},
		{"p", func(c *valuelinesapi.CheckInput) { c.P = "5.5" }},
	}
	for _, c := range cases {
		in := goodCheck()
		c.mod(&in)
		got := New().Check(in)
		if len(got) != 1 || got[0].Field != c.field {
			t.Errorf("%s: %v", c.field, got)
		}
	}
}

func TestCheckBuyBelowFair(t *testing.T) {
	t.Parallel()
	in := goodCheck()
	in.Margin = []valuelinesapi.MarginRow{
		{PFrom: "1", M: "0.30", W: "0.10", H: "0.40", S: "0.30"},
		{PFrom: "3", M: "0.07", W: "0.07", H: "0.30", S: "0.22"},
	}
	got := New().Check(in)
	if len(got) != 1 || got[0].Field != "margin" || !strings.Contains(got[0].Text, "P 3") {
		t.Errorf("%v", got)
	}
	in.P = "2"
	if got := New().Check(in); len(got) != 0 {
		t.Errorf("other row fails: %v", got)
	}
}

func TestCheckIgnoresOptionalFields(t *testing.T) {
	t.Parallel()
	in := valuelinesapi.CheckInput{Type: "first", VersionDate: "2026-01-02", V: "100", P: "3"}
	if got := New().Check(in); len(got) != 0 {
		t.Errorf("%v", got)
	}
}
