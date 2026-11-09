package sources_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/sources"
)

type sectionFlexSource struct{}

var _ sources.FlexSource = sectionFlexSource{}

func (sectionFlexSource) Fetch(context.Context, sources.FlexRequest) (sources.FlexReport, error) {
	return sources.FlexReport{Statements: []sources.FlexStatement{{
		AccountID: "ACCT",
		Sections: map[string][]map[string]string{
			"CashReportCurrency": {{"currency": "USD", "endingCash": "100.5"}, {"currency": "JPY", "endingCash": "7"}},
			"ChangeInNAV":        {{"endingValue": "1000", "accountId": "ACCT"}},
		},
	}}}, nil
}

func TestFlexStatementSectionsContract(t *testing.T) {
	t.Parallel()
	var src sources.FlexSource = sectionFlexSource{}
	rep, err := src.Fetch(context.Background(), sources.FlexRequest{})
	if err != nil {
		t.Fatal(err)
	}
	sec := rep.Statements[0].Sections
	if got := len(sec["CashReportCurrency"]); got != 2 {
		t.Fatalf("CashReportCurrency rows = %d, want 2", got)
	}
	if got := sec["CashReportCurrency"][1]["endingCash"]; got != "7" {
		t.Fatalf("endingCash = %q, want 7", got)
	}
	if got := sec["ChangeInNAV"][0]["endingValue"]; got != "1000" {
		t.Fatalf("endingValue = %q, want 1000", got)
	}
	if rep.Statements[0].Trades != nil || sec["Missing"] != nil {
		t.Fatal("absent sections and trades must stay nil")
	}
}
