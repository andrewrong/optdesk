package fred

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func day(s string) types.SerializableDate {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: t}
}

func ptr(f float64) *float64 { return &f }

func TestMapping(t *testing.T) {
	t.Parallel()
	pub := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"observations":[
	  {"realtime_start":"2024-02-01","realtime_end":"2024-02-02","date":"2024-01-02","value":"."},
	  {"realtime_start":"2024-02-01","realtime_end":"2024-02-02","date":"2024-01-03","value":"1.5"}]}`)
	got, err := mapObservations("DGS10", body, pub)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Value != nil || got[0].Status != "no_value" {
		t.Errorf("dot: value %v status %q", got[0].Value, got[0].Status)
	}
	if got[1].Value == nil || *got[1].Value != 1.5 || got[1].Status != "present" {
		t.Errorf("value row: %+v", got[1])
	}
	if got[0].Unit != contracts.SeriesUnit("percent") {
		t.Errorf("unit = %q", got[0].Unit)
	}
	if !got[0].RealtimeStart.Equal(day("2024-02-01").Time) || !got[0].RealtimeEnd.Equal(day("2024-02-02").Time) {
		t.Errorf("vintage lost: %v %v", got[0].RealtimeStart, got[0].RealtimeEnd)
	}
	if !got[0].PublishedAt.Equal(pub) {
		t.Errorf("published_at = %v", got[0].PublishedAt)
	}
}

func TestMappingRejectsBadInput(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`not json`,
		`{"observations":[{"realtime_start":"2024-02-01","realtime_end":"2024-02-01","date":"x","value":"1"}]}`,
		`{"observations":[{"realtime_start":"2024-02-01","realtime_end":"2024-02-01","date":"2024-01-02","value":"abc"}]}`,
	} {
		if _, err := mapObservations("DGS10", []byte(body), time.Time{}); err == nil {
			t.Errorf("no error for %s", body)
		}
	}
}

func TestFixtureMapping(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("fixtures/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	pub := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
	got, err := mapObservations("DGS10", body, pub)
	if err != nil {
		t.Fatal(err)
	}
	mk := func(date string, v *float64, status string) contracts.SeriesPoint {
		return contracts.SeriesPoint{
			SeriesID: "DGS10", ObservationDate: day(date),
			RealtimeStart: day("2024-02-01"), RealtimeEnd: day("2024-02-01"),
			Value: v, Status: status, Unit: contracts.SeriesUnit("percent"), Currency: "",
			PublishedAt: pub, Source: "fred", SourceRevision: "2024-02-01",
		}
	}
	want := []contracts.SeriesPoint{
		mk("2024-01-02", ptr(3.95), "present"),
		mk("2024-01-03", nil, "no_value"),
		mk("2024-01-04", ptr(4.02), "present"),
	}
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	withValue := 0
	for i := range want {
		g, w := got[i], want[i]
		if (g.Value == nil) != (w.Value == nil) || (g.Value != nil && *g.Value != *w.Value) {
			t.Errorf("%d: value %v want %v", i, g.Value, w.Value)
		}
		if g.Value != nil {
			withValue++
		}
		g.Value, w.Value = nil, nil
		if g != w {
			t.Errorf("%d: got %+v want %+v", i, g, w)
		}
	}
	if withValue != 2 {
		t.Errorf("with value = %d", withValue)
	}
}

func TestMappingUnconfiguredSeries(t *testing.T) {
	t.Parallel()
	body := []byte(`{"observations":[
	  {"realtime_start":"2024-02-01","realtime_end":"2024-02-02","date":"2024-01-03","value":"1.5"}]}`)
	got, err := mapObservations("CPIAUCSL", body, time.Time{})
	if err == nil || got != nil {
		t.Fatalf("got %v, err %v", got, err)
	}
	if !strings.Contains(err.Error(), "CPIAUCSL") {
		t.Errorf("error does not name the series: %v", err)
	}
}

type seriesMeta struct {
	SeriesID string `json:"series_id"`
	Units    string `json:"units"`
}

// compareUnits reports every disagreement between a unit table and recorded FRED metadata.
func compareUnits(table map[string]contracts.SeriesUnit, meta []seriesMeta) []string {
	var fails []string
	recorded := map[string]string{}
	for _, m := range meta {
		recorded[m.SeriesID] = m.Units
	}
	for id, u := range table {
		units, ok := recorded[id]
		switch {
		case !ok:
			fails = append(fails, fmt.Sprintf("%s: no recorded metadata", id))
		case units != "Percent" && u == contracts.SeriesUnit("percent"):
			fails = append(fails, fmt.Sprintf("%s: table says percent, recorded units %q", id, units))
		case units != "Index" && u == indexPoints:
			fails = append(fails, fmt.Sprintf("%s: table says index_points, recorded units %q", id, units))
		}
	}
	for id, units := range recorded {
		u, ok := table[id]
		if !ok {
			continue
		}
		if units == "Percent" && u != contracts.SeriesUnit("percent") {
			fails = append(fails, fmt.Sprintf("%s: recorded Percent, table says %q", id, u))
		}
		if units == "Index" && u != indexPoints {
			fails = append(fails, fmt.Sprintf("%s: recorded Index, table says %q", id, u))
		}
	}
	return fails
}

func loadMeta(t *testing.T) []seriesMeta {
	t.Helper()
	raw, err := os.ReadFile("testdata/series_units.json")
	if err != nil {
		t.Fatal(err)
	}
	var meta []seriesMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	return meta
}

func TestSeriesUnitsMatchMetadata(t *testing.T) {
	t.Parallel()
	for _, f := range compareUnits(seriesUnits, loadMeta(t)) {
		t.Error(f)
	}
}

func TestSeriesUnitsDetectsWrongClaim(t *testing.T) {
	t.Parallel()
	table := map[string]contracts.SeriesUnit{}
	for k, v := range seriesUnits {
		table[k] = v
	}
	table["CPIAUCSL"] = contracts.SeriesUnit("percent")
	fails := compareUnits(table, loadMeta(t))
	if len(fails) != 1 || !strings.Contains(fails[0], "CPIAUCSL") {
		t.Errorf("fails = %v", fails)
	}
}

func TestMappingMarketSeries(t *testing.T) {
	t.Parallel()
	pub := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
	body := func(v string) []byte {
		return []byte(`{"observations":[{"realtime_start":"2024-02-01","realtime_end":"2024-02-01","date":"2024-01-03","value":"` + v + `"}]}`)
	}
	for _, id := range []string{"SP500", "VIXCLS", "VXNCLS", "RVXCLS", "VXVCLS"} {
		got, err := mapObservations(id, body("18.2"), pub)
		if err != nil || len(got) != 1 {
			t.Fatalf("%s: %v %v", id, got, err)
		}
		if got[0].Unit != indexPoints || got[0].Value == nil || *got[0].Value != 18.2 || got[0].Status != "present" {
			t.Errorf("%s: %+v", id, got[0])
		}
	}
	got, err := mapObservations("VIXCLS", body("."), pub)
	if err != nil || len(got) != 1 || got[0].Value != nil || got[0].Status != "no_value" || got[0].Unit != indexPoints {
		t.Errorf("VIXCLS dot: %+v %v", got, err)
	}
	got, err = mapObservations("DGS3MO", body("5.35"), pub)
	if err != nil || len(got) != 1 || got[0].Value == nil || *got[0].Value != 5.35 || got[0].Unit != contracts.SeriesUnit("percent") {
		t.Errorf("DGS3MO: %+v %v", got, err)
	}
}

func TestSeriesUnitsDetectsIndexPointsOnPercent(t *testing.T) {
	t.Parallel()
	table := map[string]contracts.SeriesUnit{"DGS10": indexPoints}
	fails := compareUnits(table, loadMeta(t))
	if len(fails) != 2 {
		t.Errorf("fails = %v", fails)
	}
}

// indexPoints is the unit of the index series; the contracts package does not re-export the constant.
const indexPoints = contracts.SeriesUnit("index_points")
