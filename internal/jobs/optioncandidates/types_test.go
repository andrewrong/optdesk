package optioncandidates_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

func TestOptionCandidatesJobFuncContract(t *testing.T) {
	t.Parallel()
	var f optioncandidates.NewOptionCandidatesJobFunc = func(
		st optioncandidates.CandidateStores, r compute.OptionCandidates, w []jobs.WatchEntry, c clock.Clock,
	) (contracts.Job, error) {
		if st.Earnings != nil || st.Accounts != nil || st.Series != nil || r != nil || len(w) != 1 || c != nil {
			t.Fatal("arguments did not arrive as given")
		}
		return nil, nil
	}
	if _, err := f(optioncandidates.CandidateStores{}, nil, []jobs.WatchEntry{{Issuer: "cik:0000000001"}}, nil); err != nil {
		t.Fatal(err)
	}
}

type noDividends struct{}

func (noDividends) Dividends(context.Context, int, contracts.Day, contracts.Day) ([]contracts.DividendExpectation, error) {
	return nil, nil
}

func TestOptionCandidatesCallsFuncContract(t *testing.T) {
	t.Parallel()
	var f optioncandidates.NewOptionCandidatesCallsFunc = func(
		st optioncandidates.CandidateStores, d dividend.Reader, r compute.OptionCandidates, w []jobs.WatchEntry, c clock.Clock,
	) (contracts.Job, error) {
		if st.Earnings != nil || d == nil || r != nil || len(w) != 1 || c != nil {
			t.Fatal("arguments did not arrive in the order given")
		}
		return nil, nil
	}
	if _, err := f(optioncandidates.CandidateStores{}, noDividends{}, nil, []jobs.WatchEntry{{Issuer: "cik:0000000001"}}, nil); err != nil {
		t.Fatal(err)
	}
}

// TestSettingsVersionFuncContract lives here, not in api_test.go: that file is exactly 800 lines on main and an
// addition would put it over the size ratchet, whose file an agent does not edit.
func TestSettingsVersionFuncContract(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC)
	var got time.Time
	var f optioncandidates.SettingsVersionFunc = func(_ context.Context, a time.Time) (int64, error) {
		got = a
		return 5, nil
	}
	v, err := f(context.Background(), at)
	if err != nil || v != 5 || !got.Equal(at) {
		t.Fatalf("version %d, err %v, at %v", v, err, got)
	}
}
