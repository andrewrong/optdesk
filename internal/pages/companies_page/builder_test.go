package companies_page

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNilReadersGiveGaps(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Read, cfg.Facts, cfg.Band = nil, nil, nil
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		WatchList struct {
			Rows []struct {
				Cells []testCell `json:"cells"`
			} `json:"rows"`
		} `json:"watch_list"`
		Core      map[string]any `json:"core"`
		Diagnosis map[string]any `json:"diagnosis"`
		Questions map[string]any `json:"questions"`
		Gaps      []struct {
			Reason string `json:"reason"`
		} `json:"gaps"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if c := page.WatchList.Rows[0].Cells[1]; c.PriceState != "" || c.Text == "" {
		t.Fatalf("cell = %+v", c)
	}
	if len(page.Gaps) != 1 || page.Gaps[0].Reason != "no_reader" {
		t.Fatalf("gaps = %+v", page.Gaps)
	}
	for name, blk := range map[string]map[string]any{"core": page.Core, "diagnosis": page.Diagnosis, "questions": page.Questions} {
		if blk["reason"] != "not_built" {
			t.Fatalf("%s = %v, want a not_built gap", name, blk)
		}
	}
}

func TestHeldRowsCarryTheMoatText(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Held = func(context.Context) (map[string]bool, error) { return map[string]bool{"cik:0000000001": true}, nil }
	cfg.Text = fakeText{"SYN1": "brand and scale"}
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Holdings struct {
			Rows []struct {
				Moat []string `json:"moat"`
			} `json:"rows"`
		} `json:"holdings"`
		WatchList struct {
			Rows []any `json:"rows"`
		} `json:"watch_list"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Holdings.Rows) != 1 || page.Holdings.Rows[0].Moat[0] != "brand and scale" || len(page.WatchList.Rows) != 0 {
		t.Fatalf("page = %s", b)
	}
}
