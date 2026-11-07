package option_data_page

import (
	"bytes"
	"testing"

	"github.com/andrewrong/optdesk/internal/pages"
)

func TestNewPage(t *testing.T) {
	t.Parallel()
	e := &fakeEod{rows: nil}
	c := cfg(e, &fakeQuotes{})
	p := NewPage(c)
	if want := (pages.Meta{Name: "option_data_page", Schema: "page.option_data_page.v1"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	got, err := p.Build(t.Context(), nil, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("page build differs from Build:\n%s\n%s", got, want)
	}
}

func TestNewPageCopiesConfig(t *testing.T) {
	t.Parallel()
	c := cfg(&fakeEod{}, &fakeQuotes{})
	p := NewPage(c)
	c.Label = "changed"
	c.Zone = nil
	got, err := p.Build(t.Context(), nil, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatalf("page uses the caller's changed config: %v", err)
	}
	if !bytes.Contains(got, []byte(`"label":"Synthetic underlying"`)) {
		t.Fatalf("label changed: %s", got)
	}
}

func TestNewPageCopiesNotes(t *testing.T) {
	t.Parallel()
	c := cfg(&fakeEod{}, &fakeQuotes{})
	c.Notes = []string{"kept"}
	p := NewPage(c)
	c.Notes[0] = "changed"
	got, err := p.Build(t.Context(), nil, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`"notes":["kept"]`)) {
		t.Fatalf("notes changed: %s", got)
	}
}
