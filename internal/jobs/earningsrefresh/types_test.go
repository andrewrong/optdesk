package earningsrefresh_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

type fakeJob struct{ id string }

func (j fakeJob) ID() string { return j.id }
func (fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (fakeJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}

type fakeTranslator struct{}

func (fakeTranslator) ToVendor(s string, _ symbol.Vendor) (string, error) { return s, nil }
func (fakeTranslator) OSIRoot(s string) string                            { return s }

// TestSymbolCtorTypes: the constructor carries the translator as the last argument, and a nil translator
// is accepted (no translation).
func TestSymbolCtorTypes(t *testing.T) {
	t.Parallel()
	var got []symbol.Translator
	var earn earningsrefresh.NewEarningsRefreshSymbolsFunc = func(
		_ map[string]sources.EarningsSource, _ store.EarningsDateWriter, _ store.EarningsDateReader, _ []jobs.WatchEntry, _ clock.Clock, tr symbol.Translator,
	) (contracts.Job, error) {
		got = append(got, tr)
		return fakeJob{id: "earnings_refresh"}, nil
	}
	for _, tr := range []symbol.Translator{fakeTranslator{}, nil} {
		if j, err := earn(nil, nil, nil, nil, nil, tr); err != nil || j.ID() != "earnings_refresh" {
			t.Fatalf("earnings_refresh: %v %v", j, err)
		}
	}
	if len(got) != 2 || got[0] != (fakeTranslator{}) || got[1] != nil {
		t.Fatalf("translators passed through: %v", got)
	}
}
