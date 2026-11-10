package advice_page_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/advice_page"
)

var (
	zone = time.FixedZone("owner", 8*3600)
	at   = time.Date(2026, 11, 10, 2, 0, 0, 0, time.UTC)
)

type fakeSessions struct {
	rows []contracts.SessionDay
	err  error
}

func (f fakeSessions) SessionDays(_ context.Context, _ string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	return f.rows, f.err
}

func sd(s string, is bool) contracts.SessionDay {
	d, _ := time.Parse("2006-01-02", s)
	return contracts.SessionDay{CalendarID: "XNYS", SessionDay: types.SerializableDate{Time: d}, IsSession: is}
}

func build(t *testing.T, c advice_page.Config, day contracts.Day) []byte {
	t.Helper()
	c.RunID, c.BuildID, c.Zone = "run1", "b1", zone
	b, err := advice_page.NewPage(c).Build(context.Background(), nil, day, at)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	return b
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", filepath.Base(url)))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile("https://optdesk.invalid/schemas/page.advice_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Fatalf("invalid: %v\n%s", err, b)
	}
}

func TestGapGolden(t *testing.T) {
	t.Parallel()
	got := build(t, advice_page.Config{}, "2026-11-10")
	want, err := os.ReadFile(filepath.Join("testdata", "gap.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestDay(t *testing.T) {
	t.Parallel()
	rows := []contracts.SessionDay{sd("2026-11-05", true), sd("2026-11-06", true), sd("2026-11-07", false), sd("2026-11-09", false)}
	cases := []struct {
		name string
		c    advice_page.Config
		want string
	}{
		{"previous session day", advice_page.Config{Sessions: fakeSessions{rows: rows}}, `"day":"2026-11-06"`},
		{"no previous session day", advice_page.Config{Sessions: fakeSessions{}}, `"day":"2026-11-09"`},
		{"nil reader", advice_page.Config{}, `"day":"2026-11-09"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := string(build(t, tc.c, "2026-11-10"))
			if !strings.Contains(b, tc.want) || !strings.Contains(b, `"reason":"not_built"`) || strings.Contains(b, "no_reader") {
				t.Fatalf("got %s, want %s with not_built", b, tc.want)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	p := advice_page.NewPage(advice_page.Config{Sessions: fakeSessions{err: errors.New("boom")}, Zone: zone})
	if _, err := p.Build(context.Background(), nil, "2026-11-10", at); err == nil {
		t.Fatal("reader error must be an error")
	}
	if _, err := advice_page.NewPage(advice_page.Config{}).Build(context.Background(), nil, "2026-11-10", at); err == nil {
		t.Fatal("nil zone must be an error")
	}
}
