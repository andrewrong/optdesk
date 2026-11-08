package macro_rates

import (
	"bytes"
	"testing"

	"github.com/andrewrong/optdesk/internal/pages"
)

func TestPage(t *testing.T) {
	t.Parallel()
	p := Page()
	if want := (pages.Meta{Name: "macro_rates", Schema: "page.macro_rates.v3"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	demo := sevenReader(t, "percent", "index_points")
	got, err := p.Build(t.Context(), demo, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(decode(t, got)["series"].([]any)); n != 7 {
		t.Fatalf("series = %d, want 7", n)
	}
	want, err := Build(t.Context(), demo, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %s want %s", got, want)
	}
}
