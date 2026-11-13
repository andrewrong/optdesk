package hoursnotice_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
)

// fakeStore is an in-memory hoursnotice.Store that keeps the promises of the interface.
type fakeStore struct{ rows []hoursnotice.Notice }

var _ hoursnotice.Store = (*fakeStore)(nil)

func (f *fakeStore) Notices(context.Context) ([]hoursnotice.Notice, error) {
	out := append([]hoursnotice.Notice{}, f.rows...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Market != out[j].Market {
			return out[i].Market < out[j].Market
		}
		return out[i].Day < out[j].Day
	})
	return out, nil
}

func (f *fakeStore) Replace(_ context.Context, market string, rows []hoursnotice.Notice) error {
	for _, r := range rows {
		if r.Market != market {
			return errors.New("row of another market")
		}
	}
	kept := f.rows[:0:0]
	for _, r := range f.rows {
		if r.Market != market {
			kept = append(kept, r)
		}
	}
	f.rows = append(kept, rows...)
	return nil
}

func (f *fakeStore) Close() error { return nil }

func n(market, day, ours, ibkr string) hoursnotice.Notice {
	return hoursnotice.Notice{Market: market, Day: day, Ours: ours, IBKR: ibkr}
}

// Writer side of the contract, read back through the Reader side.
func TestReplaceContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s hoursnotice.Store = &fakeStore{}
	if got, err := s.Notices(ctx); err != nil || len(got) != 0 {
		t.Fatalf("empty store = %v, %v", got, err)
	}
	us := []hoursnotice.Notice{n("XNYS", "2026-11-27", "0930-1300", "CLOSED"), n("XNYS", "2026-11-26", "0930-1600", "CLOSED")}
	jp := []hoursnotice.Notice{n("XTKS", "2026-11-24", "0900-1530", "0900-1500")}
	if err := s.Replace(ctx, "XNYS", us); err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(ctx, "XTKS", jp); err != nil {
		t.Fatal(err)
	}

	// Repeating a replace changes nothing; rows come ordered by market and day.
	if err := s.Replace(ctx, "XNYS", us); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Notices(ctx)
	want := []hoursnotice.Notice{us[1], us[0], jp[0]}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("rows = %+v, want %+v", got, want)
	}

	// Fewer rows: the missing day is gone, the other market stays.
	if err := s.Replace(ctx, "XNYS", us[:1]); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Notices(ctx)
	if len(got) != 2 || got[0] != us[0] || got[1] != jp[0] {
		t.Fatalf("after fewer rows = %+v", got)
	}

	// A row of another market fails the call and changes nothing.
	if err := s.Replace(ctx, "XNYS", []hoursnotice.Notice{jp[0]}); err == nil {
		t.Error("row of another market accepted")
	}
	if got, _ = s.Notices(ctx); len(got) != 2 {
		t.Errorf("failed replace changed rows: %+v", got)
	}

	// An empty replace clears the market only.
	if err := s.Replace(ctx, "XNYS", nil); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Notices(ctx); len(got) != 1 || got[0] != jp[0] {
		t.Errorf("after empty replace = %+v", got)
	}
}

// Reader side: a Store is usable where only a Reader is wanted.
func TestReaderIsASubsetOfStore(t *testing.T) {
	t.Parallel()
	var r hoursnotice.Reader = &fakeStore{rows: []hoursnotice.Notice{n("XTKS", "2026-11-24", "a", "b"), n("XNYS", "2026-11-27", "c", "d")}}
	got, err := r.Notices(context.Background())
	if err != nil || len(got) != 2 || got[0].Market != "XNYS" {
		t.Fatalf("reader = %+v, %v", got, err)
	}
}
