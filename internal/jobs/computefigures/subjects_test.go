package computefigures

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestSubjectsNamedBySymbol(t *testing.T) {
	t.Parallel()
	f := world()
	f.latest = []store.OptionEodLatest{{UnderlyingID: 1, SessionDay: dayD}, {UnderlyingID: 2, SessionDay: dayD},
		{UnderlyingID: 3, SessionDay: "2026-11-09"}}
	f.eod[2] = []contracts.OptionEod{eodRow(200, fp(1), fp(2), nil, closeD)}
	f.eod[2][0].UnderlyingID = 2
	f.eod[3] = nil // only later rows: dropped
	got, err := subjects(context.Background(), f.stores(), dayD)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].name != "SPY" || got[0].id != 1 || got[1].name != "underlying:2" || got[1].kind != "underlying" {
		t.Fatalf("subjects %+v, want SPY and underlying:2", got)
	}
}
