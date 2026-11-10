package option_candidates_page

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// namesFake answers the names it holds and records the calls.
type namesFake struct {
	mu    sync.Mutex
	names map[int]string
	err   error
	calls [][]int64
}

func (f *namesFake) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]int64(nil), ids...))
	if f.err != nil {
		return nil, f.err
	}
	var out []store.InstrumentName
	for _, id := range ids {
		if n, ok := f.names[int(id)]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstrumentID < out[j].InstrumentID })
	return out, nil
}

func withUnderlying(kind string, id int, ul int) store.Candidate {
	c := cand(kind, id, "ok")
	c.UnderlyingID = ul
	c.Issuer = "cik:0000320193"
	return c
}

func companyTexts(t *testing.T, names store.InstrumentNameReader, rows []store.Candidate) []string {
	t.Helper()
	c := cfg(fakeReader{rows: map[string][]store.Candidate{"2026-11-09/short_put": rows}})
	c.Names = names
	var p outPage
	if err := json.Unmarshal(buildBytes(t, c, buildDay), &p); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range p.Groups[0].Main.Rows {
		out = append(out, r.Cells["company"].Text)
	}
	return out
}

func TestCompanyCellTicker(t *testing.T) {
	t.Parallel()
	f := &namesFake{names: map[int]string{7: "AAPL", 8: "MSFT"}}
	rows := []store.Candidate{withUnderlying("short_put", 1, 7), withUnderlying("short_put", 2, 8), withUnderlying("short_put", 3, 7)}
	got := companyTexts(t, f, rows)
	want := []string{"AAPL", "MSFT", "AAPL"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d company = %q, want %q", i, got[i], want[i])
		}
	}
	if len(f.calls) != 1 || len(f.calls[0]) != 2 {
		t.Errorf("calls = %v, want one call with 2 distinct ids", f.calls)
	}
	if rows[0].Issuer != "cik:0000320193" {
		t.Errorf("input row changed: %q", rows[0].Issuer)
	}
}

func TestCompanyCellFallbackIssuer(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{withUnderlying("short_put", 1, 7), withUnderlying("short_put", 2, 8), withUnderlying("short_put", 3, 9)}
	for name, names := range map[string]store.InstrumentNameReader{
		"nil reader": nil,
		"missing id": &namesFake{names: map[int]string{7: "AAPL"}},
		"empty name": &namesFake{names: map[int]string{7: "", 8: "", 9: ""}},
	} {
		got := companyTexts(t, names, rows)
		for i, g := range got {
			if name == "missing id" && i == 0 {
				if g != "AAPL" {
					t.Errorf("%s: row 0 = %q", name, g)
				}
				continue
			}
			if g != "cik:0000320193" {
				t.Errorf("%s: row %d = %q, want stored issuer", name, i, g)
			}
		}
	}
}

func TestNamesReaderError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	c := cfg(fakeReader{rows: map[string][]store.Candidate{"2026-11-09/short_put": {withUnderlying("short_put", 1, 7)}}})
	c.Names = &namesFake{err: boom}
	_, err := Build(context.Background(), c, nil, buildDay, time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}
