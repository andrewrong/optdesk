package barsrefresh

import (
	"context"
	"strconv"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

// capJob builds a job over three names with a first-pull cap of two.
func capJob(t *testing.T) (*job, *fakeHistory, *memBars) {
	t.Helper()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "10.0000"))}
	st := &memBars{}
	var entries []jobs.WatchEntry
	for i := 1; i <= 3; i++ {
		entries = append(entries, entry("cik:"+strconv.Itoa(i), "N"+strconv.Itoa(i), int64(100+i)))
	}
	jb, err := newJob(&fakeBroker{listed: true}, h, fakeResolver{}, st, st, entries, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	j := jb.(*job)
	j.m.BackfillNamesPerRun = 2
	j.m.AdjustedCadenceDays = 0
	return j, h, st
}

func TestCapDeniedKeyRetryable(t *testing.T) {
	t.Parallel()
	j, h, st := capJob(t)
	res := j.Run(context.Background(), runDay, keysOf("cik:1", "cik:2"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	queries, rows := len(h.queries), len(st.rows)
	res = j.Run(context.Background(), runDay, keysOf("cik:3"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || res.Returned != 0 {
		t.Fatalf("result %+v", res)
	}
	if len(h.queries) != queries || len(st.rows) != rows {
		t.Fatalf("denied key made requests %d->%d or rows %d->%d", queries, len(h.queries), rows, len(st.rows))
	}
	// a call with both kinds keeps the Retryable error and counts only the pulled bars
	res = j.Run(context.Background(), runDay, keysOf("cik:1", "cik:3"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("mixed result %+v", res)
	}
	// a key with stored bars is never denied, a key that began a first pull stays eligible
	for _, k := range []string{"cik:1", "cik:2"} {
		if r := j.Run(context.Background(), runDay, keysOf(k)); r.Err != nil {
			t.Fatalf("%s: %v", k, r.Err)
		}
	}
}

func TestCapDeniedKeyPullsOnALaterRun(t *testing.T) {
	t.Parallel()
	j, _, st := capJob(t)
	for _, k := range []string{"cik:1", "cik:2", "cik:3"} {
		j.Run(context.Background(), runDay, keysOf(k))
	}
	if len(st.rows) != 2 {
		t.Fatalf("rows %d", len(st.rows))
	}
	next := contracts.Day("2026-11-10")
	if res := j.Run(context.Background(), next, keysOf("cik:1", "cik:2")); res.Err != nil {
		t.Fatal(res.Err)
	}
	res := j.Run(context.Background(), next, keysOf("cik:3"))
	if res.Err != nil || res.Returned == 0 {
		t.Fatalf("result %+v", res)
	}
	var stored bool
	for _, r := range st.rows {
		stored = stored || r.InstrumentID == 103
	}
	if !stored {
		t.Fatalf("rows %+v", st.rows)
	}
}
