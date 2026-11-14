package fredseries

import (
	"context"
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestPlanHash(t *testing.T) {
	t.Parallel()
	one := []byte("version = 1\nid = \"fred_series\"\nseries = [\"DGS10\"]\n")
	other := []byte("version = 1\nid = \"fred_series\"\nseries = [\"DGS10\", \"DGS2\"]\n")
	day := contracts.Day("2026-11-05")

	keys, h1, err := planFrom(one, day)
	if err != nil {
		t.Fatal(err)
	}
	want := contracts.ExpectedKey{DatasetID: "obs.series_point", SubjectKey: "DGS10"}
	if len(keys) != 1 || keys[0] != want {
		t.Fatalf("keys = %v", keys)
	}
	_, h2, _ := planFrom(one, day)
	if h1 != h2 {
		t.Fatalf("same input, hashes %s and %s", h1, h2)
	}
	_, h3, _ := planFrom(other, day)
	if h1 == h3 {
		t.Fatal("another series list gave the same hash")
	}
}

func TestEmbeddedManifest(t *testing.T) {
	t.Parallel()
	want := []string{"DGS10", "DGS3MO", "SP500", "VIXCLS", "VXNCLS", "RVXCLS", "VXVCLS",
		"WALCL", "WDTGAL", "RRPONTSYD", "DFF", "BAMLH0A0HYM2", "BAMLC0A0CM", "BAA10Y", "SOFR", "WRESBAL", "IORB", "SOFR99", "NFCI", "STLFSI4", "TLAACBW027SBOG", "WLCFLPCL"}
	wantWindowed := []string{"WALCL", "WDTGAL", "RRPONTSYD", "DFF", "BAMLH0A0HYM2", "BAMLC0A0CM", "BAA10Y", "SOFR", "WRESBAL", "IORB", "SOFR99", "NFCI", "STLFSI4", "TLAACBW027SBOG", "WLCFLPCL"}
	m, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 1 || m.ID != "fred_series" || !slices.Equal(m.Series, want) ||
		!slices.Equal(m.Windowed, wantWindowed) || m.LookbackDays != 14 ||
		m.Window.Rule != "after_close" || m.Window.LatenessHours != 96 {
		t.Fatalf("manifest = %+v", m)
	}
	j, err := New(&fakeSource{}, &fakeWriter{}, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-05")
	if err != nil || len(keys) != len(want) {
		t.Fatalf("keys %v err %v", keys, err)
	}
}

func manifestWith(extra string) []byte {
	return []byte("version = 1\nid = \"fred_series\"\nseries = [\"DGS10\", \"WALCL\"]\n" + extra)
}

func TestWindowedIDNotInSeriesRefused(t *testing.T) {
	t.Parallel()
	_, err := parseManifest(manifestWith("windowed = [\"DFF\"]\nlookback_days = 14\n"))
	if err == nil {
		t.Fatal("windowed id outside series accepted")
	}
	if _, err := parseManifest(manifestWith("windowed = [\"WALCL\"]\nlookback_days = 14\n")); err != nil {
		t.Fatal(err)
	}
}

func TestLookbackOutOfRangeRefused(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"0", "32"} {
		if _, err := parseManifest(manifestWith("windowed = [\"WALCL\"]\nlookback_days = " + n + "\n")); err == nil {
			t.Fatalf("lookback_days %s accepted", n)
		}
	}
	for _, n := range []string{"1", "31"} {
		if _, err := parseManifest(manifestWith("windowed = [\"WALCL\"]\nlookback_days = " + n + "\n")); err != nil {
			t.Fatalf("lookback_days %s: %v", n, err)
		}
	}
}

func TestPlanHashCoversWindowed(t *testing.T) {
	t.Parallel()
	day := contracts.Day("2026-11-05")
	hash := func(extra string) string {
		_, h, err := planFrom(manifestWith(extra), day)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := hash("windowed = [\"WALCL\"]\nlookback_days = 14\n")
	if base != hash("windowed = [\"WALCL\"]\nlookback_days = 14\n") {
		t.Fatal("same input, different hashes")
	}
	if base == hash("windowed = [\"WALCL\"]\nlookback_days = 15\n") {
		t.Fatal("another lookback_days gave the same hash")
	}
	if base == hash("windowed = [\"WALCL\", \"DGS10\"]\nlookback_days = 14\n") {
		t.Fatal("another windowed list gave the same hash")
	}
	if hash("windowed = [\"WALCL\", \"DGS10\"]\nlookback_days = 14\n") != hash("windowed = [\"DGS10\", \"WALCL\"]\nlookback_days = 14\n") {
		t.Fatal("windowed order changed the hash")
	}
}
