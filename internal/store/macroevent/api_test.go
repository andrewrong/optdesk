package macroevent_test

import (
	"context"
	"io"
	"sort"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// fakeStore keeps the promises of the interface in memory.
type fakeStore struct {
	rows map[[2]string]macroevent.Event // key: kind, date
	held map[string]contracts.Day       // kind -> held day
}

var (
	_ macroevent.Writer         = (*fakeStore)(nil)
	_ macroevent.Reader         = (*fakeStore)(nil)
	_ macroevent.OpenWriterFunc = func(context.Context, string) (macroevent.Writer, io.Closer, error) { return nil, nil, nil }
	_ macroevent.OpenReaderFunc = func(context.Context, string) (macroevent.Reader, io.Closer, error) { return nil, nil, nil }
)

func (f *fakeStore) ReplaceFuture(_ context.Context, kind, source string, runDay contracts.Day, events []macroevent.Event) (macroevent.Outcome, error) {
	var out macroevent.Outcome
	latest, stored := contracts.Day(""), contracts.Day("")
	in := map[contracts.Day]bool{}
	for _, e := range events {
		in[e.Date] = true
		if e.Date > latest {
			latest = e.Date
		}
	}
	for _, r := range f.rows {
		if r.Kind == kind && r.Source == source && r.Date > stored {
			stored = r.Date
		}
	}
	for _, e := range events {
		k := [2]string{kind, string(e.Date)}
		if o, ok := f.rows[k]; !ok || o != e {
			f.rows[k] = e
			out.Written++
		}
	}
	if len(events) == 0 || latest < stored {
		out.Held = true
		f.held[kind] = runDay
		return out, nil
	}
	for k, r := range f.rows {
		if r.Kind == kind && r.Source == source && r.Date > runDay && !in[r.Date] {
			delete(f.rows, k)
			out.Deleted++
		}
	}
	delete(f.held, kind)
	return out, nil
}

func (f *fakeStore) Between(_ context.Context, from, to contracts.Day) ([]macroevent.Event, error) {
	out := []macroevent.Event{}
	for _, r := range f.rows {
		if r.Date >= from && r.Date <= to {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

func (f *fakeStore) Kinds(context.Context) ([]macroevent.Kind, error) {
	by := map[[2]string]*macroevent.Kind{}
	for _, r := range f.rows {
		k := by[[2]string{r.Kind, r.Source}]
		if k == nil {
			k = &macroevent.Kind{Kind: r.Kind, Source: r.Source, HeldDay: f.held[r.Kind]}
			by[[2]string{r.Kind, r.Source}] = k
		}
		k.Rows++
		if r.RunDay > k.LastRunDay {
			k.LastRunDay = r.RunDay
		}
		if r.Date > k.LastDate {
			k.LastDate = r.Date
		}
	}
	out := []macroevent.Kind{}
	for _, k := range by {
		out = append(out, *k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out, nil
}

func ev(kind, date, run string) macroevent.Event {
	return macroevent.Event{Kind: kind, Source: "fred", SourceRef: "1", Date: contracts.Day(date), RunDay: contracts.Day(run)}
}

func TestMacroEventContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &fakeStore{rows: map[[2]string]macroevent.Event{}, held: map[string]contracts.Day{}}
	var w macroevent.Writer = f
	var r macroevent.Reader = f

	cpi := []macroevent.Event{ev("cpi", "2026-12-10", "2026-11-14"), ev("cpi", "2026-11-20", "2026-11-14")}
	if o, err := w.ReplaceFuture(ctx, "cpi", "fred", "2026-11-14", cpi); err != nil || o.Written != 2 {
		t.Fatalf("first call: %+v %v", o, err)
	}
	if o, err := w.ReplaceFuture(ctx, "cpi", "fred", "2026-11-14", cpi); err != nil || o.Written != 0 || o.Deleted != 0 {
		t.Fatalf("the same call twice writes 0 the second time: %+v %v", o, err)
	}
	if _, err := w.ReplaceFuture(ctx, "ppi", "fred", "2026-11-15", []macroevent.Event{ev("ppi", "2026-11-20", "2026-11-15")}); err != nil {
		t.Fatal(err)
	}

	checkBetween(t, r)

	// A shorter answer is held; Kinds shows the latest run day, date and held day of each kind.
	if o, _ := w.ReplaceFuture(ctx, "cpi", "fred", "2026-11-16", nil); !o.Held {
		t.Fatalf("an empty answer is held: %+v", o)
	}
	checkKinds(t, r)
}

func checkBetween(t *testing.T, r macroevent.Reader) {
	t.Helper()
	got, err := r.Between(context.Background(), "2026-11-01", "2026-12-31")
	if err != nil || len(got) != 3 {
		t.Fatalf("between: %v %v", got, err)
	}
	want := [][2]string{{"2026-11-20", "cpi"}, {"2026-11-20", "ppi"}, {"2026-12-10", "cpi"}}
	for i, e := range got {
		if string(e.Date) != want[i][0] || e.Kind != want[i][1] {
			t.Fatalf("date then kind order: %v", got)
		}
	}
}

func checkKinds(t *testing.T, r macroevent.Reader) {
	t.Helper()
	ks, err := r.Kinds(context.Background())
	if err != nil || len(ks) != 2 {
		t.Fatalf("kinds: %v %v", ks, err)
	}
	c := ks[0]
	if c.Kind != "cpi" || c.LastRunDay != "2026-11-14" || c.LastDate != "2026-12-10" || c.HeldDay != "2026-11-16" || c.Rows != 2 {
		t.Fatalf("cpi kind: %+v", c)
	}
	if p := ks[1]; p.Kind != "ppi" || p.LastRunDay != "2026-11-15" || p.HeldDay != "" {
		t.Fatalf("ppi kind: %+v", p)
	}
}
