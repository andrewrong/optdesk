package option_data_page

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeTerms struct {
	rows  []store.OptionTerms
	err   error
	mu    sync.Mutex
	calls [][]int64
}

func (f *fakeTerms) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]int64(nil), ids...))
	return f.rows, f.err
}

func TestBuildTermsGolden(t *testing.T) {
	t.Parallel()
	in := loadInput(t)
	e, q := fromInput(in)
	var recs []store.OptionTerms
	if err := json.Unmarshal(fixture(t, "terms-input"), &recs); err != nil {
		t.Fatal(err)
	}
	c := inputCfg(in, e, q)
	c.Notes = []string{"Synthetic fixture."}
	c.Symbol, c.Terms = "SYNTH", &fakeTerms{rows: recs}
	b, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, b, "terms")
}

func TestBuildTerms(t *testing.T) {
	t.Parallel()
	t.Run("one call with kept ids in row order, at most 500", func(t *testing.T) {
		var rows []contracts.OptionEod
		for i := 1; i <= 600; i++ {
			rows = append(rows, eodRow(t, i, f64(float64(i)), "2026-11-06T21:00:00Z"))
		}
		ft := &fakeTerms{}
		c := cfg(&fakeEod{rows: rows}, &fakeQuotes{})
		c.Terms = ft
		m := buildMap(t, c)
		if len(ft.calls) != 1 || len(ft.calls[0]) != 500 {
			t.Fatalf("calls = %d", len(ft.calls))
		}
		for i, id := range ids(m) {
			if float64(ft.calls[0][i]) != id {
				t.Fatalf("id %d = %d, row has %v", i, ft.calls[0][i], id)
			}
		}
	})
	t.Run("row without a record keeps null and gap", func(t *testing.T) {
		ft := &fakeTerms{rows: []store.OptionTerms{{InstrumentID: 1, Right: "P", Strike: "12.5", ExpirationDate: "2026-12-18"}}}
		c := cfg(&fakeEod{rows: []contracts.OptionEod{
			eodRow(t, 1, f64(5), "2026-11-06T21:00:00Z"), eodRow(t, 2, f64(1), "2026-11-06T21:00:00Z")}}, &fakeQuotes{})
		c.Terms = ft
		rows := rowsOf(buildMap(t, c))
		a, b := rows[0].(map[string]any), rows[1].(map[string]any)
		if a["right"] != "put" || a["expiry"] != "2026-12-18" || a["strike"].(map[string]any)["text"] != "12.50" {
			t.Fatalf("row 1 = %v", a)
		}
		if b["right"] != nil || b["expiry"] != nil || b["strike"].(map[string]any)["reason"] != "no_data" {
			t.Fatalf("row 2 = %v", b)
		}
	})
	t.Run("terms error returns no bytes", func(t *testing.T) {
		c := cfg(&fakeEod{rows: []contracts.OptionEod{eodRow(t, 1, f64(1), "2026-11-06T21:00:00Z")}}, &fakeQuotes{})
		c.Terms = &fakeTerms{err: errors.New("down")}
		b, err := Build(t.Context(), c, "2026-11-06", fixedAt)
		if err == nil || b != nil {
			t.Fatalf("err = %v, bytes = %d", err, len(b))
		}
	})
	t.Run("empty row set makes no terms call", func(t *testing.T) {
		ft := &fakeTerms{}
		c := cfg(&fakeEod{}, &fakeQuotes{})
		c.Terms = ft
		buildMap(t, c)
		if len(ft.calls) != 0 {
			t.Fatalf("calls = %d", len(ft.calls))
		}
	})
}
