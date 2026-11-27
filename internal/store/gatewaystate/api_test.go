package gatewaystate_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/gatewaystate"
)

// fakeStore is an in-memory Reader and Writer that keeps the promises of the interface.
type fakeStore struct{ rows map[string]gatewaystate.State }

var (
	_ gatewaystate.Reader = (*fakeStore)(nil)
	_ gatewaystate.Writer = (*fakeStore)(nil)
)

func (f *fakeStore) States(context.Context) ([]gatewaystate.State, error) {
	out := make([]gatewaystate.State, 0, len(f.rows))
	for _, s := range f.rows {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out, nil
}

func (f *fakeStore) Observe(_ context.Context, alias string, down bool, at time.Time) error {
	if f.rows == nil {
		f.rows = map[string]gatewaystate.State{}
	}
	s, ok := f.rows[alias]
	switch {
	case !ok:
		f.rows[alias] = gatewaystate.State{Alias: alias, Down: down, Since: at, ObservedAt: at}
	case at.Before(s.ObservedAt):
	case s.Down == down:
		s.ObservedAt = at
		f.rows[alias] = s
	default:
		f.rows[alias] = gatewaystate.State{Alias: alias, Down: down, Since: at, ObservedAt: at}
	}
	return nil
}

func (f *fakeStore) Close() error { return nil }

func TestGatewayStateContractFake(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &fakeStore{}
	t0 := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	t2 := t1.Add(time.Minute)
	for _, o := range []struct {
		alias string
		down  bool
		at    time.Time
	}{{"wife", false, t0}, {"main", true, t0}, {"main", true, t1}, {"main", false, t2}} {
		if err := f.Observe(ctx, o.alias, o.down, o.at); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.States(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []gatewaystate.State{
		{Alias: "main", Down: false, Since: t2, ObservedAt: t2},
		{Alias: "wife", Down: false, Since: t0, ObservedAt: t0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestStatesNonNilSlice(t *testing.T) {
	t.Parallel()
	got, err := (&fakeStore{}).States(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty non-nil slice", got)
	}
}
