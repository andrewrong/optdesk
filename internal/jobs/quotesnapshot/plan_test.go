package quotesnapshot

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const minimal = "id = \"quote_snapshot\"\n[[instrument]]\nsymbol = \"SPY\"\ncon_id = 1\ncurrency = \"USD\"\n"

func TestPlan(t *testing.T) {
	t.Parallel()
	j, err := New(&fakeBroker{}, &fakeResolver{}, &fakeWriter{}, fakeClockAt(t, "2026-11-05 09:00"))
	if err != nil {
		t.Fatal(err)
	}
	day := contracts.Day("2026-11-05")
	keys, _, err := j.Plan(context.Background(), day)
	if err != nil {
		t.Fatal(err)
	}
	var want []contracts.ExpectedKey
	for _, s := range []string{"0945", "1230", "1530"} {
		want = append(want, contracts.ExpectedKey{DatasetID: "obs.quote_snapshot", SubjectKey: s})
	}
	if len(keys) != 3 || keys[0] != want[0] || keys[1] != want[1] || keys[2] != want[2] {
		t.Fatalf("keys = %v", keys)
	}
	// A Saturday has the same keys: the job has no calendar.
	sat, _, _ := j.Plan(context.Background(), "2026-11-07")
	if len(sat) != 3 {
		t.Fatalf("saturday keys = %v", sat)
	}

	hash := func(data, d string) string {
		m, err := parseManifest([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		_, h, _ := m.plan(contracts.Day(d))
		return h
	}
	base := hash(minimal, "2026-11-05")
	if base != hash(minimal, "2026-11-05") {
		t.Fatal("same input, two hashes")
	}
	if base == hash(minimal, "2026-11-06") {
		t.Fatal("day did not change the hash")
	}
	if base == hash("slots = [\"1000\"]\n"+minimal, "2026-11-05") {
		t.Fatal("slots did not change the hash")
	}
	if base == hash(minimal+"[[instrument]]\nsymbol = \"QQQ\"\ncon_id = 2\ncurrency = \"USD\"\n", "2026-11-05") {
		t.Fatal("instruments did not change the hash")
	}
}

func TestManifestDefaults(t *testing.T) {
	t.Parallel()
	m, err := parseManifest([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}
	if m.Zone != "America/New_York" || m.chunk() != 50 || m.budget() != 8*time.Minute || m.grace() != 10*time.Minute {
		t.Fatalf("defaults: %+v", m)
	}
	if got := m.Slots; len(got) != 3 || got[0] != "0945" || got[1] != "1230" || got[2] != "1530" {
		t.Fatalf("slots = %v", got)
	}
	j, err := build([]byte(minimal), &fakeBroker{}, &fakeResolver{}, &fakeWriter{}, fakeClockAt(t, "2026-11-05 09:00"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, closeOff := j.Window(); closeOff != 4*time.Hour {
		t.Fatalf("lateness = %v", closeOff)
	}
	// the embedded manifest states the same slots and lateness
	e, err := New(&fakeBroker{}, &fakeResolver{}, &fakeWriter{}, fakeClockAt(t, "2026-11-05 09:00"))
	if err != nil {
		t.Fatal(err)
	}
	cal, open, closeOff := e.(*job).Window()
	if cal != "XNYS" || open != 0 || closeOff != 4*time.Hour {
		t.Fatalf("window = %q %v %v", cal, open, closeOff)
	}
}

func TestManifestRejects(t *testing.T) {
	t.Parallel()
	for name, data := range map[string]string{
		"no instrument": "id = \"x\"\n",
		"bad slot":      "slots = [\"25\"]\n" + minimal,
		"bad zone":      "zone = \"Nowhere/Land\"\n" + minimal,
		"zero chunk":    "chunk = 0\n" + minimal,
	} {
		if _, err := parseManifest([]byte(data)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
