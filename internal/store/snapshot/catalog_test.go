package snapshot

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func catalogDay(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCatalog(t *testing.T) {
	t.Parallel()
	f := &fakeDB{rows: [][]any{
		{int64(8), catalogDay(t, "2026-11-06")},
		{int64(12), catalogDay(t, "2026-11-04")},
	}}
	got, err := (&catalogStore{db: f}).OptionEodLatest(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	want := []store.OptionEodLatest{{UnderlyingID: 8, SessionDay: "2026-11-06"}, {UnderlyingID: 12, SessionDay: "2026-11-04"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if !strings.Contains(f.selSQL, "ORDER BY underlying_id") {
		t.Fatalf("statement does not order by underlying_id: %s", f.selSQL)
	}

	empty, err := (&catalogStore{db: &fakeDB{}}).OptionEodLatest(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("no row gave %#v, want an empty non-nil list", empty)
	}
}

func TestCatalogSince(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		since     contracts.Day
		wantWhere bool
		wantArgs  []any
		permanent bool
	}{
		{name: "empty", since: ""},
		{name: "day", since: "2026-11-01", wantWhere: true, wantArgs: []any{catalogDay(t, "2026-11-01")}},
		{name: "not a day", since: "11/01/2026", permanent: true},
		{name: "instant", since: "2026-11-01T00:00:00Z", permanent: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeDB{}
			_, err := (&catalogStore{db: f}).OptionEodLatest(context.Background(), c.since)
			if c.permanent {
				assertKind(t, err, contracts.Permanent, nil)
				if f.selSQL != "" || len(f.ctxs) != 0 {
					t.Fatalf("the seam saw a call: %q", f.selSQL)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if has := strings.Contains(f.selSQL, "WHERE"); has != c.wantWhere {
				t.Fatalf("WHERE present = %v, want %v: %s", has, c.wantWhere, f.selSQL)
			}
			if len(f.selArgs) != len(c.wantArgs) || (len(c.wantArgs) > 0 && !reflect.DeepEqual(f.selArgs, c.wantArgs)) {
				t.Fatalf("args %v, want %v", f.selArgs, c.wantArgs)
			}
		})
	}
}

func TestCatalogErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	f := &fakeDB{selErr: boom}
	got, err := (&catalogStore{db: f}).OptionEodLatest(context.Background(), "")
	assertKind(t, err, contracts.Retryable, boom)
	if got != nil {
		t.Fatalf("got %v with an error", got)
	}
	if !strings.Contains(f.selSQL, "rd.option_eod_r1") {
		t.Fatalf("statement does not read rd.option_eod_r1: %s", f.selSQL)
	}
	if strings.Contains(f.selSQL, "obs"+".") {
		t.Fatalf("statement names the table: %s", f.selSQL)
	}
}
