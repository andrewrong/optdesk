package barsrefresh_test

import (
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
)

func raw(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("job.toml")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestManifestLoads(t *testing.T) {
	t.Parallel()
	m, err := barsrefresh.DefaultManifest()
	if err != nil {
		t.Fatal(err)
	}
	got := []int{
		m.Version, m.BackfillDays, m.DailyDays, m.AdjustedCadenceDays, m.BackfillNamesPerRun,
		m.DeepenDays, m.DeepenPieceDays, m.DeepenNamesPerRun,
	}
	want := []int{1, 400, 5, 7, 10, 1825, 730, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("values %v, want %v", got, want)
		}
	}
	if m.ID != "bars_refresh" || m.RestateTolerance.Cmp(mustRat(t, "1/100000")) != 0 {
		t.Fatalf("id %q tolerance %v", m.ID, m.RestateTolerance)
	}
	checkIndexes(t, m.Indexes)
	s := m.Schedule
	if s.Calendar != "XNYS" || s.Anchor != "session_close" || s.Offset != 10*time.Minute || s.RunBound != 10*time.Minute {
		t.Fatalf("schedule %+v", s)
	}
}

func checkIndexes(t *testing.T, ixs []barsrefresh.Index) {
	t.Helper()
	var syms []string
	for _, ix := range ixs {
		syms = append(syms, ix.Symbol)
		if ix.ConID != 0 || ix.Exchange == "" || ix.Currency == "" || ix.SecType == "" {
			t.Fatalf("index %+v", ix)
		}
	}
	if strings.Join(syms, ",") != "SPX,NDX,RUT,N225" {
		t.Fatalf("indexes %v", syms)
	}
}

func TestManifestRefusesBadDays(t *testing.T) {
	t.Parallel()
	for _, key := range []string{
		"backfill_days", "daily_days", "adjusted_cadence_days",
		"backfill_names_per_run", "deepen_days", "deepen_piece_days",
	} {
		src := raw(t)
		lines := strings.Split(src, "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, key+" =") {
				lines[i] = key + " = 0"
			}
		}
		_, err := barsrefresh.LoadManifest([]byte(strings.Join(lines, "\n")))
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("%s: got %v", key, err)
		}
	}
}

func TestManifestRefusesBadTolerance(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"abc", "1/3"} {
		src := strings.Replace(raw(t), `"0.00001"`, `"`+v+`"`, 1)
		_, err := barsrefresh.LoadManifest([]byte(src))
		if err == nil || !strings.Contains(err.Error(), "restate_tolerance") {
			t.Fatalf("%s: got %v", v, err)
		}
		if strings.Contains(err.Error(), v) {
			t.Fatalf("the error shows the value: %v", err)
		}
	}
}

func TestManifestRefusesIncompleteIndex(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"symbol", "exchange", "currency"} {
		src := raw(t)
		lines := strings.Split(src, "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, key+" =") {
				lines = append(lines[:i], lines[i+1:]...)
				break
			}
		}
		_, err := barsrefresh.LoadManifest([]byte(strings.Join(lines, "\n")))
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("%s: got %v", key, err)
		}
	}
}

func TestManifestRefusesNoCalendar(t *testing.T) {
	t.Parallel()
	src := strings.Replace(raw(t), `calendar = "XNYS"`, "", 1)
	if _, err := barsrefresh.LoadManifest([]byte(src)); err == nil || !strings.Contains(err.Error(), "calendar") {
		t.Fatalf("got %v", err)
	}
}

func TestManifestDeepenZeroIsNoCap(t *testing.T) {
	t.Parallel()
	if _, err := barsrefresh.LoadManifest([]byte(raw(t))); err != nil {
		t.Fatalf("0 is no cap: %v", err)
	}
	src := strings.Replace(raw(t), "deepen_names_per_run = 0", "deepen_names_per_run = -1", 1)
	_, err := barsrefresh.LoadManifest([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "deepen_names_per_run") {
		t.Fatalf("got %v", err)
	}
}

func mustRat(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatal(s)
	}
	return r
}
