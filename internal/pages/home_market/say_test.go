package home_market_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/pages/home_market"
)

const pageDay = contracts.Day("2026-11-06")

func tagged(state, g1, g3, dataDay string) *contracts.ReadingRecord {
	tag := func(n, v string) gen.ReadingRecordParametersElem {
		return gen.ReadingRecordParametersElem{Param: n, ValueTag: &v}
	}
	var d types.SerializableDate
	_ = d.UnmarshalJSON([]byte(`"` + dataDay + `"`))
	return &contracts.ReadingRecord{
		Day: d, State: "ok",
		Parameters: []gen.ReadingRecordParametersElem{
			tag("state", state), tag("gap_1m", g1), tag("gap_3m", g3), tag("data_day", dataDay),
		},
	}
}

func TestFromReadingEachState(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"LEADING":      "科技近 1 个月、3 个月都跑赢大盘（试行，不是预测）。",
		"IN LINE":      "科技与大盘大体同步（试行，不是预测）。",
		"LAGGING":      "科技近 1 个月、3 个月都跑输大盘。科技股的新 put：先等一等，或减量、选更远的行权价（试行，不是预测）。",
		"LAGGING HARD": "科技近 3 个月明显跑输大盘。科技股的新 put：先等一等，或减量、选更远的行权价（试行，不是预测）。",
	}
	for state, sentence := range want {
		v := home_market.FromReading(tagged(state, "-1.00", "-2.00", "2026-11-06"), pageDay)
		if v.State != state || v.Say != sentence || v.Gap != "" {
			t.Errorf("%s: view = %+v", state, v)
		}
	}
}

func TestFromReadingNumbersLine(t *testing.T) {
	t.Parallel()
	v := home_market.FromReading(tagged("LAGGING", "-3.12", "-6.40", "2026-11-06"), pageDay)
	want := "NDX minus SPX: 1 month -3.12 pt, 3 months -6.40 pt; trial thresholds, not a forecast"
	if v.Numbers != want {
		t.Fatalf("numbers = %q", v.Numbers)
	}
}

func TestFromReadingNilIsNoData(t *testing.T) {
	t.Parallel()
	if v := home_market.FromReading(nil, pageDay); v.Gap != "no_data" || v.State != "" {
		t.Fatalf("view = %+v", v)
	}
}

func TestFromReadingOldDataDayIsStale(t *testing.T) {
	t.Parallel()
	if v := home_market.FromReading(tagged("LEADING", "1.00", "2.00", "2026-10-31"), pageDay); v.Gap != "stale_input" {
		t.Fatalf("6 days old: %+v", v)
	}
	if v := home_market.FromReading(tagged("LEADING", "1.00", "2.00", "2026-11-01"), pageDay); v.Gap != "" {
		t.Fatalf("5 days old: %+v", v)
	}
}

func TestFromReadingReasonThreeIsStale(t *testing.T) {
	t.Parallel()
	r := &contracts.ReadingRecord{State: "missing_input"}
	three := 3
	r.ReasonCode = &three
	if v := home_market.FromReading(r, pageDay); v.Gap != "stale_input" {
		t.Fatalf("view = %+v", v)
	}
}

func TestFromReadingNotOkIsMissingInput(t *testing.T) {
	t.Parallel()
	r := tagged("LEADING", "1.00", "2.00", "2026-11-06")
	r.State = "missing_input"
	two := 2
	r.ReasonCode = &two
	if v := home_market.FromReading(r, pageDay); v.Gap != "missing_input" || v.State != "" {
		t.Fatalf("view = %+v", v)
	}
}

func TestFromReadingUnknownStateIsMissingInput(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"SIDEWAYS", ""} {
		if v := home_market.FromReading(tagged(s, "1.00", "2.00", "2026-11-06"), pageDay); v.Gap != "missing_input" {
			t.Errorf("state %q: %+v", s, v)
		}
	}
}

func TestSentencesAreShort(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"LEADING", "IN LINE", "LAGGING", "LAGGING HARD"} {
		v := home_market.FromReading(tagged(state, "1.00", "2.00", "2026-11-06"), pageDay)
		if n := utf8.RuneCountInString(v.Say); n < 1 || n > 200 || strings.TrimSpace(v.Say) == "" {
			t.Errorf("%s: %d characters", state, n)
		}
	}
}
