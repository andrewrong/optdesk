package optionresult

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func withBroker(p compute.LedgerPosition, b string) compute.LedgerPosition {
	p.BrokerRealized = b
	return p
}

func TestReliabilityEqualDiffer(t *testing.T) {
	t.Parallel()
	got := summarise(
		withBroker(pos("1", "closed", "USD", "-1", "2", "10.0000", "2026-01-20"), "10"),
		withBroker(pos("2", "closed", "USD", "-1", "2", "5", "2026-01-20"), "5.0000"),
		withBroker(pos("3", "closed", "USD", "-1", "2", "7", "2026-01-20"), "7.01"),
		withBroker(pos("4", "closed", "USD", "-1", "2", "7", "2026-01-20"), "n/a"),
	).Sections[0].Reliability
	if got.Equal != 2 || got.Differ != 2 || got.NoBroker != 0 {
		t.Fatalf("reliability %+v", got)
	}
}

func TestNoBrokerFigureCounted(t *testing.T) {
	t.Parallel()
	got := summarise(
		pos("1", "closed", "USD", "-1", "2", "10", "2026-01-20"),
		withBroker(pos("2", "closed", "USD", "-1", "2", "5", "2026-01-20"), "5"),
	).Sections[0]
	if r := got.Reliability; r.NoBroker != 1 || r.Equal != 1 || r.Differ != 0 {
		t.Fatalf("reliability %+v", r)
	}
	if got.Realized != "15.0000" {
		t.Fatalf("realized %q", got.Realized)
	}
}

func TestUnresolvedAndReviewListed(t *testing.T) {
	t.Parallel()
	in := compute.ResultInput{
		Positions:  []compute.LedgerPosition{pos("1", "closed", "USD", "-1", "2", "10", "2026-01-20")},
		Unresolved: []string{"u1", "u2"}, Review: []string{"r1"}, AsOf: "2026-02-01",
	}
	got := New().Summarise(in).Sections[0]
	if !reflect.DeepEqual(got.Reliability.Unresolved, []string{"u1", "u2"}) || !reflect.DeepEqual(got.Reliability.Review, []string{"r1"}) {
		t.Fatalf("reliability %+v", got.Reliability)
	}
	if got.Realized != "10.0000" || got.Reliability.NoBroker != 1 {
		t.Fatalf("ids entered a figure: %+v", got)
	}
}
