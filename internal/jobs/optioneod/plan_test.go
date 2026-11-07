package optioneod

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const minimalToml = `
id = "option_eod"
version = 1
[[underlying]]
symbol = "TEST"
con_id = 1
currency = "USD"
`

const twoToml = `
id = "option_eod"
version = 1
[[underlying]]
symbol = "ZZZ"
con_id = 2
currency = "USD"
[[underlying]]
symbol = "AAA"
con_id = 1
currency = "USD"
`

func TestManifestDefaults(t *testing.T) {
	t.Parallel()
	m, err := parseManifest([]byte(minimalToml))
	if err != nil {
		t.Fatal(err)
	}
	if m.Zone != "America/New_York" || m.MaxExpiries != 3 || m.StrikeBand != 0.10 || m.MaxStrikesPerSide != 8 ||
		m.Chunk != 50 || m.BudgetMinutes != 8 || m.FreezeAfter != "1615" || m.OptionPriceIncrement != "0.01" {
		t.Fatalf("defaults wrong: %+v", m)
	}
	cal, open, closeOff, err := windowOf(m)
	if err != nil || cal != "" || open != 0 || closeOff != 5*time.Hour {
		t.Fatalf("window = %q %v %v %v, want lateness 5h", cal, open, closeOff, err)
	}
	if _, err := build(manifestBytes, nil, nil, nil, nil); err != nil {
		t.Fatalf("embedded manifest: %v", err)
	}
}

func TestManifestErrors(t *testing.T) {
	t.Parallel()
	for name, data := range map[string]string{
		"no underlying": "id = \"x\"\n",
		"bad freeze":    minimalToml + "\n",
		"bad rule":      "id = \"x\"\n[window]\nrule = \"other\"\n[[underlying]]\nsymbol=\"A\"\ncon_id=1\ncurrency=\"USD\"\n",
	} {
		if name == "bad freeze" {
			data = "freeze_after = \"9999\"\n" + data
		}
		m, err := parseManifest([]byte(data))
		if err == nil {
			_, _, _, err = windowOf(m)
		}
		if err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestPlan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	day := contracts.Day("2026-11-06")
	keys, hash, err := planFrom([]byte(twoToml), day)
	if err != nil {
		t.Fatal(err)
	}
	want := []contracts.ExpectedKey{{DatasetID: "obs.option_eod", SubjectKey: "AAA"}, {DatasetID: "obs.option_eod", SubjectKey: "ZZZ"}}
	if len(keys) != 2 || keys[0] != want[0] || keys[1] != want[1] {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	_, again, _ := planFrom([]byte(twoToml), day)
	_, otherDay, _ := planFrom([]byte(twoToml), "2026-11-09")
	_, otherManifest, _ := planFrom([]byte(twoToml+"\n"), day)
	changed := []byte(twoToml + "")
	changed = append([]byte("chunk = 10\n"), changed...)
	_, otherSetting, _ := planFrom(changed, day)
	if again != hash || otherDay == hash || otherSetting == hash {
		t.Fatalf("hash: same=%v otherDay=%v otherSetting=%v", again == hash, otherDay == hash, otherSetting == hash)
	}
	if otherManifest != hash {
		t.Fatal("whitespace must not change the hash")
	}
	j, err := build([]byte(twoToml), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	jk, jh, err := j.Plan(ctx, day)
	if err != nil || jh != hash || len(jk) != 2 || j.ID() != "option_eod" {
		t.Fatalf("job plan = %v %q %v", jk, jh, err)
	}
}
