package job_console

import (
	"fmt"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

func TestAlreadyPresentRequestIsNotNothingProduced(t *testing.T) {
	t.Parallel()
	nothingLine := fmt.Sprintf(zhNothingProducedFmt, 7, "compute_figures", "2026-11-12")
	presentLine := fmt.Sprintf(zhAllPresentFmt, 7, "compute_figures", "2026-11-12")
	got := one(t, rowReason(7, jobrequest.StateDone, "completed", line("already_present", "done"))).Sentence
	if !strings.Contains(got, presentLine) || strings.Contains(got, "没有产出") {
		t.Errorf("sentence %q", got)
	}
	got = one(t, rowReason(7, jobrequest.StateDone, "completed", line("ran", "done"))).Sentence
	if !strings.Contains(got, nothingLine) || strings.Contains(got, presentLine) {
		t.Errorf("sentence %q", got)
	}
}
