package price_lines_page

import (
	"encoding/json"
	"testing"

	"github.com/andrewrong/optdesk/internal/pages"
)

func TestPageMeta(t *testing.T) {
	t.Parallel()
	p := Page()
	if want := (pages.Meta{Name: "price_lines_page", Schema: "page.price_lines_page.v1"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	got, err := p.Build(t.Context(), nil, "2026-11-14", fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	o := out{}
	if err := json.Unmarshal(got, &o); err != nil {
		t.Fatal(err)
	}
	if len(o.Gaps) != 1 || o.Gaps[0].Reason != "no_reader" || len(o.Rows) != 0 {
		t.Fatalf("page = %s", got)
	}
}
