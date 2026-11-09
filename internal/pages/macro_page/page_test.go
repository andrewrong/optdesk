package macro_page

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
)

func pageConfig() Config {
	return Config{
		RunID: "run-1", BuildID: "build-1", Scope: "macro", Zone: time.UTC,
		Indicators: []Indicator{{SeriesID: "DGS10", Name: "ten-year", SourceText: "FRED"}},
	}
}

func pageReader(t *testing.T) fakeReader {
	t.Helper()
	return fakeReader{points: map[string][]contracts.SeriesPoint{"DGS10": {
		pt(t, "2026-11-04", "2026-11-04", "percent", f64(4.2)),
	}}}
}

func TestNewPage(t *testing.T) {
	t.Parallel()
	c := pageConfig()
	p := NewPage(c)
	if want := (pages.Meta{Name: "macro_page", Schema: "page.macro_page.v1"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	r := pageReader(t)
	got, err := p.Build(t.Context(), r, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Build(t.Context(), r, c, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestNewPageCopiesConfig(t *testing.T) {
	t.Parallel()
	c := pageConfig()
	p := NewPage(c)
	r := pageReader(t)
	before, err := p.Build(t.Context(), r, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	c.Indicators[0] = Indicator{SeriesID: "DGS2", Name: "two-year", SourceText: "other"}
	after, err := p.Build(t.Context(), r, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("before %s after %s", before, after)
	}
}

func goldenReader(t *testing.T) fakeReader {
	t.Helper()
	return fakeReader{points: map[string][]contracts.SeriesPoint{
		"DGS10": {pt(t, "2026-11-03", "2026-11-04", "percent", f64(4.25)), pt(t, "2026-11-04", "2026-11-05", "percent", f64(4.3))},
		"DGS2":  {pt(t, "2026-11-04", "2026-11-05", "percent", nil)},
	}}
}

func goldenConfig() Config {
	c := cfg("DGS10", "DGS2")
	c.Indicators[0].DetailHref = "/indicator/DGS10"
	return c
}

func TestConfigReadNilUnchanged(t *testing.T) {
	t.Parallel()
	c := goldenConfig()
	c.Read = nil
	b, err := Build(t.Context(), goldenReader(t), c, "2026-11-05", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Fatalf("got\n%s\nwant\n%s", b, golden)
	}
}

func TestConfigReadWithoutReadingUnchanged(t *testing.T) {
	t.Parallel()
	calls := 0
	c := goldenConfig()
	c.Read = func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		calls++
		return nil, nil
	}
	b, err := Build(t.Context(), goldenReader(t), c, "2026-11-05", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Fatalf("got\n%s\nwant\n%s", b, golden)
	}
	if calls == 0 {
		t.Fatal("Read was not called")
	}
}
