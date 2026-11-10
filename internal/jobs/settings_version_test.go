package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/jobs"
)

// TestSettingsVersionFuncContract lives here, not in api_test.go: that file is exactly 800 lines on main and an
// addition would put it over the size ratchet, whose file an agent does not edit.
func TestSettingsVersionFuncContract(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC)
	var got time.Time
	var f jobs.SettingsVersionFunc = func(_ context.Context, a time.Time) (int64, error) {
		got = a
		return 5, nil
	}
	v, err := f(context.Background(), at)
	if err != nil || v != 5 || !got.Equal(at) {
		t.Fatalf("version %d, err %v, at %v", v, err, got)
	}
}
