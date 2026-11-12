package flex_import

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/pages"
)

func TestPageIsReadOnly(t *testing.T) {
	t.Parallel()
	p := Page()
	if want := (pages.Meta{Name: "flex_import", Schema: "page.flex_import.v1"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	out, err := p.Build(t.Context(), nil, "2026-11-12", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	want, err := Build(Input{Gaps: []Gap{{Kind: "gap", Reason: "no_reader", Text: readOnlyGapText}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(want) {
		t.Fatalf("got %s", out)
	}
	validate(t, out)
}

func TestUnknownStatusOrReasonIsError(t *testing.T) {
	t.Parallel()
	bad := []Input{
		{Form: testForm, Files: []File{{Name: "a", Reason: "disk_full"}}},
		{Form: testForm, Recent: []Recent{{AtText: "t", Status: "weird"}}},
		{Form: testForm, Recent: []Recent{{AtText: "t", Status: "failed", Reason: "disk_full"}}},
		{Form: testForm, Recent: []Recent{{AtText: "t", Status: "failed"}}},
		{Form: testForm, Recent: []Recent{{AtText: "t", Status: "stored", Reason: "not_report"}}},
	}
	for i, in := range bad {
		if out, err := Build(in); err == nil {
			t.Fatalf("case %d: no error, got %s", i, out)
		}
	}
}
