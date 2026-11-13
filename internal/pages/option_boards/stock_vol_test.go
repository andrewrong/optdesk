package option_boards

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/atombender/go-jsonschema/pkg/types"
)

type volKey struct{ id, subject string }

// volFake serves stored readings by id and subject; it ignores the window, so the builder must apply it.
type volFake struct {
	rows map[volKey][]contracts.ReadingRecord
}

func (f volFake) WinnerReadings(_ context.Context, name, _, subject string, _, _ contracts.Day) ([]contracts.ReadingRecord, error) {
	return f.rows[volKey{name, subject}], nil
}
func (volFake) Close() error { return nil }

func volRec(day, value string) contracts.ReadingRecord {
	t, err := time.Parse(dayLayout, day)
	if err != nil {
		panic(err)
	}
	r := contracts.ReadingRecord{SubjectKind: "underlying", State: "ok", Day: types.SerializableDate{Time: t}}
	if value != "" {
		r.Value = &value
	}
	return r
}

func volMissing(day string, code int) contracts.ReadingRecord {
	r := volRec(day, "")
	r.State, r.ReasonCode = gen.ReadingRecordStateMissingInput, &code
	return r
}

var allIDs = []string{
	"market.iv_ibkr", "market.rv20", "market.rv60", "market.rv120",
	"market.vol_forecast", "market.vol_edge", "market.ivp_ibkr", "market.vol_forecast_err",
}
var allLabels = []string{"iv_name", "rv20", "rv60", "rv120", "forecast", "edge", "ivp", "forecast_err"}

// figuresOf stores one ok reading of day for every id of the subject; edge is the text of market.vol_edge.
func figuresOf(subject, day, edge string) volFake {
	f := volFake{rows: map[volKey][]contracts.ReadingRecord{}}
	for i, id := range allIDs {
		v := "0.2" + string(rune('0'+i)) + "0000"
		if id == "market.vol_edge" {
			v = edge
		}
		f.rows[volKey{id, subject}] = []contracts.ReadingRecord{volRec(day, v)}
	}
	return f
}

func volOf(t *testing.T, rd store.ReadingViewReader, rows []store.Candidate, key string) volView {
	t.Helper()
	c := stockCfg(stockDay, rows, key)
	c.Readings = rd
	p, _ := buildStockPayload(t, c, stockDay)
	if len(p.VolView.Rows) != 8 {
		t.Fatalf("want 8 rows, got %+v", p.VolView)
	}
	for i, r := range p.VolView.Rows {
		if r.Label != allLabels[i] {
			t.Fatalf("row %d label %q, want %q", i, r.Label, allLabels[i])
		}
	}
	return p.VolView
}

func wantAllGaps(t *testing.T, v volView) {
	t.Helper()
	for _, r := range v.Rows {
		if r.Gap == "" || r.Text != "" || r.DataDay != "" {
			t.Errorf("row %+v is not a plain gap", r)
		}
	}
}

func wantAllOk(t *testing.T, v volView) {
	t.Helper()
	for _, r := range v.Rows {
		if r.Gap != "" || r.Text == "" || r.DataDay == "" {
			t.Errorf("row %+v is not ok", r)
		}
	}
}

func TestVolViewNoRow(t *testing.T) {
	t.Parallel()
	v := volOf(t, figuresOf("NVDA", "2026-11-10", "0.030000"), stockFixture(), "NVDA")
	wantAllOk(t, v)
	if v.Name != "NVDA" {
		t.Errorf("name %q", v.Name)
	}
}

func TestVolViewBlockedRow(t *testing.T) {
	t.Parallel()
	blocked := []store.Candidate{stockRowOf(103, issuerApple, "2026-11-13", 3, "60", "blocked", nil, nil, gatesThin)}
	wantAllOk(t, volOf(t, figuresOf("AAPL", "2026-11-10", "0.030000"), blocked, "AAPL"))
}

func TestVolViewNoFigures(t *testing.T) {
	t.Parallel()
	v := volOf(t, volFake{}, stockFixture(), "AAPL")
	wantAllGaps(t, v)
	for _, r := range v.Rows {
		if r.Gap != "no_input" {
			t.Errorf("gap %q", r.Gap)
		}
	}
	if v.Coaching != volNoEdge {
		t.Errorf("coaching %q", v.Coaching)
	}
}

func TestVolViewNoImplied(t *testing.T) {
	t.Parallel()
	f := figuresOf("AAPL", "2026-11-10", "0.030000")
	delete(f.rows, volKey{"market.iv_ibkr", "AAPL"})
	v := volOf(t, f, stockFixture(), "AAPL")
	if v.Rows[0].Gap != "no_input" || v.Rows[0].Text != "" {
		t.Errorf("iv_name %+v", v.Rows[0])
	}
	for _, r := range v.Rows[1:4] {
		if r.Gap != "" {
			t.Errorf("%s is a gap", r.Label)
		}
	}
}

func TestVolViewDataDay(t *testing.T) {
	t.Parallel()
	f := figuresOf("AAPL", "2026-11-10", "0.030000")
	f.rows[volKey{"market.rv60", "AAPL"}] = []contracts.ReadingRecord{volRec("2026-11-06", "0.250000")}
	f.rows[volKey{"market.rv120", "AAPL"}] = []contracts.ReadingRecord{volMissing("2026-11-09", 2)}
	v := volOf(t, f, stockFixture(), "AAPL")
	for _, r := range v.Rows {
		want := "2026-11-10"
		switch r.Label {
		case "rv60":
			want = "2026-11-06"
		case "rv120":
			want = "2026-11-09"
			if r.Gap != "short_history" || r.Text != "" {
				t.Errorf("rv120 %+v", r)
			}
		}
		if r.DataDay != want {
			t.Errorf("%s data_day %q, want %q", r.Label, r.DataDay, want)
		}
	}
	if v.Rows[2].Text != "0.250000" {
		t.Errorf("rv60 text %q", v.Rows[2].Text)
	}
}

func TestVolViewNilReader(t *testing.T) {
	t.Parallel()
	v := volOf(t, nil, stockFixture(), "AAPL")
	wantAllGaps(t, v)
	if v.Method == "" || v.Coaching != volNoEdge {
		t.Errorf("view %+v", v)
	}
}

func TestVolViewIssuerKey(t *testing.T) {
	t.Parallel()
	f := figuresOf("AAPL", "2026-11-10", "0.030000")
	wantAllOk(t, volOf(t, f, stockFixture(), issuerApple))
	// an issuer key with no candidate row has no known ticker: its upper case finds no reading
	wantAllGaps(t, volOf(t, f, nil, issuerApple))
}

func TestVolViewWindow(t *testing.T) {
	t.Parallel()
	f := figuresOf("AAPL", "2026-11-10", "0.030000")
	f.rows[volKey{"market.rv20", "AAPL"}] = []contracts.ReadingRecord{volRec("2026-11-02", "0.200000")} // 8 days old
	f.rows[volKey{"market.rv60", "AAPL"}] = []contracts.ReadingRecord{volRec("2026-11-07", "0.210000")} // 3 days old
	v := volOf(t, f, stockFixture(), "AAPL")
	if r := v.Rows[1]; r.Gap != "no_input" || r.DataDay != "" {
		t.Errorf("rv20 %+v", r)
	}
	if r := v.Rows[2]; r.Gap != "" || r.DataDay != "2026-11-07" || r.Text != "0.210000" {
		t.Errorf("rv60 %+v", r)
	}
}

func TestVolViewFollowedName(t *testing.T) {
	t.Parallel()
	// a name with stored figures and no candidate row at all (no candidate reader rows)
	wantAllOk(t, volOf(t, figuresOf("TSLA", "2026-11-10", "-0.020000"), nil, "tsla"))
}

func TestVolViewMethod(t *testing.T) {
	t.Parallel()
	for _, edge := range []string{"0.030000", "-0.030000", "0.000000", ""} {
		f := figuresOf("AAPL", "2026-11-10", edge)
		if edge == "" {
			delete(f.rows, volKey{"market.vol_edge", "AAPL"})
		}
		v := volOf(t, f, stockFixture(), "AAPL")
		for _, s := range []string{"its horizon is not stated", "not the same horizon", "not to a strike or an expiry"} {
			if !strings.Contains(v.Method, s) {
				t.Errorf("method lacks %q", s)
			}
		}
		if !strings.HasSuffix(v.Coaching, "not a track record.") || !strings.Contains(v.Coaching, "does not know the next earnings move") {
			t.Errorf("edge %q coaching %q", edge, v.Coaching)
		}
	}
}

func TestVolSentenceCases(t *testing.T) {
	t.Parallel()
	tail := " This page does not know the next earnings move, the skew, how well this forecast has fitted this name " +
		"(see the error row), early assignment, or the horizon of implied. One past error is shown, not a track record."
	above := "Implied volatility is above the forecast by the edge shown." + tail
	below := "Implied volatility is below the forecast by the edge shown." + tail
	inside := "Implied volatility is within one volatility point of the forecast." + tail
	none := "No edge is shown because an input is missing; the rows name it." + tail
	for edge, want := range map[string]string{
		"0.030000": above, "-0.030000": below, "0.004000": inside, "-0.004000": inside,
		"0.010000": inside, "-0.010000": inside, "0.010001": above, "-0.010001": below, "": none,
	} {
		if got := volSentence(edge); got != want {
			t.Errorf("edge %q: %q", edge, got)
		}
	}
}
