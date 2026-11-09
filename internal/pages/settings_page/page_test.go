package settings_page

import (
	"bytes"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/pages"
)

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	c := baseConfig()
	c.Confirm = &Confirm{Rows: []ConfirmRow{{Label: "a", Old: "1", New: "2"}}, Form: settingsapi.FormToken{Expires: "1", MAC: macText}}
	p := NewPage(c)
	if want := (pages.Meta{Name: "settings_page", Schema: "page.settings_page.v2"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	got, err := p.Build(t.Context(), nil, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Build(t.Context(), c, nil, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("page build differs from Build:\n%s\n%s", got, want)
	}
	c.Confirm.Rows[0].New = "changed"
	again, err := p.Build(t.Context(), nil, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, want) {
		t.Fatalf("page follows the caller's changed confirm:\n%s", again)
	}
}
