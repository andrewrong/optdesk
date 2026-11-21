package spreadlegsapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegs"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegsapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fake keeps the promises of the interface from the other side: one row per structure, pure.
type fake struct{}

func (fake) Spreads(in spreadlegsapi.SpreadIn) spreadlegsapi.SpreadResult {
	var r spreadlegsapi.SpreadResult
	for _, s := range in.Structures {
		r.Rows = append(r.Rows, spreadlegsapi.SpreadFigures{ID: s.ID})
	}
	return r
}

func TestSpreadContractBothSides(t *testing.T) {
	t.Parallel()
	in := spreadlegsapi.SpreadIn{
		Structures: []compute.Structure{{ID: "a:1", Currency: "USD"}},
		AsOf:       contracts.Day("2026-11-21"),
	}
	for name, mk := range map[string]spreadlegsapi.NewSpreadFunc{
		"fake": func() spreadlegsapi.SpreadFigure { return fake{} },
		"real": spreadlegs.New,
	} {
		rows := mk().Spreads(in).Rows
		if len(rows) != 1 || rows[0].ID != "a:1" {
			t.Fatalf("%s: rows %+v", name, rows)
		}
	}
}
