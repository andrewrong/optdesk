package ops

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

func TestRunsOnReadsView(t *testing.T) {
	t.Parallel()
	fin := time.Date(2026, 11, 12, 21, 5, 0, 0, time.UTC)
	var none *time.Time
	f := &fakeDB{rows: [][]any{
		{"obs.option_open", "AAPL", "ok", &fin, 10, 9, "", ""},
		{"obs.option_open", "MSFT", "running", none, 0, 0, "", ""},
	}}
	got, err := (&runs{db: f}).RunsOn(context.Background(), "2026-11-12")
	if err != nil {
		t.Fatal(err)
	}
	want := []runview.Row{
		{SessionDay: "2026-11-12", Dataset: "obs.option_open", Key: "AAPL", Status: "ok", FinishedAt: fin, Returned: 10, Usable: 9},
		{SessionDay: "2026-11-12", Dataset: "obs.option_open", Key: "MSFT", Status: "running"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v want %+v", got, want)
	}
	empty, err := (&runs{db: &fakeDB{}}).RunsOn(context.Background(), "2026-11-12")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("empty = %#v, %v", empty, err)
	}
	if _, err := (&runs{db: f}).RunsOn(context.Background(), "nope"); err == nil || kind(t, err) != contracts.Permanent {
		t.Errorf("bad day err = %v", err)
	}
}

func TestReplaceReasonsUpserts(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	at := time.Date(2026, 11, 12, 21, 5, 0, 0, time.UTC)
	err := (&runs{db: f}).ReplaceReasons(context.Background(), "2026-11-12", "obs.option_open", "AAPL", []runview.Reason{
		{Scope: "AAPL", Code: "no_chain", Detail: "x", At: at}, {Scope: "MSFT", Code: "no_close", At: at}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) || len(f.execSQL) != 3 {
		t.Fatalf("events %v sqls %d", f.events, len(f.execSQL))
	}
	if !strings.HasPrefix(f.execSQL[0], "DELETE FROM ops.option_run_reason") || !strings.Contains(f.execSQL[1], "ON CONFLICT") {
		t.Errorf("sqls = %v", f.execSQL)
	}
	if a := f.execArgs[2]; a[2] != "AAPL" || a[3] != "MSFT" || a[4] != "no_close" || a[6] != at {
		t.Errorf("args = %v", a)
	}
	// A clean run is a delete alone.
	f = &fakeDB{}
	if err := (&runs{db: f}).ReplaceReasons(context.Background(), "2026-11-12", "obs.option_open", "AAPL", nil); err != nil || len(f.execSQL) != 1 {
		t.Errorf("clean run: %v, %v", f.execSQL, err)
	}
}

func TestReasonsReadNeverNilAndNotWiredUsesItsKey(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	r := &runs{db: f}
	for name, call := range map[string]func() ([]runview.Reason, error){
		"on":        func() ([]runview.Reason, error) { return r.ReasonsOn(context.Background(), "2026-11-12") },
		"not wired": func() ([]runview.Reason, error) { return r.NotWired(context.Background()) },
	} {
		got, err := call()
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%s: %#v, %v", name, got, err)
		}
	}
}
