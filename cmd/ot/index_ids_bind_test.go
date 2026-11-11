package main

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/store/instrument"
)

var _ computefigures.IndexIDReader = (*instrument.IndexIDs)(nil)

func TestIndexIDsDefaultAdapterNilOnError(t *testing.T) {
	t.Parallel()
	store, err := openIndexIDsDefault(context.Background(), "")
	if err == nil {
		t.Fatal("open with an empty dsn succeeded")
	}
	if store != nil {
		t.Fatalf("store %#v, want a nil interface", store)
	}
}
