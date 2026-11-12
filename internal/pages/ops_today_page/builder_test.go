package ops_today_page

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/store/runview"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

var (
	t0  = time.Date(2026, 11, 12, 14, 0, 0, 0, time.UTC)
	now = t0.Add(time.Hour)
)

type outRow struct {
	Dataset    string  `json:"dataset"`
	Key        string  `json:"key"`
	Market     string  `json:"market"`
	Status     string  `json:"status"`
	ErrorCode  *string `json:"error_code"`
	ReasonCode *string `json:"reason_code"`
	ReturnedN  int     `json:"returned_n"`
	UsableN    int     `json:"usable_n"`
	Detail     *string `json:"detail"`
}

func rowsOf(t *testing.T, runs []runview.Row, reasons []runview.Reason) []outRow {
	t.Helper()
	b, err := Build(runs, reasons, now)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Today struct {
			Rows []outRow `json:"rows"`
		} `json:"today"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p.Today.Rows
}

func run(dataset, key, status string, at time.Time) runview.Row {
	return runview.Row{SessionDay: "2026-11-12", Dataset: dataset, Key: key, Status: status, FinishedAt: at}
}

func TestNewestRunPerKeyOnly(t *testing.T) {
	t.Parallel()
	var runs []runview.Row
	for i := 0; i < 40; i++ {
		r := run("obs.option_open", "k1", "retryable", t0.Add(time.Duration(i)*time.Minute))
		r.Returned = i
		runs = append(runs, r)
	}
	runs = append(runs, run("obs.option_open", "k2", "ok", t0))
	// the newest of k1 comes first in the input: order must not matter
	runs[0], runs[39] = runs[39], runs[0]
	rows := rowsOf(t, runs, nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Key != "k1" || rows[0].ReturnedN != 39 {
		t.Fatalf("k1 row = %+v", rows[0])
	}
}

func TestRunningRowIsSkipped(t *testing.T) {
	t.Parallel()
	runs := []runview.Row{
		run("obs.option_open", "k1", "retryable", t0),
		run("obs.option_open", "k1", "running", time.Time{}),
		run("obs.option_open", "k2", "running", time.Time{}),
	}
	rows := rowsOf(t, runs, nil)
	if len(rows) != 1 || rows[0].Key != "k1" || rows[0].Status != "retryable" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestReasonJoinedToRun(t *testing.T) {
	t.Parallel()
	r := run("obs.option_open", "k1", "no_value", t0)
	r.ErrorCode = "e1"
	reasons := []runview.Reason{
		{Dataset: "obs.option_open", Subject: "k1", Scope: "AAPL", Code: "no_chain", Detail: "d1"},
		{Dataset: "obs.option_open", Subject: "k1", Scope: "MSFT", Code: "no_close", Detail: "d2"},
		{Dataset: "obs.option_open", Subject: "other", Code: "no_spot", Detail: "x"},
		{Dataset: "obs.option_open.jp", Subject: "k1", Code: "no_spot", Detail: "y"},
	}
	rows := rowsOf(t, []runview.Row{r}, reasons)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	got := rows[0]
	if got.ReasonCode == nil || *got.ReasonCode != "no_chain" || got.ErrorCode == nil || *got.ErrorCode != "e1" {
		t.Fatalf("row = %+v", got)
	}
	if got.Detail == nil || *got.Detail != "AAPL: d1; MSFT: d2" {
		t.Fatalf("detail = %v", got.Detail)
	}
	bare := rowsOf(t, []runview.Row{run("obs.option_open", "k9", "ok", t0)}, reasons)
	if bare[0].ReasonCode != nil || bare[0].Detail != nil || bare[0].ErrorCode != nil {
		t.Fatalf("bare row = %+v", bare[0])
	}
}

func TestNotWiredRowHasKeyNameOnly(t *testing.T) {
	t.Parallel()
	reasons := []runview.Reason{
		{Day: runview.NotWiredDay, Dataset: runview.NotWiredDataset, Subject: "scheduler", Scope: "option_open",
			Code: "not_wired", Detail: "OT_PG_DSN_OPTION_OPEN"},
		{Day: runview.NotWiredDay, Dataset: runview.NotWiredDataset, Subject: "scheduler", Scope: "list",
			Code: "not_wired", Detail: "OT_PG_DSN_LIST"},
	}
	rows := rowsOf(t, nil, reasons)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	for i, want := range []struct{ key, name string }{{"option_open", "OT_PG_DSN_OPTION_OPEN"}, {"list", "OT_PG_DSN_LIST"}} {
		r := rows[i]
		if r.Key != want.key || r.Status != "not_wired" || r.ReasonCode == nil || *r.ReasonCode != "not_wired" ||
			r.Detail == nil || *r.Detail != want.name {
			t.Fatalf("row %d = %+v", i, r)
		}
	}
}

func TestMarketFromDatasetSuffix(t *testing.T) {
	t.Parallel()
	var runs []runview.Row
	for _, d := range []string{"plan.option_open_item", "plan.option_open_item.jp", "obs.option_open", "obs.option_open.jp",
		"master.option_list", "master.option_list.jp", "master.option_contract", "master.option_contract.jp"} {
		runs = append(runs, run(d, "k", "ok", t0))
	}
	rows := rowsOf(t, runs, nil)
	if len(rows) != 8 {
		t.Fatalf("rows = %+v", rows)
	}
	for _, r := range rows {
		want := "us"
		if strings.HasSuffix(r.Dataset, ".jp") {
			want = "jp"
		}
		if r.Market != want {
			t.Errorf("%s market = %s", r.Dataset, r.Market)
		}
	}
}

func TestOtherDatasetsNotListed(t *testing.T) {
	t.Parallel()
	runs := []runview.Row{
		run("obs.series_point", "k", "ok", t0),
		run("ops.not_wired", "scheduler", "ok", t0),
		run("obs.option_open_extra", "k", "ok", t0),
	}
	if rows := rowsOf(t, runs, nil); len(rows) != 0 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestNoRowsGivesEmptyPayload(t *testing.T) {
	t.Parallel()
	b, err := Build(nil, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"today":{"rows":[]}`) || !strings.Contains(string(b), `"day":"2026-11-12"`) {
		t.Fatalf("payload = %s", b)
	}
	validate(t, b)
}

func TestPayloadValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("é", 500)
	runs := []runview.Row{run("master.option_list.jp", "k", "ok", t0), run("obs.option_open", "k", "failed", t0)}
	runs[1].ErrorCode = "boom"
	reasons := []runview.Reason{
		{Dataset: "obs.option_open", Subject: "k", Scope: "A", Code: "no_chain", Detail: long},
		{Dataset: runview.NotWiredDataset, Subject: "scheduler", Scope: "list", Code: "not_wired", Detail: "OT_X"},
	}
	b, err := Build(runs, reasons, now)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
}

func TestListJobReasonsShownOnItsRow(t *testing.T) {
	t.Parallel()
	for _, ds := range []string{"master.option_list", "master.option_list.jp"} {
		for _, code := range []string{"held_stale", "outside_range", "no_spot", "late_start"} {
			r := run(ds, "zone", "ok", t0)
			reasons := []runview.Reason{{Dataset: ds, Subject: "zone", Scope: "AAPL", Code: code, Detail: "d"}}
			rows := rowsOf(t, []runview.Row{r}, reasons)
			if len(rows) != 1 || rows[0].ReasonCode == nil || *rows[0].ReasonCode != code || rows[0].Detail == nil {
				t.Errorf("%s %s: rows = %+v", ds, code, rows)
			}
		}
	}
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(schemaDir + file)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	schema, err := c.Compile(schemaHost + "page.ops_today_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(v); err != nil {
		t.Errorf("%v\n%s", err, b)
	}
}
