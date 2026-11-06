package profile_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/profile"
)

var _ profile.LoadFunc = profile.Load

// env returns a getenv over a map that fails the test on any other variable.
func env(t *testing.T, m map[string]string) func(string) string {
	t.Helper()
	return func(k string) string {
		if k != "OT_HOME" && k != "OT_PROFILE" {
			t.Errorf("Load read %s", k)
		}
		return m[k]
	}
}

func TestLoadSelectsProfile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		env  map[string]string
		want string
		fail bool
	}{
		{"no home", nil, "demo", false},
		{"no home, demo", map[string]string{"OT_PROFILE": "demo"}, "demo", false},
		{"no home, real", map[string]string{"OT_PROFILE": "real"}, "", true},
		{"home", map[string]string{"OT_HOME": t.TempDir()}, "real", false},
		{"home, demo", map[string]string{"OT_HOME": t.TempDir(), "OT_PROFILE": "demo"}, "demo", false},
		{"unknown", map[string]string{"OT_PROFILE": "x"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := profile.Load(env(t, tc.env))
			if (err != nil) != tc.fail || p.Name != tc.want {
				t.Fatalf("got %q, %v; want %q, fail=%v", p.Name, err, tc.want, tc.fail)
			}
			if !tc.fail && len(p.LogFields) == 0 {
				t.Fatal("no log fields")
			}
		})
	}
}
