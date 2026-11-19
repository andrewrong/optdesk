package optioneod

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

type fakeContractReader struct {
	refs  []openquote.OptionRef
	err   error
	lists int
	mu    sync.Mutex
}

func (f *fakeContractReader) ListOptions(context.Context, int64) ([]openquote.OptionRef, error) {
	f.mu.Lock()
	f.lists++
	f.mu.Unlock()
	return f.refs, f.err
}

type fakeInstruments struct {
	conIDs map[int64]int64 // instrument id -> con_id
	err    error
}

func (f fakeInstruments) Instruments(_ context.Context, ids []int64) ([]store.Instrument, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []store.Instrument
	for _, id := range ids {
		if c, ok := f.conIDs[id]; ok {
			out = append(out, store.Instrument{ID: id, Identifiers: []store.Identifier{
				{Scheme: "osi", Value: "x"}, {Scheme: "ibkr_conid", Value: strconv.FormatInt(c, 10)}}})
		}
	}
	return out, nil
}

// storedRig runs a key with the first n of the pairs of an untouched run stored; the stored con_id is the one the fake
// broker would give, so rows equal those of a run without stored contracts.
func storedRig(t *testing.T, stored int, strikesPerSide int) (*rig, *fakeContractReader, []broker.Contract) {
	t.Helper()
	base := newRig(t, "2026-11-06 17:00")
	base.job.m.MaxStrikesPerSide = strikesPerSide
	if res := base.run(testDay, testKey); res.Err != nil {
		t.Fatalf("base run: %+v", res.Err)
	}
	var pairs []broker.Contract
	for _, s := range base.b.snaps {
		if s.kind == broker.KindOptionEod {
			pairs = append(pairs, s.cs...)
		}
	}
	r := newRig(t, "2026-11-06 17:00")
	r.job.m.MaxStrikesPerSide = strikesPerSide
	rd := &fakeContractReader{}
	in := fakeInstruments{conIDs: map[int64]int64{}}
	for i, p := range pairs[:stored] {
		id := int64(7000 + i)
		in.conIDs[id] = conID(p)
		rd.refs = append(rd.refs, openquote.OptionRef{UnderlyingID: 1, TradingClass: p.TradingClass, Multiplier: p.Multiplier,
			Expiry: contracts.Day(p.Expiry[:4] + "-" + p.Expiry[4:6] + "-" + p.Expiry[6:]), Strike: p.Strike + ".0", Right: p.Right, InstrumentID: id})
	}
	if _, err := WithContracts(r.job, rd, in); err != nil {
		t.Fatal(err)
	}
	return r, rd, pairs
}

func TestStoredContractsNotResolved(t *testing.T) {
	t.Parallel()
	base := newRig(t, "2026-11-06 17:00")
	base.run(testDay, testKey)
	r, rd, pairs := storedRig(t, 4, 1)
	if len(pairs) != 4 {
		t.Fatalf("pairs %d", len(pairs))
	}
	res := r.run(testDay, testKey)
	if res.Err != nil || r.b.resolves != 0 || rd.lists != 1 {
		t.Fatalf("res %+v resolves %d lists %d", res, r.b.resolves, rd.lists)
	}
	if !reflect.DeepEqual(r.w.all(), base.w.all()) {
		t.Fatalf("rows differ:\n%+v\n%+v", r.w.all(), base.w.all())
	}
}

// listFive calls listed with the first 5 of the 6 pairs of the chain, 3 of them stored.
func listFive(t *testing.T) (*rig, []broker.Contract, []broker.Contract) {
	t.Helper()
	r, _, pairs := storedRig(t, 3, 3)
	if len(pairs) < 5 {
		t.Fatalf("pairs %d", len(pairs))
	}
	pairs = pairs[:5]
	rs := &runState{j: r.job, day: testDay, start: r.clk.Now()}
	u, _ := r.job.underlyingOf("TEST")
	got, terr := rs.listed(context.Background(), u, 1, pairs)
	if terr != nil {
		t.Fatal(terr)
	}
	return r, pairs, got
}

func TestStoredContractsGapResolved(t *testing.T) {
	t.Parallel()
	r, pairs, got := listFive(t)
	if r.b.resolves != 2 {
		t.Fatalf("resolves %d", r.b.resolves)
	}
	if len(got) != 5 {
		t.Fatalf("got %d", len(got))
	}
	for i, c := range got {
		if c.ConID != conID(pairs[i]) || c.Strike != pairs[i].Strike || c.Right != pairs[i].Right || c.Multiplier != "100" {
			t.Fatalf("contract %d = %+v, pair %+v", i, c, pairs[i])
		}
	}
}

func TestStoredContractsReadFails(t *testing.T) {
	t.Parallel()
	base := newRig(t, "2026-11-06 17:00")
	base.run(testDay, testKey)
	var buf bytes.Buffer
	r, rd, _ := storedRig(t, 4, 1)
	rd.err = errors.New("db gone")
	r.job.logger = slog.New(slog.NewTextHandler(&buf, nil))
	res := r.run(testDay, testKey)
	if res.Err != nil || !reflect.DeepEqual(r.w.all(), base.w.all()) || r.b.resolves != 4 {
		t.Fatalf("res %+v resolves %d", res, r.b.resolves)
	}
	if !strings.Contains(buf.String(), "level=WARN") || !strings.Contains(buf.String(), "db gone") ||
		!strings.Contains(buf.String(), "symbol=TEST") || !strings.Contains(buf.String(), "job=option_eod") {
		t.Fatalf("log %q", buf.String())
	}
}

func TestStoredContractsCountsLine(t *testing.T) {
	t.Parallel()
	r, _, pairs := storedRig(t, 3, 3)
	pairs = pairs[:5]
	r.b.notListed[pairs[4].Strike+" "+pairs[4].Right] = true
	var buf bytes.Buffer
	r.job.logger = slog.New(slog.NewTextHandler(&buf, nil))
	rs := &runState{j: r.job, day: testDay, start: r.clk.Now()}
	u, _ := r.job.underlyingOf("TEST")
	got, terr := rs.listed(context.Background(), u, 1, pairs)
	if terr != nil || len(got) != 4 {
		t.Fatalf("got %d %v", len(got), terr)
	}
	out := buf.String()
	if strings.Count(out, "optioneod: contracts") != 1 || !strings.Contains(out, "level=INFO") ||
		!strings.Contains(out, "stored=3") || !strings.Contains(out, "resolved=2") || !strings.Contains(out, "not_listed=1") ||
		!strings.Contains(out, "seconds=") {
		t.Fatalf("log %q", out)
	}
}
