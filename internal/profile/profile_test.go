package profile_test

import (
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/profile"
)

var _ profile.LoadFunc = profile.Load

// env returns a getenv over a map that fails the test on any other variable.
func env(t *testing.T, m map[string]string) func(string) string {
	t.Helper()
	return func(k string) string {
		if k != "OT_HOME" && k != "OT_PROFILE" && k != "OT_ZONE" {
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
		{"home", map[string]string{"OT_HOME": t.TempDir(), "OT_ZONE": "UTC"}, "real", false},
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

func TestLoadZone(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	cases := []struct {
		name     string
		env      map[string]string
		wantZone string
		errHas   string
		secret   string
	}{
		{"demo", nil, "UTC", "", ""},
		{"demo by profile", map[string]string{"OT_HOME": home, "OT_PROFILE": "demo"}, "UTC", "", ""},
		{"real UTC", map[string]string{"OT_HOME": home, "OT_ZONE": "UTC"}, "UTC", "", ""},
		{"real unset", map[string]string{"OT_HOME": home}, "", "OT_ZONE", ""},
		{"real empty", map[string]string{"OT_HOME": home, "OT_ZONE": ""}, "", "OT_ZONE", ""},
		{"real Local", map[string]string{"OT_HOME": home, "OT_ZONE": "Local"}, "", "OT_ZONE", "Local"},
		{"real unknown", map[string]string{"OT_HOME": home, "OT_ZONE": "no/such/zone"}, "", "OT_ZONE", "no/such/zone"},
		{"real needs home", map[string]string{"OT_PROFILE": "real", "OT_ZONE": "UTC"}, "", "OT_PROFILE=real needs OT_HOME", ""},
		{"bad profile", map[string]string{"OT_PROFILE": "x", "OT_ZONE": "UTC"}, "", "OT_PROFILE", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var asked []string
			getenv := func(k string) string {
				asked = append(asked, k)
				return tc.env[k]
			}
			p, err := profile.Load(getenv)
			if tc.errHas != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errHas) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.errHas)
				}
				if tc.secret != "" && strings.Contains(err.Error(), tc.secret) {
					t.Fatalf("err %q contains the value %q", err, tc.secret)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Zone == nil || p.Zone.String() != tc.wantZone {
				t.Fatalf("zone = %v, want %s", p.Zone, tc.wantZone)
			}
			if p.Name == "demo" {
				if p.Zone != time.UTC {
					t.Fatalf("demo zone is not time.UTC")
				}
				for _, k := range asked {
					if k == "OT_ZONE" {
						t.Fatal("demo asked for OT_ZONE")
					}
				}
			}
		})
	}
}
