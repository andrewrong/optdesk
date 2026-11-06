package main

import (
	"testing"
	"time"
)

func TestDayDefaultIsNewYork(t *testing.T) {
	t.Parallel()
	d, err := dayOrDefault(fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)}, "")
	if err != nil || d != "2026-11-05" {
		t.Fatalf("day %q err %v, want 2026-11-05", d, err)
	}
}

func TestDayFlagWins(t *testing.T) {
	t.Parallel()
	d, err := dayOrDefault(fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)}, "2025-01-31")
	if err != nil || d != "2025-01-31" {
		t.Fatalf("day %q err %v", d, err)
	}
}

func TestDayMalformed(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"2026-13-01", "20261105", "x", "2026-11-5"} {
		if _, err := dayOrDefault(fakeClock{}, f); err == nil {
			t.Errorf("flag %q accepted", f)
		}
	}
}
