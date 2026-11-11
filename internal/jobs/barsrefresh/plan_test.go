package barsrefresh

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

func TestPlanOneKeyPerEntryAndIndex(t *testing.T) {
	t.Parallel()
	j, _ := build(t, &fakeHistory{answer: fixed()}, &memBars{}, entry("cik:0000000001", "AAA", 101), entry("edinet:E1", "BBB", 102))
	keys, hash, err := j.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"cik:0000000001", "edinet:E1", "index:SPX", "index:NDX", "index:RUT", "index:N225"}
	if len(keys) != len(want) {
		t.Fatalf("keys %+v", keys)
	}
	for i, k := range keys {
		if k.SubjectKey != want[i] || k.DatasetID != "obs.price_bar" {
			t.Fatalf("key %d is %+v", i, k)
		}
	}
	_, again, _ := j.Plan(context.Background(), runDay)
	_, other, _ := j.Plan(context.Background(), contracts.Day("2026-11-10"))
	if hash == "" || hash != again || hash == other {
		t.Fatalf("hash %q %q %q", hash, again, other)
	}
}

func TestPlanIssuerlessEntryKeyedBySymbol(t *testing.T) {
	t.Parallel()
	j, _ := build(t, &fakeHistory{answer: fixed()}, &memBars{}, entry("", "SPY", 103), entry("cik:0000000001", "AAA", 101))
	keys, _, err := j.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sym:SPY", "cik:0000000001", "index:SPX", "index:NDX", "index:RUT", "index:N225"}
	if len(keys) != len(want) {
		t.Fatalf("keys %+v", keys)
	}
	for i, k := range keys {
		if k.SubjectKey != want[i] {
			t.Fatalf("key %d is %+v", i, k)
		}
	}
}

func TestTwoIssuerlessEntriesOfOneSymbolAreDuplicate(t *testing.T) {
	t.Parallel()
	_, err := New(&fakeBroker{listed: true}, &fakeHistory{answer: fixed()}, fakeResolver{}, &memBars{}, &memBars{},
		[]jobs.WatchEntry{entry("", "SPY", 103), entry("", "SPY", 104)}, fixedClock{})
	if err == nil {
		t.Fatal("want a duplicate key error")
	}
}
