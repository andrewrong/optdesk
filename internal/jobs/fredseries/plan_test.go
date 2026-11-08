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
	want := []string{"DGS10", "DGS3MO", "SP500", "VIXCLS", "VXNCLS", "RVXCLS", "VXVCLS"}
	m, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 1 || m.ID != "fred_series" || !slices.Equal(m.Series, want) ||
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
