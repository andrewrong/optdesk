package companydata_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
)

const (
	ocfC   = "NetCashProvidedByUsedInOperatingActivities"
	capexC = "PaymentsToAcquirePropertyPlantAndEquipment"
)

func TestFreeCashIsOCFMinusCapex(t *testing.T) {
	t.Parallel()
	q := byNumber(t, must(t, facts3m("120", "45")), 1)
	wantCell(t, "free cash", q.FreeCash, "75", true)
	if q.FreeCash.Concept != ocfC+" - "+capexC {
		t.Fatalf("concept = %q", q.FreeCash.Concept)
	}
}

func TestFreeCashMissingWhenEitherMissing(t *testing.T) {
	t.Parallel()
	only := facts3m("120", "45")
	wantMissing(t, "no capex", byNumber(t, must(t, only[:1]), 1).FreeCash, companydataapi.WhyNoFact)
	wantMissing(t, "no ocf", byNumber(t, must(t, only[1:]), 1).FreeCash, companydataapi.WhyNoFact)
}

func TestFreeCashNegativeAndExact(t *testing.T) {
	t.Parallel()
	wantCell(t, "free cash", byNumber(t, must(t, facts3m("10.10", "30.30")), 1).FreeCash, "-20.20", true)
}
