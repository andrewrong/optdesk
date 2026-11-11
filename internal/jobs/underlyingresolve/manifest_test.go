package underlyingresolve

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestManifestLoads(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("job.toml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(b)
	if err != nil {
		t.Fatal(err)
	}
	s := m.Schedule
	if m.ID != "underlying_resolve" || m.Version != 1 || m.RetryDays != 7 || m.MaxNames != 20 ||
		s.Calendar != "XNYS" || s.Anchor != "session_close" || s.Offset != 45*time.Minute || s.RunBound != 10*time.Minute {
		t.Fatalf("manifest %+v", m)
	}
}

func TestManifestRefusesBelowOne(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("job.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ from, to, key string }{
		{"retry_days = 7", "retry_days = 0", "retry_days"},
		{"max_names = 20", "max_names = 0", "max_names"},
		{"max_names = 20", "max_names = -3", "max_names"},
	} {
		_, err := LoadManifest([]byte(strings.Replace(string(b), c.from, c.to, 1)))
		if err == nil || !strings.Contains(err.Error(), c.key) {
			t.Errorf("%s: err %v", c.to, err)
		}
	}
}
