package fundkey_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
)

// fakeStore keeps the promises of the interfaces: a differing row replaces the
// stored one, an equal row is counted unchanged, replay keeps a stored answer.
// Rows are told apart by ErrorCode (value for facts).
type fakeStore struct {
	earn  map[string]string
	div   map[string]string
	facts map[string]string
}

var (
	_ fundkey.EarningsUpserter = (*fakeStore)(nil)
	_ fundkey.DividendUpserter = (*fakeStore)(nil)
	_ fundkey.FactUpserter     = (*fakeStore)(nil)
)

func put(m *map[string]string, k, v string, replace bool, c *fundkey.WriteCount) {
	if *m == nil {
		*m = map[string]string{}
	}
	old, ok := (*m)[k]
	if ok && (!replace || old == v) {
		c.Unchanged++
		return
	}
	(*m)[k] = v
	c.Written++
}

func (f *fakeStore) earnings(rows []contracts.EarningsDate, replace bool) fundkey.WriteCount {
	var c fundkey.WriteCount
	for _, r := range rows {
		put(&f.earn, r.Issuer+"|"+r.ObservedOn.String(), r.ErrorCode, replace, &c)
	}
	return c
}

func (f *fakeStore) UpsertEarnings(_ context.Context, rows []contracts.EarningsDate, _ string) (fundkey.WriteCount, error) {
	return f.earnings(rows, true), nil
}

func (f *fakeStore) ReplayEarnings(_ context.Context, rows []contracts.EarningsDate, _ string) (fundkey.WriteCount, error) {
	return f.earnings(rows, false), nil
}

func (f *fakeStore) dividends(rows []contracts.DividendExpectation, replace bool) fundkey.WriteCount {
	var c fundkey.WriteCount
	for _, r := range rows {
		put(&f.div, r.Issuer, r.ErrorCode, replace, &c)
	}
	return c
}

func (f *fakeStore) UpsertDividends(_ context.Context, rows []contracts.DividendExpectation, _ string) (fundkey.WriteCount, error) {
	return f.dividends(rows, true), nil
}

func (f *fakeStore) ReplayDividends(_ context.Context, rows []contracts.DividendExpectation, _ string) (fundkey.WriteCount, error) {
	return f.dividends(rows, false), nil
}

func (f *fakeStore) UpsertFacts(_ context.Context, rows []contracts.FundamentalFact, _ string) (fundkey.WriteCount, error) {
	var c fundkey.WriteCount
	for _, r := range rows {
		put(&f.facts, r.Concept+"|"+r.DocID, r.Value, true, &c)
	}
	return c, nil
}

func check(t *testing.T, what string, got fundkey.WriteCount, err error, written, unchanged int) {
	t.Helper()
	if err != nil || got.Written != written || got.Unchanged != unchanged {
		t.Fatalf("%s: got %+v, %v; want written %d unchanged %d", what, got, err, written, unchanged)
	}
}

func TestEarningsUpserterContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := &fakeStore{}
	var u fundkey.EarningsUpserter = fs

	got, err := u.UpsertEarnings(ctx, nil, "w")
	check(t, "empty list", got, err, 0, 0)

	a := contracts.EarningsDate{Issuer: "AAPL"}
	got, err = u.UpsertEarnings(ctx, []contracts.EarningsDate{a}, "w")
	check(t, "first write", got, err, 1, 0)

	b := contracts.EarningsDate{Issuer: "MSFT"}
	got, err = u.UpsertEarnings(ctx, []contracts.EarningsDate{a, b}, "w")
	check(t, "one equal one new", got, err, 1, 1)

	a2 := a
	a2.ErrorCode = "x"
	got, err = u.UpsertEarnings(ctx, []contracts.EarningsDate{a2}, "w")
	check(t, "differing row", got, err, 1, 0)

	got, err = u.ReplayEarnings(ctx, []contracts.EarningsDate{a, contracts.EarningsDate{Issuer: "NVDA"}}, "w")
	check(t, "replay keeps stored, adds missing", got, err, 1, 1)
	if fs.earn["AAPL|"+a.ObservedOn.String()] != "x" {
		t.Fatalf("replay replaced a stored answer")
	}
	got, err = u.ReplayEarnings(ctx, nil, "w")
	check(t, "replay empty", got, err, 0, 0)
}

func TestDividendUpserterContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := &fakeStore{}
	var u fundkey.DividendUpserter = fs

	got, err := u.UpsertDividends(ctx, nil, "w")
	check(t, "empty list", got, err, 0, 0)

	a := contracts.DividendExpectation{Issuer: "KO"}
	got, err = u.UpsertDividends(ctx, []contracts.DividendExpectation{a}, "w")
	check(t, "first write", got, err, 1, 0)

	b := contracts.DividendExpectation{Issuer: "PEP"}
	got, err = u.UpsertDividends(ctx, []contracts.DividendExpectation{a, b}, "w")
	check(t, "one equal one new", got, err, 1, 1)

	a2 := a
	a2.ErrorCode = "x"
	got, err = u.UpsertDividends(ctx, []contracts.DividendExpectation{a2}, "w")
	check(t, "differing row", got, err, 1, 0)

	got, err = u.ReplayDividends(ctx, []contracts.DividendExpectation{a, {Issuer: "JNJ"}}, "w")
	check(t, "replay keeps stored, adds missing", got, err, 1, 1)
	if fs.div["KO"] != "x" {
		t.Fatalf("replay replaced a stored answer")
	}
	got, err = u.ReplayDividends(ctx, nil, "w")
	check(t, "replay empty", got, err, 0, 0)
}

func TestFactUpserterContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := &fakeStore{}
	var u fundkey.FactUpserter = fs

	got, err := u.UpsertFacts(ctx, nil, "w")
	check(t, "empty list", got, err, 0, 0)

	a := contracts.FundamentalFact{Concept: "Revenues", DocID: "d1", Value: "10"}
	got, err = u.UpsertFacts(ctx, []contracts.FundamentalFact{a}, "w")
	check(t, "first write", got, err, 1, 0)

	b := contracts.FundamentalFact{Concept: "Assets", DocID: "d1", Value: "5"}
	got, err = u.UpsertFacts(ctx, []contracts.FundamentalFact{a, b}, "w")
	check(t, "one equal one new", got, err, 1, 1)

	a2 := a
	a2.Value = "11"
	got, err = u.UpsertFacts(ctx, []contracts.FundamentalFact{a2}, "w")
	check(t, "differing row", got, err, 1, 0)
	if fs.facts["Revenues|d1"] != "11" {
		t.Fatalf("stored value not replaced")
	}
}
