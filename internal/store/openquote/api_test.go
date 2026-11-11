package openquote_test

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// fake keeps the promises of the interfaces in memory: the caller side of the contract.
type fake struct {
	open   []contracts.OptionOpen
	plan   []contracts.OptionOpenItem
	marks  map[openquote.NotListedKey]contracts.OptionNotListed
	refs   map[[7]string]openquote.OptionRef
	closed bool
}

var (
	_ openquote.OptionOpenWriter     = (*fake)(nil)
	_ openquote.OptionOpenReader     = (*fake)(nil)
	_ openquote.OptionOpenPlanStore  = (*fake)(nil)
	_ openquote.OptionNotListedStore = (*fake)(nil)
	_ openquote.OptionContractStore  = (*fake)(nil)
	_ openquote.Stores               = (*fake)(nil)
	_ openquote.OpenFunc             = func(context.Context, string) (openquote.Stores, error) { return &fake{}, nil }
	_ contracts.TypedError           = (*refused)(nil)
	_ interface{ Close() error }     = (*fake)(nil)
	_ openquote.OptionOpenStore      = (*fake)(nil)
	_ error                          = (*refused)(nil)
)

type refused struct{}

func (*refused) Error() string             { return "refused" }
func (*refused) Kind() contracts.ErrorKind { return contracts.Permanent }
func day(s string) types.SerializableDate {
	d, _ := time.Parse(time.DateOnly, s)
	return types.SerializableDate{Time: d}
}
func (f *fake) Close() error { f.closed = true; return nil }
func keyOf(m contracts.OptionNotListed) openquote.NotListedKey {
	return openquote.NotListedKey{UnderlyingID: int64(m.UnderlyingID), Expiry: contracts.Day(m.Expiry.Format(time.DateOnly)),
		Strike: m.Strike, Right: string(m.Right), TradingClass: m.TradingClass}
}

func (f *fake) InsertOptionOpen(_ context.Context, rows []contracts.OptionOpen) error {
	for _, r := range rows {
		if r.Slot == openquote.SlotRefresh {
			return &refused{}
		}
	}
	for _, r := range rows {
		dup := false
		for _, o := range f.open {
			if o.SessionDay == r.SessionDay && o.CaptureID == r.CaptureID && o.Slot == r.Slot && o.InstrumentID == r.InstrumentID {
				dup = true
			}
		}
		if !dup {
			f.open = append(f.open, r)
		}
	}
	return nil
}

func (f *fake) OptionOpen(_ context.Context, d contracts.Day) ([]contracts.OptionOpen, error) {
	best := map[int]contracts.OptionOpen{}
	for _, r := range f.open {
		if r.SessionDay.Format(time.DateOnly) != string(d) {
			continue
		}
		if o, ok := best[r.InstrumentID]; !ok || r.ReceivedAt.After(o.ReceivedAt) {
			best[r.InstrumentID] = r
		}
	}
	out := []contracts.OptionOpen{}
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstrumentID < out[j].InstrumentID })
	return out, nil
}

func (f *fake) PutPlan(_ context.Context, items []contracts.OptionOpenItem) error {
	f.plan = append(f.plan, items...)
	return nil
}

func (f *fake) Plan(_ context.Context, d contracts.Day, slot string, ids ...int64) ([]contracts.OptionOpenItem, error) {
	latest := map[int]time.Time{}
	for _, it := range f.plan {
		if it.PlannedAt.After(latest[it.UnderlyingID]) {
			latest[it.UnderlyingID] = it.PlannedAt
		}
	}
	out := []contracts.OptionOpenItem{}
	for _, it := range f.plan {
		want := len(ids) == 0
		for _, id := range ids {
			want = want || int64(it.UnderlyingID) == id
		}
		if want && it.SessionDay.Format(time.DateOnly) == string(d) && it.Slot == slot && it.PlannedAt.Equal(latest[it.UnderlyingID]) {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UnderlyingID != out[j].UnderlyingID {
			return out[i].UnderlyingID < out[j].UnderlyingID
		}
		return out[i].Rank < out[j].Rank
	})
	return out, nil
}

func (f *fake) AddNotListed(_ context.Context, ms []contracts.OptionNotListed) error {
	if f.marks == nil {
		f.marks = map[openquote.NotListedKey]contracts.OptionNotListed{}
	}
	for _, m := range ms {
		f.marks[keyOf(m)] = m
	}
	return nil
}

func (f *fake) NotListed(_ context.Context, now time.Time) ([]contracts.OptionNotListed, error) {
	out := []contracts.OptionNotListed{}
	for _, m := range f.marks {
		if m.RetryAt.After(now) {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Strike < out[j].Strike })
	return out, nil
}

func (f *fake) RemoveNotListed(_ context.Context, k openquote.NotListedKey) (bool, error) {
	_, ok := f.marks[k]
	delete(f.marks, k)
	return ok, nil
}

func (f *fake) RemoveNotListedByUnderlying(_ context.Context, id int64) (int, error) {
	n := 0
	for k := range f.marks {
		if k.UnderlyingID == id {
			delete(f.marks, k)
			n++
		}
	}
	return n, nil
}

func (f *fake) RemoveNotListedExpired(_ context.Context, d contracts.Day) (int, error) {
	n := 0
	for k := range f.marks {
		if k.Expiry < d {
			delete(f.marks, k)
			n++
		}
	}
	return n, nil
}

func (f *fake) PutOption(_ context.Context, r openquote.OptionRef) error {
	if f.refs == nil {
		f.refs = map[[7]string]openquote.OptionRef{}
	}
	k := [7]string{strconv.FormatInt(r.UnderlyingID, 10), r.Currency, r.Exchange, r.TradingClass, r.Multiplier + string(r.Expiry), r.Strike, r.Right}
	f.refs[k] = r
	return nil
}

func (f *fake) ListOptions(_ context.Context, id int64) ([]openquote.OptionRef, error) {
	out := []openquote.OptionRef{}
	for _, r := range f.refs {
		if r.UnderlyingID == id {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstrumentID < out[j].InstrumentID })
	return out, nil
}

func TestDSNLineAndRefreshMarker(t *testing.T) {
	t.Parallel()
	if openquote.DSNEnvOptionOpen != "OT_PG_DSN_OPTION_OPEN" || openquote.SlotRefresh != "refresh" {
		t.Fatalf("%q %q", openquote.DSNEnvOptionOpen, openquote.SlotRefresh)
	}
}

func TestOptionOpenWriterRefusesRefreshAndReaderGivesNewestPerInstrument(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s openquote.Stores = &fake{}
	t0 := time.Date(2026, 11, 11, 14, 45, 0, 0, time.UTC)
	mk := func(slot string, inst int, at time.Time) contracts.OptionOpen {
		return contracts.OptionOpen{SessionDay: day("2026-11-11"), CaptureID: "c-" + slot, Slot: slot, InstrumentID: inst, ReceivedAt: at}
	}
	if err := s.InsertOptionOpen(ctx, []contracts.OptionOpen{mk(openquote.SlotRefresh, 1, t0)}); err == nil {
		t.Fatal("a refresh row must be refused")
	}
	if err := s.InsertOptionOpen(ctx, []contracts.OptionOpen{mk("0945", 1, t0), mk("0945", 2, t0), mk("1000", 1, t0.Add(time.Minute))}); err != nil {
		t.Fatal(err)
	}
	got, err := s.OptionOpen(ctx, "2026-11-11")
	if err != nil || len(got) != 2 || got[0].Slot != "1000" || got[1].InstrumentID != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if none, err := s.OptionOpen(ctx, "2026-11-12"); err != nil || none == nil || len(none) != 0 {
		t.Fatalf("empty day = %#v, %v", none, err)
	}
}

func TestPlanLatestPerUnderlying(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s openquote.OptionOpenPlanStore = &fake{}
	t0 := time.Date(2026, 11, 11, 13, 45, 0, 0, time.UTC)
	it := func(plan string, und, inst, rank int, at time.Time) contracts.OptionOpenItem {
		return contracts.OptionOpenItem{SessionDay: day("2026-11-11"), Slot: "0945", PlanID: plan, UnderlyingID: und, InstrumentID: inst, Rank: rank, PlannedAt: at}
	}
	if err := s.PutPlan(ctx, []contracts.OptionOpenItem{
		it("old", 7, 70, 0, t0), it("new", 7, 71, 1, t0.Add(time.Minute)),
		it("new", 7, 72, 0, t0.Add(time.Minute)), it("x", 8, 80, 0, t0),
	}); err != nil {
		t.Fatal(err)
	}
	all, _ := s.Plan(ctx, "2026-11-11", "0945")
	if len(all) != 3 || all[0].InstrumentID != 72 || all[1].InstrumentID != 71 || all[2].UnderlyingID != 8 {
		t.Fatalf("all = %+v", all)
	}
	one, _ := s.Plan(ctx, "2026-11-11", "0945", 8)
	if len(one) != 1 || one[0].PlanID != "x" {
		t.Fatalf("one = %+v", one)
	}
}

func TestNotListedMarks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s openquote.OptionNotListedStore = &fake{}
	now := time.Date(2026, 11, 11, 1, 0, 0, 0, time.UTC)
	m := func(und int, exp, strike string, retry time.Time) contracts.OptionNotListed {
		return contracts.OptionNotListed{UnderlyingID: und, Expiry: day(exp), Strike: strike, Right: "P", TradingClass: "AAPL", RetryAt: retry}
	}
	if err := s.AddNotListed(ctx, []contracts.OptionNotListed{
		m(1, "2026-11-20", "100", now.Add(time.Hour)), m(1, "2026-12-18", "101", now.Add(-time.Hour)),
		m(2, "2026-12-18", "102", now.Add(time.Hour)),
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.NotListed(ctx, now); len(got) != 2 {
		t.Fatalf("not yet due = %d, want 2", len(got))
	}
	if n, _ := s.RemoveNotListedExpired(ctx, "2026-12-01"); n != 1 {
		t.Fatalf("expired = %d", n)
	}
	if n, _ := s.RemoveNotListedByUnderlying(ctx, 2); n != 1 {
		t.Fatalf("by underlying = %d", n)
	}
	if ok, _ := s.RemoveNotListed(ctx, openquote.NotListedKey{UnderlyingID: 1, Expiry: "2026-12-18", Strike: "101", Right: "P", TradingClass: "AAPL"}); !ok {
		t.Fatal("key not removed")
	}
	if ok, _ := s.RemoveNotListed(ctx, openquote.NotListedKey{UnderlyingID: 1}); ok {
		t.Fatal("missing key reported removed")
	}
}

func TestOptionContractPutAndList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s openquote.OptionContractStore = &fake{}
	r := openquote.OptionRef{UnderlyingID: 5, Currency: "USD", Exchange: "SMART", TradingClass: "AAPL",
		Multiplier: "100", Expiry: "2026-12-18", Strike: "180", Right: "P", InstrumentID: 11}
	if err := s.PutOption(ctx, r); err != nil {
		t.Fatal(err)
	}
	r.InstrumentID = 12
	if err := s.PutOption(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ListOptions(ctx, 5)
	if len(got) != 1 || got[0].InstrumentID != 12 {
		t.Fatalf("got %+v", got)
	}
	if none, _ := s.ListOptions(ctx, 6); none == nil || len(none) != 0 {
		t.Fatalf("none = %#v", none)
	}
}

// listFake keeps the promises of OptionListStore in memory: the caller side of the contract.
type listFake struct {
	lists []openquote.ListRecord
	runs  []openquote.ListRunRecord
}

var _ openquote.OptionListStore = (*listFake)(nil)

func (f *listFake) PutLists(_ context.Context, rows []openquote.ListRecord) error {
	for _, r := range rows {
		replaced := false
		for i, o := range f.lists {
			if o.UnderlyingID == r.UnderlyingID && o.Expiry == r.Expiry && o.Class == r.Class {
				f.lists[i], replaced = r, true
			}
		}
		if !replaced {
			f.lists = append(f.lists, r)
		}
	}
	return nil
}

func (f *listFake) Lists(_ context.Context, id int64) ([]openquote.ListRecord, error) {
	out := []openquote.ListRecord{}
	for _, r := range f.lists {
		if r.UnderlyingID == id {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *listFake) PutListRuns(_ context.Context, rows []openquote.ListRunRecord) error {
	for _, r := range rows {
		replaced := false
		for i, o := range f.runs {
			if o.UnderlyingID == r.UnderlyingID && o.Session == r.Session {
				f.runs[i], replaced = r, true
			}
		}
		if !replaced {
			f.runs = append(f.runs, r)
		}
	}
	return nil
}

func (f *listFake) ListRuns(_ context.Context, session contracts.Day) ([]openquote.ListRunRecord, error) {
	out := []openquote.ListRunRecord{}
	for _, r := range f.runs {
		if r.Session == session {
			out = append(out, r)
		}
	}
	return out, nil
}

func TestOptionListPutAndReadBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s openquote.OptionListStore = &listFake{}
	at := time.Date(2026, 11, 12, 11, 20, 0, 0, time.UTC)
	if err := s.PutLists(ctx, []openquote.ListRecord{{UnderlyingID: 5, Expiry: "2026-12-18", Class: "AAPL",
		ListedAt: at, ListedSession: "2026-11-12", Count: 516, TriedAt: at, Outcome: "listed"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Lists(ctx, 5)
	if len(got) != 1 || got[0].Count != 516 || got[0].ListedSession != "2026-11-12" {
		t.Fatalf("got %+v", got)
	}
	if none, _ := s.Lists(ctx, 6); none == nil || len(none) != 0 {
		t.Fatalf("none = %#v", none)
	}
	if err := s.PutListRuns(ctx, []openquote.ListRunRecord{
		{UnderlyingID: 5, Session: "2026-11-12", Expected: []string{"2026-12-18|AAPL", "2026-12-24|AAPL"}, Listed: 2, Reason: "none", UpdatedAt: at},
		{UnderlyingID: 6, Session: "2026-11-12", Reason: "not_finished", UpdatedAt: at},
	}); err != nil {
		t.Fatal(err)
	}
	runs, _ := s.ListRuns(ctx, "2026-11-12")
	if len(runs) != 2 || len(runs[0].Expected) != 2 || runs[1].Expected != nil {
		t.Fatalf("runs %+v", runs)
	}
	if none, _ := s.ListRuns(ctx, "2026-11-13"); none == nil || len(none) != 0 {
		t.Fatalf("none = %#v", none)
	}
}

func TestRunRowExpectedNamesTheGroups(t *testing.T) {
	t.Parallel()
	var s openquote.OptionListStore = &listFake{}
	ctx := context.Background()
	_ = s.PutListRuns(ctx, []openquote.ListRunRecord{{UnderlyingID: 5, Session: "2026-11-12",
		Expected: []string{"2026-12-18|AAPL", "2026-12-24|AAPL"}, Reason: "not_finished"}})
	runs, _ := s.ListRuns(ctx, "2026-11-12")
	if len(runs) != 1 || len(runs[0].Expected) != 2 || runs[0].Expected[1] != "2026-12-24|AAPL" {
		t.Fatalf("runs %+v", runs)
	}
}

func TestPutListRunsUpdatesExistingKey(t *testing.T) {
	t.Parallel()
	var s openquote.OptionListStore = &listFake{}
	ctx := context.Background()
	_ = s.PutListRuns(ctx, []openquote.ListRunRecord{{UnderlyingID: 5, Session: "2026-11-12", Reason: "not_finished"}})
	_ = s.PutListRuns(ctx, []openquote.ListRunRecord{{UnderlyingID: 5, Session: "2026-11-12", Reason: "none"}})
	runs, _ := s.ListRuns(ctx, "2026-11-12")
	if len(runs) != 1 || runs[0].Reason != "none" {
		t.Fatalf("runs %+v", runs)
	}
}
