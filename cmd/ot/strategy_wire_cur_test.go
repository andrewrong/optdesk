package main

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/candidatecur"
)

// curOldStore is an old store that also reads rows by currency.
type curOldStore struct {
	kindDayCandidates
	rows []candidatecur.Row
}

func (c *curOldStore) CandidatesIn(context.Context, contracts.Day, string, string) ([]candidatecur.Row, error) {
	return c.rows, nil
}

func TestResultCandidatesPassYenReader(t *testing.T) {
	t.Parallel()
	want := candidatecur.Row{RunID: "jp1", Currency: "JPY"}
	old := &curOldStore{rows: []candidatecur.Row{want}}
	opt, err := openResultCandidateWith(
		func(context.Context, string) (resultStore, error) { return &fakeResultStore{}, nil },
		func(context.Context, string) (candidateStore, error) { return old, nil })(t.Context(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	rd, ok := opt.(candidatecur.Reader)
	if !ok {
		t.Fatal("the opened store does not hold candidatecur.Reader")
	}
	got, err := rd.CandidatesIn(t.Context(), "2026-11-19", "short_put", "JPY")
	if err != nil || len(got) != 1 || got[0].RunID != "jp1" || got[0].Currency != "JPY" {
		t.Fatalf("CandidatesIn = %+v, %v", got, err)
	}
}

func TestResultCandidatesWithoutYenReaderStayNil(t *testing.T) {
	t.Parallel()
	opt, err := cutoverOpener(&fakeResultStore{}, &kindDayCandidates{})(t.Context(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := opt.(candidatecur.Reader); ok {
		t.Fatal("a store without CandidatesIn must not hold candidatecur.Reader")
	}
}
