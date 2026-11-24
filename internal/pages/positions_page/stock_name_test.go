package positions_page_test

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/pages/positions_page"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

type fakeUnderlyings struct {
	rows []underlying.Row
	err  error
}

func (f fakeUnderlyings) Current(context.Context) ([]underlying.Row, error) { return f.rows, f.err }
func (fakeUnderlyings) History(context.Context, int) ([]underlying.Row, error) {
	return nil, nil
}

func stockNames(t *testing.T, c positions_page.Config) []any {
	t.Helper()
	m, _ := build(t, c, "main")
	rows := at0(t, m, "views", 0, "stocks", "rows").([]any)
	var out []any
	for i := range rows {
		if code := at0(t, m, "views", 0, "stocks", "rows", i, "code", "text"); code == "" {
			t.Fatal("empty code")
		}
		out = append(out, at0(t, m, "views", 0, "stocks", "rows", i, "name", "text"))
	}
	return out
}

func TestStockNameIsCompanyName(t *testing.T) {
	t.Parallel()
	c := config(account())
	c.Underlyings = fakeUnderlyings{rows: []underlying.Row{{Key: "US:AAA", Symbol: "AAA", Name: "Alpha Corp"}, {Key: "US:BBB", Symbol: "BBB"}}}
	m, _ := build(t, c, "main")
	rows := at0(t, m, "views", 0, "stocks", "rows").([]any)
	if len(rows) == 0 {
		t.Fatal("no stock row")
	}
	for i := range rows {
		code := at0(t, m, "views", 0, "stocks", "rows", i, "code", "text")
		name := at0(t, m, "views", 0, "stocks", "rows", i, "name", "text")
		value := at0(t, m, "views", 0, "stocks", "rows", i, "name", "value")
		want := positions_page.TextNoStoredName
		if code == "AAA" {
			want = "Alpha Corp"
		}
		if name != want || value != want {
			t.Fatalf("%v: name = %v value = %v, want %q", code, name, value, want)
		}
		if name == code {
			t.Fatalf("name of %v is the symbol", code)
		}
	}
	for name, u := range map[string]underlying.Reader{"nil": nil, "error": fakeUnderlyings{err: errors.New("down")}} {
		c.Underlyings = u
		for _, n := range stockNames(t, c) {
			if n != positions_page.TextNoStoredName {
				t.Fatalf("%s registry: name = %v", name, n)
			}
		}
	}
}
