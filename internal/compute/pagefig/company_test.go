package pagefig

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func TestGlanceCountsEveryState(t *testing.T) {
	t.Parallel()
	rows := []blockapi.CompanyState{
		{Symbol: "A", State: "band"}, {Symbol: "B", State: "band"}, {Symbol: "C", State: "expired"},
		{Symbol: "D", State: "unvalued"}, {Symbol: "E", State: "owner override"},
	}
	got := New().Glance(rows)
	want := map[string]string{"band": "2", "owner override": "1", "awaiting approval": "0", "expired": "1", "unvalued": "1"}
	if len(got) != len(want) {
		t.Fatalf("lines: %+v", got)
	}
	for _, l := range got {
		if l.Amount != (compute.FigureValue{Value: want[l.Name]}) {
			t.Errorf("%s = %+v, want %s", l.Name, l.Amount, want[l.Name])
		}
	}
}

func TestUnchangedRowIsSilent(t *testing.T) {
	t.Parallel()
	rows := []blockapi.CompanyState{
		{Symbol: "A", State: "band", BandToday: "低估", BandThen: "低估"},
		{Symbol: "B", State: "band", BandToday: "合理", BandThen: "低估"},
		{Symbol: "C", State: "band", BandToday: "合理"},
	}
	got := New().Changes(rows)
	if len(got) != 1 || got[0].Symbol != "B" || got[0].Text != "低估 → 合理" {
		t.Fatalf("changes: %+v", got)
	}
}
