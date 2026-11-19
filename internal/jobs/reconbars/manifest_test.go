package reconbars_test

import (
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/jobs/reconbars"
)

func TestManifestLoads(t *testing.T) {
	t.Parallel()
	m, err := reconbars.DefaultManifest()
	if err != nil {
		t.Fatal(err)
	}
	s := m.Schedule
	if m.ID != "recon_bars" || m.SourceID != "yfinance" || s.Calendar != "XNYS" || s.Anchor != "session_close" ||
		s.RunBound != 30*time.Minute {
		t.Fatalf("manifest %+v", m)
	}
}

func TestSourceIdIsData(t *testing.T) {
	t.Parallel()
	m, err := reconbars.LoadManifest([]byte(`
id = "recon_bars"
[schedule]
calendar = "XNYS"
run_bound = "30m"
[source]
id = "tiingo"
`))
	if err != nil {
		t.Fatal(err)
	}
	if m.SourceID != "tiingo" {
		t.Fatalf("source id %q", m.SourceID)
	}
}

func TestManifestRefusesBadValues(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ toml, key string }{
		"no source":      {"id = \"x\"\n[schedule]\ncalendar = \"XNYS\"\nrun_bound = \"30m\"\n", "source.id"},
		"short bound":    {"id = \"x\"\n[schedule]\ncalendar = \"XNYS\"\nrun_bound = \"2m\"\n[source]\nid = \"a\"\n", "schedule.run_bound"},
		"no calendar":    {"id = \"x\"\n[source]\nid = \"a\"\n", "schedule.calendar"},
		"bad bound text": {"id = \"x\"\n[schedule]\ncalendar = \"XNYS\"\nrun_bound = \"secret\"\n[source]\nid = \"a\"\n", "schedule.run_bound"},
	} {
		_, err := reconbars.LoadManifest([]byte(c.toml))
		if err == nil || !strings.Contains(err.Error(), c.key) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("%s: error %v", name, err)
		}
	}
}
