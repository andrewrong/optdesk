package optioneod

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

type replaceCall struct {
	day              contracts.Day
	dataset, subject string
	rows             []runview.Reason
}

type recordingReasons struct {
	calls []replaceCall
	err   error
}

func (w *recordingReasons) ReplaceReasons(_ context.Context, day contracts.Day, dataset, subject string, rows []runview.Reason) error {
	w.calls = append(w.calls, replaceCall{day, dataset, subject, rows})
	return w.err
}

// fullQuote has every counted field set.
func fullQuote() broker.Quote {
	return broker.Quote{Status: broker.StatusOK, Bid: num(1), Ask: num(2), Last: num(1.5), Volume: num(3), OpenInterest: num(4),
		ImpliedVol: num(0.2), Delta: num(0.5), Gamma: num(0.1), Theta: num(-0.1), Vega: num(0.2)}
}

func details(rows []runview.Reason) map[string]string {
	out := map[string]string{}
	for _, r := range rows {
		out[r.Scope] = r.Code + ": " + r.Detail
	}
	return out
}

func TestTallyUsesReportedReason(t *testing.T) {
	t.Parallel()
	cases := map[string]broker.MissingField{
		"not_received_release": {Field: "open_interest", Reason: broker.MissingNotReceivedRelease},
		"not_received_grace":   {Field: "open_interest", Reason: broker.MissingNotReceivedGrace},
		"invalid":              {Field: "open_interest", Reason: broker.MissingInvalid},
		"received_zero":        {Field: "open_interest", Reason: broker.MissingReceivedZero},
		"market_data_type":     {Field: "open_interest", Reason: broker.MissingMarketDataType},
		"error_200":            {Field: "open_interest", Reason: broker.MissingError, ErrorCode: 200},
	}
	for want, m := range cases {
		q := fullQuote()
		q.OpenInterest = nil
		q.Missing = []broker.MissingField{m}
		q.Status = broker.StatusPartial // the status never picks the cause
		var tl tally
		tl.add(q, statusOK)
		if got := tl.fields["open_interest"]; !reflect.DeepEqual(got, map[string]int{want: 1}) {
			t.Fatalf("%s: counted %v", want, got)
		}
	}
}

func TestTallyUnreportedWithoutEntry(t *testing.T) {
	t.Parallel()
	q := fullQuote()
	q.Delta, q.ImpliedVol = nil, nil
	q.Missing = []broker.MissingField{{Field: "bid", Reason: broker.MissingInvalid}} // bid is set: not counted
	var tl tally
	tl.add(q, statusOK)
	want := map[string]map[string]int{"delta": {"unreported": 1}, "iv": {"unreported": 1}}
	if !reflect.DeepEqual(tl.fields, want) {
		t.Fatalf("fields %v", tl.fields)
	}
}

func TestTallyQuotesScope(t *testing.T) {
	t.Parallel()
	var tl tally
	for _, q := range []broker.Quote{
		{Status: broker.StatusNoData},
		{Status: broker.StatusNoSubscription},
		{Status: broker.StatusTimeout},
		{Status: broker.StatusDisconnected},
		{Status: broker.StatusError, ErrorCode: 354},
		{Status: broker.StatusError},
		{Status: broker.StatusOK}, // row status 2: no bid, ask, last, iv
	} {
		q.Missing = []broker.MissingField{{Field: "bid", Reason: broker.MissingInvalid}}
		st, _ := statusOf(q)
		tl.add(q, st)
	}
	want := map[string]int{"no_data": 1, "no_subscription": 1, "timeout": 1, "disconnected": 1, "error_354": 1, "error": 1, "empty": 1}
	if !reflect.DeepEqual(tl.noQuote, want) || len(tl.fields) != 0 || tl.usable != 0 {
		t.Fatalf("noQuote %v fields %v usable %d", tl.noQuote, tl.fields, tl.usable)
	}
}

func TestReasonsDetailText(t *testing.T) {
	t.Parallel()
	var tl tally
	for i := 0; i < 200; i++ {
		q := fullQuote()
		q.OpenInterest = nil
		q.Missing = []broker.MissingField{{Field: "open_interest", Reason: broker.MissingNotReceivedGrace}}
		if i < 5 {
			q.ImpliedVol = nil
			q.Missing = append(q.Missing, broker.MissingField{Field: "implied_vol", Reason: broker.MissingInvalid})
		}
		if i == 5 {
			q.ImpliedVol = nil
		}
		tl.add(q, statusOK)
	}
	for i := 0; i < 10; i++ {
		tl.add(broker.Quote{Status: broker.StatusNoData}, statusNoQuote)
	}
	tl.add(broker.Quote{Status: broker.StatusTimeout}, statusError)
	tl.add(broker.Quote{Status: broker.StatusTimeout}, statusError)
	at := time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)
	rows := tl.reasons(testDay, "obs.option_eod", "TEST", at)
	want := map[string]string{
		"field:open_interest": "field_missing: open_interest missing 200/200 usable quotes: not_received_grace 200",
		"field:iv":            "field_missing: iv missing 6/200 usable quotes: invalid 5, unreported 1",
		"quotes":              "field_missing: no quote 12/212 quotes: no_data 10, timeout 2",
	}
	if got := details(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("details %v", got)
	}
	for _, r := range rows {
		if r.Day != testDay || r.Dataset != "obs.option_eod" || r.Subject != "TEST" || !r.At.Equal(at) {
			t.Fatalf("row %+v", r)
		}
	}
}

func TestFieldsCompleteRow(t *testing.T) {
	t.Parallel()
	at := time.Unix(0, 0).UTC()
	var tl tally
	for i := 0; i < 200; i++ {
		tl.add(fullQuote(), statusOK)
	}
	rows := tl.reasons(testDay, "d", "s", at)
	if len(rows) != 1 || rows[0].Code != "fields_complete" || rows[0].Scope != "fields" || rows[0].Detail != "no missing field in 200 usable quotes" {
		t.Fatalf("rows %+v", rows)
	}
	var none tally
	if rows := none.reasons(testDay, "d", "s", at); len(rows) != 0 {
		t.Fatalf("empty tally gave %+v", rows)
	}
	var quotesOnly tally
	quotesOnly.add(broker.Quote{Status: broker.StatusNoData}, statusNoQuote)
	rows = quotesOnly.reasons(testDay, "d", "s", at)
	if len(rows) != 1 || rows[0].Scope != "quotes" {
		t.Fatalf("no usable quote must give no fields_complete: %+v", rows)
	}
}

func graceRig(t *testing.T) (*rig, *recordingReasons) {
	t.Helper()
	r := newRig(t, "2026-11-06 17:00")
	// 100 strikes (50 per side of the price 100) of both rights: 200 contracts, 7 chunks of 30
	r.job.m.Chunk, r.job.m.MaxStrikesPerSide, r.job.m.StrikeBand = 30, 50, 1
	strikes := make([]string, 200)
	for i := range strikes {
		strikes[i] = strconv.Itoa(i + 1)
	}
	r.b.chains[0].Strikes = strikes
	r.b.full, r.b.noOI = true, true
	r.b.missing = []broker.MissingField{{Field: "open_interest", Reason: broker.MissingNotReceivedGrace}}
	rw := &recordingReasons{}
	r.job.reasons = rw
	return r, rw
}

func TestRunWritesOncePerKey(t *testing.T) {
	t.Parallel()
	r, rw := graceRig(t)
	res := r.run(testDay, testKey)
	if res.Err != nil || res.Returned != 200 || r.b.optionCalls != 7 {
		t.Fatalf("result %+v, chunks %d", res, r.b.optionCalls)
	}
	if len(rw.calls) != 1 {
		t.Fatalf("want one call, got %d", len(rw.calls))
	}
	c := rw.calls[0]
	if c.day != "2026-11-06" || c.dataset != "obs.option_eod" || c.subject != "TEST" || len(c.rows) != 1 {
		t.Fatalf("call %+v", c)
	}
	if got := c.rows[0]; got.Scope != "field:open_interest" || got.Detail != "open_interest missing 200/200 usable quotes: not_received_grace 200" ||
		!got.At.Equal(r.clk.Now().UTC()) || got.At.Location() != time.UTC {
		t.Fatalf("row %+v", got)
	}
}

func TestRunStoredNothingKeepsEarlierReasons(t *testing.T) {
	t.Parallel()
	r, rw := graceRig(t)
	r.b.onChains = func() { r.clk.Advance(9 * time.Minute) } // the budget ends before the first insert
	res := r.run(testDay, testKey)
	if kindOf(res) != contracts.Retryable || res.Returned != 0 {
		t.Fatalf("result %+v", res)
	}
	if len(rw.calls) != 0 {
		t.Fatalf("nothing stored must write nothing, got %d calls", len(rw.calls))
	}
}

func TestRunLaterChunkFailureStillWrites(t *testing.T) {
	t.Parallel()
	r, rw := graceRig(t)
	r.b.afterSnap = func(n int) {
		if n == 2 {
			r.b.snapErr = errors.New("gateway down") // the third chunk fails
		}
	}
	res := r.run(testDay, testKey)
	if kindOf(res) != contracts.Permanent || res.Returned != 60 {
		t.Fatalf("result %+v", res)
	}
	if len(rw.calls) != 1 || len(rw.calls[0].rows) != 1 ||
		rw.calls[0].rows[0].Detail != "open_interest missing 60/60 usable quotes: not_received_grace 60" {
		t.Fatalf("calls %+v", rw.calls)
	}
}

func TestRunWriterErrorDoesNotFailRun(t *testing.T) {
	t.Parallel()
	r, rw := graceRig(t)
	rw.err = errors.New("no such table")
	res := r.run(testDay, testKey)
	if res.Err != nil || res.Returned != 200 || res.Usable != 200 || len(r.w.all()) != 200 || len(rw.calls) != 1 {
		t.Fatalf("result %+v, calls %d", res, len(rw.calls))
	}
	plain := newRig(t, "2026-11-06 17:00")
	if res := plain.run(testDay, testKey); res.Err != nil || res.Returned != 4 {
		t.Fatalf("without writer %+v", res)
	}
}
