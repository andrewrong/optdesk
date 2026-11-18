package basefactpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/basefact"
)

// fakeSource answers by the view name found in the query.
type fakeSource struct {
	rows map[string][]record
	errs map[string]error
	wait map[string]bool // wait for the context instead of answering
}

func (f fakeSource) Rows(ctx context.Context, sql string) ([]record, error) {
	for _, v := range views {
		if !strings.Contains(sql, v.name) {
			continue
		}
		if f.wait[v.group] {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		if err := f.errs[v.group]; err != nil {
			return nil, err
		}
		return f.rows[v.group], nil
	}
	return nil, errors.New("unknown view")
}

func (fakeSource) Close() error { return nil }

func newTest(src source) *reader { return &reader{src: src, timeout: time.Millisecond} }

func day(s string) *time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return &t
}

var v = "v"

func four() map[string][]record {
	return map[string][]record{
		basefact.GroupBars:    {{Dataset: "bars", Rule: "newest", Subject: "USD", N: 1, Day: day("2026-11-17"), Value: &v}},
		basefact.GroupOption:  {{Dataset: "quotes", Rule: "rows", N: 5, Bad: 1}},
		basefact.GroupOther:   {{Dataset: "rates", Rule: "rows", N: 3}},
		basefact.GroupAccount: {{Dataset: "account", Rule: "rows", N: 2}},
	}
}

func TestFactsMergeFourViews(t *testing.T) {
	t.Parallel()
	res, err := newTest(fakeSource{rows: four()}).Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 4 || len(res.Failed) != 0 || res.Failed == nil {
		t.Fatalf("facts %d failed %v", len(res.Facts), res.Failed)
	}
	var bars basefact.Fact
	for _, f := range res.Facts {
		if f.Dataset == "bars" {
			bars = f
		}
	}
	want := basefact.Fact{Dataset: "bars", Rule: "newest", Subject: "USD", N: 1, Day: "2026-11-17", Value: "v"}
	if bars != want {
		t.Fatalf("bars = %+v, want %+v", bars, want)
	}
}

func TestFactsOrder(t *testing.T) {
	t.Parallel()
	rows := four()
	rows[basefact.GroupBars] = []record{
		{Dataset: "bars", Rule: "rows", Subject: "b"},
		{Dataset: "bars", Rule: "newest", Subject: "z"},
		{Dataset: "bars", Rule: "newest", Subject: "a"},
	}
	res, err := newTest(fakeSource{rows: rows}).Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Facts {
		got = append(got, f.Dataset+"/"+f.Rule+"/"+f.Subject)
	}
	want := "account/rows/,bars/newest/a,bars/newest/z,bars/rows/b,quotes/rows/,rates/rows/"
	if strings.Join(got, ",") != want {
		t.Fatalf("order = %v, want %s", got, want)
	}
}

func TestFactsNullDay(t *testing.T) {
	t.Parallel()
	rows := map[string][]record{basefact.GroupBars: {{Dataset: "bars", Rule: "newest"}}}
	res, err := newTest(fakeSource{rows: rows}).Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 || res.Facts[0].Day != "" {
		t.Fatalf("facts = %+v, want empty day", res.Facts)
	}
}

func TestNullValueIsEmpty(t *testing.T) {
	t.Parallel()
	rows := map[string][]record{basefact.GroupBars: {{Dataset: "bars", Rule: "rows", Value: nil, Day: nil}}}
	res, err := newTest(fakeSource{rows: rows}).Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facts) != 1 || res.Facts[0].Value != "" || res.Facts[0].Day != "" || len(res.Failed) != 0 {
		t.Fatalf("facts = %+v failed = %v, want one fact with empty value and day", res.Facts, res.Failed)
	}
}

func TestOneViewFailsOthersStay(t *testing.T) {
	t.Parallel()
	src := fakeSource{rows: four(), errs: map[string]error{basefact.GroupOption: errors.New("boom")}}
	res, err := newTest(src).Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failed) != 1 || res.Failed[0] != basefact.GroupOption {
		t.Fatalf("failed = %v", res.Failed)
	}
	if len(res.Facts) != 3 {
		t.Fatalf("facts = %+v, want 3", res.Facts)
	}
	for _, f := range res.Facts {
		if f.Dataset == "quotes" {
			t.Fatalf("fact of failed view kept: %+v", f)
		}
	}
}

func TestOneViewTimesOut(t *testing.T) {
	t.Parallel()
	src := fakeSource{rows: four(), wait: map[string]bool{basefact.GroupOther: true}}
	res, err := newTest(src).Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failed) != 1 || res.Failed[0] != basefact.GroupOther || len(res.Facts) != 3 {
		t.Fatalf("failed = %v, facts = %d", res.Failed, len(res.Facts))
	}
}

func TestAllViewsFailReturnsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	errs := map[string]error{}
	for _, v := range views {
		errs[v.group] = boom
	}
	res, err := newTest(fakeSource{errs: errs}).Facts(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if res.Facts == nil || len(res.Facts) != 0 || len(res.Failed) != 4 {
		t.Fatalf("res = %+v", res)
	}
}
