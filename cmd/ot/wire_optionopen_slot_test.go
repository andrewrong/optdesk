package main

import (
	"context"
	"errors"
	"testing"
)

func TestNoOpenStoreAnswersItsErrorForASlot(t *testing.T) {
	t.Parallel()
	want := errors.New("stand-in")
	rows, err := noOpenStore{err: want}.OptionOpenSlot(context.Background(), "2026-11-19", "open", []int64{1, 2})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %v, want none", rows)
	}
}
