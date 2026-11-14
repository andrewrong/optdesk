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
func compareUnits(table map[string]unitRow, meta []seriesMeta) []string {
	var fails []string
	recorded := map[string]string{}
	for _, m := range meta {
		recorded[m.SeriesID] = m.Units
	}
	for id, row := range table {
		units, ok := recorded[id]
		u := row.unit
		switch {
		case !ok:
			fails = append(fails, fmt.Sprintf("%s: no recorded metadata", id))
		case units != "Percent" && u == contracts.SeriesUnit("percent"):
			fails = append(fails, fmt.Sprintf("%s: table says percent, recorded units %q", id, units))
		case units != "Index" && u == indexPoints:
			fails = append(fails, fmt.Sprintf("%s: table says index_points, recorded units %q", id, units))
		case u == ccyAmount && !ccyTextMatchesShift(units, row.shift):
			fails = append(fails, fmt.Sprintf("%s: ccy_amount shift %d does not match recorded units %q", id, row.shift, units))
		}
	}
	for id, units := range recorded {
		row, ok := table[id]
		if !ok {
			continue
		}
		u := row.unit
		if units == "Percent" && u != contracts.SeriesUnit("percent") {
			fails = append(fails, fmt.Sprintf("%s: recorded Percent, table says %q", id, u))
		}
		if units == "Index" && u != indexPoints {
			fails = append(fails, fmt.Sprintf("%s: recorded Index, table says %q", id, u))
		}
		if strings.HasSuffix(units, "of U.S. Dollars") && u != ccyAmount {
			fails = append(fails, fmt.Sprintf("%s: recorded %q, table says %q", id, units, u))
		}
	}
	return fails
}

// ccyTextMatchesShift holds the two full texts a ccy_amount row may record.
func ccyTextMatchesShift(units string, shift int) bool {
	return (units == "Millions of U.S. Dollars" && shift == 6) ||
		(units == "Billions of U.S. Dollars" && shift == 9)
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
	table := map[string]unitRow{}
	for k, v := range seriesUnits {
		table[k] = v
	}
	table["CPIAUCSL"] = unitRow{unit: contracts.SeriesUnit("percent")}
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
	table := map[string]unitRow{"DGS10": {unit: indexPoints}}
	fails := compareUnits(table, loadMeta(t))
	if len(fails) != 2 {
		t.Errorf("fails = %v", fails)
	}
}

func TestShiftDecimalText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in    string
		shift int
		want  float64
	}{
		{"0.123", 9, 123000000},
		{"6700000", 6, 6.7e12},
		{"-1.5", 6, -1500000},
		{"1.5", 0, 1.5},
	} {
		body := []byte(`{"observations":[{"realtime_start":"2026-11-06","realtime_end":"2026-11-06","date":"2026-11-04","value":"` + c.in + `"}]}`)
		id := map[int]string{6: "WALCL", 9: "RRPONTSYD", 0: "DFF"}[c.shift]
		got, err := mapObservations(id, body, time.Time{})
		if err != nil || len(got) != 1 || got[0].Value == nil || *got[0].Value != c.want {
			t.Errorf("%s shift %d: %+v %v, want %v", c.in, c.shift, got, err, c.want)
		}
	}
	dot := []byte(`{"observations":[{"realtime_start":"2026-11-06","realtime_end":"2026-11-06","date":"2026-11-04","value":"."}]}`)
	got, err := mapObservations("WALCL", dot, time.Time{})
	if err != nil || len(got) != 1 || got[0].Value != nil || got[0].Status != "no_value" {
		t.Errorf("dot: %+v %v", got, err)
	}
}

func TestCcyAmountRowsCarryUSD(t *testing.T) {
	t.Parallel()
	body := []byte(`{"observations":[{"realtime_start":"2026-11-06","realtime_end":"2026-11-06","date":"2026-11-04","value":"6700000"}]}`)
	got, err := mapObservations("WALCL", body, time.Time{})
	if err != nil || len(got) != 1 || got[0].Unit != ccyAmount || got[0].Currency != "USD" {
		t.Errorf("WALCL: %+v %v", got, err)
	}
	got, err = mapObservations("DGS10", body, time.Time{})
	if err != nil || len(got) != 1 || got[0].Currency != "" {
		t.Errorf("DGS10: %+v %v", got, err)
	}
}

func TestUnitsTextMatchesShift(t *testing.T) {
	t.Parallel()
	meta := loadMeta(t)
	if fails := compareUnits(seriesUnits, meta); len(fails) != 0 {
		t.Errorf("table fails: %v", fails)
	}
	table := map[string]unitRow{}
	for k, v := range seriesUnits {
		table[k] = v
	}
	row := table["WALCL"]
	row.shift = 9
	table["WALCL"] = row
	if fails := compareUnits(table, meta); len(fails) != 1 || !strings.Contains(fails[0], "WALCL") {
		t.Errorf("WALCL shift 9: %v", fails)
	}
	table["WALCL"] = seriesUnits["WALCL"]
	altered := append([]seriesMeta(nil), meta...)
	for i := range altered {
		if altered[i].SeriesID == "RRPONTSYD" {
			altered[i].Units = "Millions of U.S. Dollars"
		}
	}
	if fails := compareUnits(table, altered); len(fails) != 1 || !strings.Contains(fails[0], "RRPONTSYD") {
		t.Errorf("RRPONTSYD millions: %v", fails)
	}
}

func TestSeriesUnitsEveryConfiguredIDRecorded(t *testing.T) {
	t.Parallel()
	recorded := map[string]bool{}
	for _, m := range loadMeta(t) {
		recorded[m.SeriesID] = true
	}
	for id := range seriesUnits {
		if !recorded[id] {
			t.Errorf("%s has no recorded text", id)
		}
	}
	table := map[string]unitRow{"NEWID": {unit: contracts.SeriesUnit("percent")}}
	if fails := compareUnits(table, loadMeta(t)); len(fails) != 1 || !strings.Contains(fails[0], "NEWID") {
		t.Errorf("unrecorded id not detected: %v", fails)
	}
}

func TestPinnedVintageSameKeyAcrossFetchDays(t *testing.T) {
	t.Parallel()
	body := func(day string) []byte {
		return []byte(`{"observations":[{"realtime_start":"` + day + `","realtime_end":"` + day + `","date":"2026-11-04","value":"6700000"}]}`)
	}
	type key struct {
		id   string
		date time.Time
		rs   time.Time
	}
	keyOf := func(id, fetchDay string) key {
		got, err := mapObservations(id, body(fetchDay), time.Time{})
		if err != nil || len(got) != 1 {
			t.Fatalf("%s: %v %v", id, got, err)
		}
		return key{got[0].SeriesID, got[0].ObservationDate.Time, got[0].RealtimeStart.Time}
	}
	if a, b := keyOf("WALCL", "2026-11-06"), keyOf("WALCL", "2026-11-07"); a != b {
		t.Errorf("WALCL keys differ: %v %v", a, b)
	}
	if a, b := keyOf("DGS10", "2026-11-06"), keyOf("DGS10", "2026-11-07"); a == b {
		t.Errorf("DGS10 keys equal: %v", a)
	}
	got, _ := mapObservations("WALCL", body("2026-11-07"), time.Time{})
	if !got[0].RealtimeEnd.Equal(day("2026-11-04").Time) || got[0].SourceRevision != "2026-11-04" {
		t.Errorf("pin: %+v", got[0])
	}
}

// mapRecorded maps one recorded observation of id (vintage 2026-11-06, observation 2026-11-04).
func mapRecorded(t *testing.T, id, value string) contracts.SeriesPoint {
	t.Helper()
	body := []byte(`{"observations":[{"realtime_start":"2026-11-06","realtime_end":"2026-11-06","date":"2026-11-04","value":"` + value + `"}]}`)
	got, err := mapObservations(id, body, time.Time{})
	if err != nil || len(got) != 1 {
		t.Fatalf("%s: %v %v", id, got, err)
	}
	return got[0]
}

func checkPinned(t *testing.T, p contracts.SeriesPoint, id string, unit contracts.SeriesUnit, ccy string, want float64) {
	t.Helper()
	if p.SeriesID != id || p.Unit != unit || p.Currency != ccy || p.Value == nil || *p.Value != want || p.Status != "present" {
		t.Errorf("%s: %+v", id, p)
	}
	obs := day("2026-11-04").Time
	if !p.RealtimeStart.Equal(obs) || !p.RealtimeEnd.Equal(obs) || p.SourceRevision != "2026-11-04" {
		t.Errorf("%s: vintage not pinned: %+v", id, p)
	}
}

func TestMappingBAMLC0A0CM(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "BAMLC0A0CM", "0.86"), "BAMLC0A0CM", percent, "", 0.86)
}

func TestMappingBAA10Y(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "BAA10Y", "1.72"), "BAA10Y", percent, "", 1.72)
}

func TestMappingSOFR(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "SOFR", "4.05"), "SOFR", percent, "", 4.05)
}

func TestMappingWRESBAL(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "WRESBAL", "2900000"), "WRESBAL", ccyAmount, "USD", 2.9e12)
}

func TestMappingIORB(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "IORB", "4.40"), "IORB", percent, "", 4.4)
}

func TestMappingSOFR99(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "SOFR99", "4.12"), "SOFR99", percent, "", 4.12)
}

func TestMappingNFCI(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "NFCI", "-0.52"), "NFCI", indexPoints, "", -0.52)
}

func TestMappingSTLFSI4(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "STLFSI4", "-0.85"), "STLFSI4", indexPoints, "", -0.85)
}

func TestMappingTLAACBW027SBOG(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "TLAACBW027SBOG", "1250.5"), "TLAACBW027SBOG", ccyAmount, "USD", 1.2505e12)
}

func TestMappingWLCFLPCL(t *testing.T) {
	t.Parallel()
	checkPinned(t, mapRecorded(t, "WLCFLPCL", "5200"), "WLCFLPCL", ccyAmount, "USD", 5.2e9)
}

func TestMapDEXJPUSValueAndMissing(t *testing.T) {
	t.Parallel()
	pub := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"observations":[` +
		`{"realtime_start":"2026-11-10","realtime_end":"2026-11-10","date":"2026-11-06","value":"153.21"},` +
		`{"realtime_start":"2026-11-10","realtime_end":"2026-11-10","date":"2026-11-07","value":"."}]}`)
	got, err := mapObservations("DEXJPUS", body, pub)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if got[0].Unit != jpyPerUsd || got[0].Value == nil || *got[0].Value != 153.21 || got[0].Status != "present" {
		t.Errorf("value point: %+v", got[0])
	}
	if got[1].Unit != jpyPerUsd || got[1].Value != nil || got[1].Status != "no_value" {
		t.Errorf("missing point: %+v", got[1])
	}
}
