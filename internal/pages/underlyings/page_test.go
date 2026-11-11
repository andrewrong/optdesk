package underlyings

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/pages"
)

func TestPage(t *testing.T) {
	t.Parallel()
	p := Page()
	if want := (pages.Meta{Name: "underlyings", Schema: "page.underlyings.v1"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	out, err := p.Build(t.Context(), nil, "2026-11-04", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, out)
	if m["session_day"] != "2026-11-04" || m["form"] != nil {
		t.Fatalf("got %s", out)
	}
	validate(t, out)
}

func TestBuildNilRows(t *testing.T) {
	t.Parallel()
	out, err := Build(Input{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, out)
	if rows, ok := m["rows"].([]any); !ok || len(rows) != 0 {
		t.Fatalf("rows = %#v", m["rows"])
	}
	if m["sentence"] != "0 names." {
		t.Fatalf("sentence = %v", m["sentence"])
	}
	if m["session_day"] != "2000-01-01" {
		t.Fatalf("session_day = %v", m["session_day"])
	}
	validate(t, out)
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
