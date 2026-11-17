package valuation_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// fake keeps rows in memory and keeps the promises of the interface: insert-only, the newest profile per key,
// ErrClosed and ErrNotEffective, and the current-valuation rule of Decision 8.
type fake struct {
	profiles []valuation.Profile
	reviews  []valuation.Review
	rvers    []valuation.RVersion
	clock    time.Time
}

var (
	_ valuation.Reader = (*fake)(nil)
	_ valuation.Writer = (*fake)(nil)
)

func (f *fake) tick() time.Time { f.clock = f.clock.Add(time.Second); return f.clock }

func (f *fake) PutProfile(_ context.Context, p valuation.Profile) error {
	p.ID, p.SetAt = int64(len(f.profiles)+1), f.tick()
	f.profiles = append(f.profiles, p)
	return nil
}

func (f *fake) PutReview(_ context.Context, r valuation.Review) error {
	hasEff, hasVoid := false, false
	for _, e := range f.reviews {
		if e.ReviewID == r.ReviewID {
			hasEff = hasEff || e.Status == "effective"
			hasVoid = hasVoid || e.Status == "void"
		}
	}
	if r.Status == "void" && !hasEff {
		return valuation.ErrNotEffective
	}
	if hasEff && r.Status != "void" || hasVoid {
		return valuation.ErrClosed
	}
	r.ID, r.EnteredAt = int64(len(f.reviews)+1), f.tick()
	if r.Status == "effective" {
		t := r.EnteredAt
		r.EffectiveAt = &t
	}
	f.reviews = append(f.reviews, r)
	return nil
}

func (f *fake) PutRVersion(_ context.Context, v valuation.RVersion) error {
	v.ID, v.SetAt = int64(len(f.rvers)+1), f.tick()
	f.rvers = append(f.rvers, v)
	return nil
}
func (f *fake) PutRTable(context.Context, valuation.RTableVersion) error           { return nil }
func (f *fake) PutMarginTable(context.Context, valuation.MarginTableVersion) error { return nil }

func (f *fake) Current(context.Context) (valuation.State, error) {
	newest := map[string]valuation.Profile{}
	for _, p := range f.profiles {
		newest[p.CompanyKey] = p // later rows win
	}
	st := valuation.State{Entries: []valuation.Entry{}}
	for _, p := range newest {
		st.Entries = append(st.Entries, valuation.Entry{Profile: p, Current: f.current(p.CompanyKey)})
	}
	sort.Slice(st.Entries, func(i, j int) bool { return st.Entries[i].Profile.CompanyKey < st.Entries[j].Profile.CompanyKey })
	return st, nil
}

func (f *fake) current(key string) *valuation.Review {
	voided := map[string]bool{}
	for _, r := range f.reviews {
		if r.Status == "void" {
			voided[r.ReviewID] = true
		}
	}
	var best *valuation.Review
	for i, r := range f.reviews {
		if r.CompanyKey != key || r.Status != "effective" || r.Type == "backfill" || voided[r.ReviewID] {
			continue
		}
		if best == nil || r.EffectiveAt.After(*best.EffectiveAt) {
			best = &f.reviews[i]
		}
	}
	if best == nil {
		return nil
	}
	c := *best
	return &c
}

func (f *fake) Reviews(_ context.Context, key string) ([]valuation.Review, error) {
	out := []valuation.Review{}
	for i := len(f.reviews) - 1; i >= 0; i-- {
		if f.reviews[i].CompanyKey == key {
			out = append(out, f.reviews[i])
		}
	}
	return out, nil
}

func (f *fake) RVersions(context.Context) ([]valuation.RVersion, error) {
	out := []valuation.RVersion{}
	for i := len(f.rvers) - 1; i >= 0; i-- {
		out = append(out, f.rvers[i])
	}
	return out, nil
}

func newFake() *fake { return &fake{clock: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)} }

func review(id, status string) valuation.Review {
	return valuation.Review{ReviewID: id, CompanyKey: "US:AAA", Type: "first", VersionDate: contracts.Day("2026-01-05"),
		Status: status, Source: "owner", V: "100", P: "3.5"}
}

func TestReaderWriterContractFake(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFake()
	var w valuation.Writer = f
	var r valuation.Reader = f

	if err := w.PutProfile(ctx, valuation.Profile{CompanyKey: "US:AAA", Name: "Old", RMarket: "US", Pool: "in_pool"}); err != nil {
		t.Fatal(err)
	}
	if err := w.PutProfile(ctx, valuation.Profile{CompanyKey: "US:AAA", Name: "Alpha", RMarket: "US", Pool: "watch"}); err != nil {
		t.Fatal(err)
	}
	for _, rv := range []valuation.Review{review("r1", "draft"), review("r1", "effective")} {
		if err := w.PutReview(ctx, rv); err != nil {
			t.Fatal(err)
		}
	}
	st, err := r.Current(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Entries) != 1 || st.Entries[0].Profile.Name != "Alpha" {
		t.Fatalf("entries = %+v, want the newest profile row only", st.Entries)
	}
	if c := st.Entries[0].Current; c == nil || c.Status != "effective" || c.V != "100" {
		t.Fatalf("current = %+v, want the effective review", c)
	}
	if err := w.PutReview(ctx, review("r1", "draft")); !errors.Is(err, valuation.ErrClosed) {
		t.Fatalf("draft after effective: err = %v, want ErrClosed", err)
	}
	if err := w.PutReview(ctx, review("r2", "void")); !errors.Is(err, valuation.ErrNotEffective) {
		t.Fatalf("void without effective: err = %v, want ErrNotEffective", err)
	}
	void := review("r1", "void")
	void.VoidReason = "typo"
	if err := w.PutReview(ctx, void); err != nil {
		t.Fatal(err)
	}
	rows, err := r.Reviews(ctx, "US:AAA")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Status != "void" || rows[2].Status != "draft" {
		t.Fatalf("reviews = %+v, want void, effective, draft (newest first)", rows)
	}
}

func TestVoidedReviewIsNotCurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFake()
	_ = f.PutProfile(ctx, valuation.Profile{CompanyKey: "US:AAA"})
	_ = f.PutReview(ctx, review("r1", "effective"))
	void := review("r1", "void")
	void.VoidReason = "wrong V"
	_ = f.PutReview(ctx, void)
	st, _ := f.Current(ctx)
	if st.Entries[0].Current != nil {
		t.Fatalf("current = %+v, want nil after the void row", st.Entries[0].Current)
	}
}

func TestEveryMethodReturnsNonNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var r valuation.Reader = newFake()
	st, err := r.Current(ctx)
	if err != nil || st.Entries == nil {
		t.Fatalf("Current: entries = %v, err = %v, want a non-nil slice", st.Entries, err)
	}
	if rows, err := r.Reviews(ctx, "none"); err != nil || rows == nil {
		t.Fatalf("Reviews: rows = %v, err = %v, want a non-nil slice", rows, err)
	}
	if vs, err := r.RVersions(ctx); err != nil || vs == nil {
		t.Fatalf("RVersions: rows = %v, err = %v, want a non-nil slice", vs, err)
	}
}
