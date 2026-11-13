package home_market

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
)

var pageDay = time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)

func ivrvReading(state, value, dataDay string, code *int) *contracts.ReadingRecord {
	r := &contracts.ReadingRecord{ReasonCode: code}
	r.State = "missing_input"
	if state == "ok" {
		r.State = "ok"
	}
	if value != "" {
		r.Value = &value
	}
	if dataDay != "" {
		r.Parameters = []gen.ReadingRecordParametersElem{{Param: "data_day", ValueTag: &dataDay}}
	}
	return r
}

func wantFigure(t *testing.T, c any, value, text string) {
	t.Helper()
	v, ok := c.(volPoints)
	if !ok || v.Kind != "value" || string(v.Value) != value || v.Text != text || v.Unit != "vol_points" {
		t.Fatalf("cell = %+v, want value %s text %s", c, value, text)
	}
}

func wantGap(t *testing.T, c any, reason, text string) {
	t.Helper()
	g, ok := c.(gap)
	if !ok || g.Reason != reason || g.Text != text {
		t.Fatalf("cell = %+v, want gap %s %q", c, reason, text)
	}
}

func TestIvRvCellSigned(t *testing.T) {
	t.Parallel()
	wantFigure(t, ivRvCell("SPX", ivrvReading("ok", "0.032000", "2026-11-13", nil), pageDay), "3.2", "+3.2")
	wantFigure(t, ivRvCell("SPX", ivrvReading("ok", "-0.014000", "2026-11-13", nil), pageDay), "-1.4", "-1.4")
	wantFigure(t, ivRvCell("SPX", ivrvReading("ok", "0.000000", "2026-11-13", nil), pageDay), "0.0", "0.0")
}

func TestIvRvCellNamesDayWhenNotPageDay(t *testing.T) {
	t.Parallel()
	wantFigure(t, ivRvCell("SPX", ivrvReading("ok", "0.032000", "2026-11-12", nil), pageDay), "3.2", "+3.2 (11-12)")
	wantFigure(t, ivRvCell("SPX", ivrvReading("ok", "0.032000", "2026-11-13", nil), pageDay), "3.2", "+3.2")
}

func TestIvRvCellStaleBeyondFiveDays(t *testing.T) {
	t.Parallel()
	wantGap(t, ivRvCell("SPX", ivrvReading("ok", "0.032000", "2026-11-07", nil), pageDay), "stale_input",
		"SPX Implied minus realised: data is stale, data day 11-07")
	wantFigure(t, ivRvCell("SPX", ivrvReading("ok", "0.032000", "2026-11-08", nil), pageDay), "3.2", "+3.2 (11-08)")
}

func TestIvRvCellStoredCode3IsStale(t *testing.T) {
	t.Parallel()
	three := 3
	wantGap(t, ivRvCell("SPX", ivrvReading("missing_input", "", "2026-11-05", &three), pageDay), "stale_input",
		"SPX Implied minus realised: data is stale, data day 11-05")
	wantGap(t, ivRvCell("SPX", ivrvReading("missing_input", "", "", &three), pageDay), "stale_input",
		"SPX Implied minus realised: data is stale")
}

func TestIvRvCellNoRowIsNoData(t *testing.T) {
	t.Parallel()
	wantGap(t, ivRvCell("NDX", nil, pageDay), "no_data", "NDX Implied minus realised: no data, not stored yet")
}

func TestIvRvCellMissingIsMissingInput(t *testing.T) {
	t.Parallel()
	one := 1
	want := "RUT Implied minus realised: no data, inputs incomplete"
	wantGap(t, ivRvCell("RUT", ivrvReading("missing_input", "", "2026-11-13", &one), pageDay), "missing_input", want)
	wantGap(t, ivRvCell("RUT", ivrvReading("ok", "0.032000", "", nil), pageDay), "missing_input", want)
	wantGap(t, ivRvCell("RUT", ivrvReading("ok", "0.032000", "garbage", nil), pageDay), "missing_input", want)
}
