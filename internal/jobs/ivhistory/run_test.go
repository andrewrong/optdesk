package ivhistory

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func fixed(bars ...broker.Bar) *fakeHistory {
	return &fakeHistory{answer: func(broker.BarQuery) ([]broker.Bar, error) { return bars, nil }}
}

func storedOk(name, d, v string) contracts.ReadingRecord {
	return contracts.ReadingRecord{Name: "market.iv_ibkr", SubjectKind: "underlying", Subject: name, Day: day(d), State: "ok", Value: &v}
}

func byDay(st *memReadings) map[string]contracts.ReadingRecord {
	m := map[string]contracts.ReadingRecord{}
	for _, r := range st.rows {
		m[r.Day.Format(dayLayout)] = r
	}
	return m
}

func TestBootstrapAsks400Days(t *testing.T) {
	t.Parallel()
	h := fixed(bar("2026-11-09", "0.28"))
	// a reading of 366 days ago is outside the last 365 days
	st := &memReadings{rows: []contracts.ReadingRecord{storedOk("AAA", "2025-11-08", "0.20")}}
	j := build(t, h, st, "AAA")
	if res := j.Run(context.Background(), runDay, keysOf("AAA")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].DurationDays != 400 || h.queries[0].Kind != broker.BarOptionImpliedVol ||
		h.queries[0].Contract.Symbol != "AAA" {
		t.Fatalf("queries %+v", h.queries)
	}
}

func TestLaterRunAsks5Days(t *testing.T) {
	t.Parallel()
	h := fixed(bar("2026-11-09", "0.28"))
	st := &memReadings{rows: []contracts.ReadingRecord{storedOk("AAA", "2026-11-06", "0.20")}}
	j := build(t, h, st, "AAA")
	if res := j.Run(context.Background(), runDay, keysOf("AAA")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].DurationDays != 5 {
		t.Fatalf("queries %+v", h.queries)
	}
}

func TestStoresOneReadingPerBar(t *testing.T) {
	t.Parallel()
	h := fixed(bar("2026-11-05", "0.31"), bar("2026-11-06", "0.3"), bar("2026-11-09", "0.28"))
	st := &memReadings{}
	j := build(t, h, st, "AAA")
	res := j.Run(context.Background(), runDay, keysOf("AAA"))
	if res.Err != nil || len(st.rows) != 3 {
		t.Fatalf("result %+v rows %d", res, len(st.rows))
	}
	for _, s := range st.rows {
		if s.Rev != 1 {
			t.Fatalf("rev %d of %+v", s.Rev, s)
		}
	}
	r := byDay(st)["2026-11-09"]
	if r.Name != "market.iv_ibkr" || r.Version != 1 || r.SubjectKind != "underlying" || r.Subject != "AAA" ||
		string(r.Origin) != "ibkr" || string(r.Unit) != "decimal" || string(r.State) != "ok" || r.Value == nil || *r.Value != "0.28" ||
		len(r.Inputs) != 1 || r.Inputs[0].InputTable != "ibkr.bars" || r.Inputs[0].Keys[0] != "AAA|2026-11-09" ||
		!r.RecordedAt.Equal(fakeClock{}.Now()) {
		t.Fatalf("reading %+v", r)
	}
}

func TestMissingBarOfDayIsNoInput(t *testing.T) {
	t.Parallel()
	for name, bars := range map[string][]broker.Bar{
		"empty answer":   nil,
		"older bar only": {bar("2026-11-06", "0.30")},
	} {
		st := &memReadings{}
		j := build(t, fixed(bars...), st, "AAA")
		if res := j.Run(context.Background(), runDay, keysOf("AAA")); res.Err != nil {
			t.Fatal(name, res.Err)
		}
		r, ok := byDay(st)["2026-11-09"]
		if !ok || string(r.State) != "missing_input" || r.Value != nil || r.ReasonCode == nil || *r.ReasonCode != 1 {
			t.Fatalf("%s: reading %+v", name, r)
		}
	}
}

func TestBadCloseIsBadInput(t *testing.T) {
	t.Parallel()
	for _, close := range []string{"0", "11", "x"} {
		st := &memReadings{}
		j := build(t, fixed(bar("2026-11-09", close)), st, "AAA")
		if res := j.Run(context.Background(), runDay, keysOf("AAA")); res.Err != nil || res.Usable != 0 {
			t.Fatalf("%s: %+v", close, res)
		}
		r := byDay(st)["2026-11-09"]
		if string(r.State) != "missing_input" || r.Value != nil || r.ReasonCode == nil || *r.ReasonCode != 5 {
			t.Fatalf("%s: reading %+v", close, r)
		}
	}
}

func TestRerunInsertsNothingNew(t *testing.T) {
	t.Parallel()
	h := fixed(bar("2026-11-06", "0.30"), bar("2026-11-09", "0.28"))
	st := &memReadings{}
	j := build(t, h, st, "AAA")
	j.Run(context.Background(), runDay, keysOf("AAA"))
	first := len(st.rows)
	j.Run(context.Background(), runDay, keysOf("AAA"))
	if first != 2 || len(st.rows) != first {
		t.Fatalf("rows %d then %d", first, len(st.rows))
	}
	if h.queries[1].DurationDays != 5 {
		t.Fatalf("second run asked %d days", h.queries[1].DurationDays)
	}
}

func TestLateBarReplacesMissingMarker(t *testing.T) {
	t.Parallel()
	answer := []broker.Bar{bar("2026-11-06", "0.30")}
	h := &fakeHistory{answer: func(broker.BarQuery) ([]broker.Bar, error) { return answer, nil }}
	st := &memReadings{}
	j := build(t, h, st, "AAA")
	j.Run(context.Background(), runDay, keysOf("AAA"))
	answer = []broker.Bar{bar("2026-11-06", "0.30"), bar("2026-11-09", "0.28")}
	res := j.Run(context.Background(), runDay, keysOf("AAA"))
	r := byDay(st)["2026-11-09"]
	if res.Usable != 1 || r.Rev != 2 || string(r.State) != "ok" {
		t.Fatalf("result %+v reading %+v", res, r)
	}
	marker := false
	for _, s := range st.rows {
		marker = marker || (s.Day.Format(dayLayout) == "2026-11-09" && s.Rev == 1 && string(s.State) == "missing_input")
	}
	if !marker {
		t.Fatalf("missing marker of rev 1 not stored: %+v", st.rows)
	}
}

func TestFirstRunHasNoRevZero(t *testing.T) {
	t.Parallel()
	st := &memReadings{}
	j := build(t, fixed(bar("2026-11-09", "0.28")), st, "AAA")
	res := j.Run(context.Background(), runDay, keysOf("AAA"))
	if res.Err != nil || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
}

func TestOneKeyFailureDoesNotStopOthers(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.Contract.Symbol == "AAA" {
			return nil, retryable{}
		}
		return []broker.Bar{bar("2026-11-09", "0.28")}, nil
	}}
	st := &memReadings{}
	j := build(t, h, st, "AAA", "BBB")
	res := j.Run(context.Background(), runDay, keysOf("AAA", "BBB"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || res.Usable != 1 || len(st.rows) != 1 {
		t.Fatalf("result %+v rows %d", res, len(st.rows))
	}
}

func TestUnknownKeyIsPermanent(t *testing.T) {
	t.Parallel()
	j := build(t, fixed(), &memReadings{}, "AAA")
	res := j.Run(context.Background(), runDay, keysOf("ZZZ"))
	if res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("result %+v", res)
	}
}

const noData162 = "broker: bars: Unknown: broker error 162: Historical Market Data Service error message:" +
	"HMDS query returned no data: X.T@SMART Option_Implied_Volatility"

func noDataFor(sym string, errText string) *fakeHistory {
	return &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.Contract.Symbol == sym {
			return nil, newErr(contracts.Permanent, "%s", errText)
		}
		return []broker.Bar{bar("2026-11-09", "0.28")}, nil
	}}
}

func TestNoDataAnswerIsNoInput(t *testing.T) {
	t.Parallel()
	st := &memReadings{}
	j := build(t, noDataFor("XXX", noData162), st, "XXX", "BBB")
	res := j.Run(context.Background(), runDay, keysOf("XXX", "BBB"))
	if res.Err != nil || res.Returned != 2 || res.Usable != 1 || len(st.rows) != 2 {
		t.Fatalf("result %+v rows %d", res, len(st.rows))
	}
	for _, r := range st.rows {
		switch r.Subject {
		case "XXX":
			if string(r.State) != "missing_input" || r.ReasonCode == nil || *r.ReasonCode != 1 || r.Day.Format(dayLayout) != "2026-11-09" {
				t.Fatalf("XXX reading %+v", r)
			}
		case "BBB":
			if string(r.State) != "ok" {
				t.Fatalf("BBB reading %+v", r)
			}
		}
	}
}

func TestNoDataRerunStoresNothingNew(t *testing.T) {
	t.Parallel()
	st := &memReadings{}
	j := build(t, noDataFor("XXX", noData162), st, "XXX")
	j.Run(context.Background(), runDay, keysOf("XXX"))
	first := len(st.rows)
	res := j.Run(context.Background(), runDay, keysOf("XXX"))
	if first != 1 || len(st.rows) != first || res.Err != nil {
		t.Fatalf("rows %d then %d, result %+v", first, len(st.rows), res)
	}
}

func TestOtherBrokerErrorStaysFailed(t *testing.T) {
	t.Parallel()
	st := &memReadings{}
	j := build(t, noDataFor("XXX", "broker error 200"), st, "XXX")
	res := j.Run(context.Background(), runDay, keysOf("XXX"))
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || len(st.rows) != 0 {
		t.Fatalf("result %+v rows %d", res, len(st.rows))
	}
}
