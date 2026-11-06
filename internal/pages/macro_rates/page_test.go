package macro_rates

import (
	"bytes"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
)

func TestPage(t *testing.T) {
	t.Parallel()
	p := Page()
	if want := (pages.Meta{Name: "macro_rates", Schema: "page.macro_rates.v1"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	demo := fakeReader{points: map[string][]contracts.SeriesPoint{"DGS10": {
		pt(t, "2026-11-02", f64(4.1)),
		pt(t, "2026-11-03", nil),
		pt(t, "2026-11-04", f64(4.2)),
	}}}
	got, err := p.Build(t.Context(), demo, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Build(t.Context(), demo, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %s want %s", got, want)
	}
}
