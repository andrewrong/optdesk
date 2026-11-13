package option_data_page

import (
	"bytes"
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store/runview"
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

// panicReader fails the test if the page reads it: nothing reads Config.Reasons yet.
type panicReader struct {
	runview.Reader
	t *testing.T
}

func (r panicReader) RunsOn(context.Context, contracts.Day) ([]runview.Row, error) {
	r.t.Error("RunsOn called")
	return nil, nil
}

func (r panicReader) ReasonsOn(context.Context, contracts.Day) ([]runview.Reason, error) {
	r.t.Error("ReasonsOn called")
	return nil, nil
}

func (r panicReader) NotWired(context.Context) ([]runview.Reason, error) {
	r.t.Error("NotWired called")
	return nil, nil
}

func TestConfigReasonsIsOptional(t *testing.T) {
	t.Parallel()
	with := cfg(&fakeEod{}, &fakeQuotes{})
	with.Reasons = panicReader{t: t}
	without := cfg(&fakeEod{}, &fakeQuotes{})
	a, err := Build(t.Context(), with, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(t.Context(), without, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("reader changed the page:\n%s\n%s", a, b)
	}
}
