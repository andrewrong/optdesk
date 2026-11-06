package profile

import (
	"fmt"
	"time"
)

// logFields is the whitelist of log attribute keys, the same for both profiles.
var logFields = []string{
	"time", "level", "msg", "component", "job", "dataset", "run_id", "attempt",
	"status", "rows", "duration_ms", "err", "profile", "version",
}

// Load implements LoadFunc. OT_HOME unset selects the demo profile and no real value
// is read; with OT_HOME set the profile is real unless OT_PROFILE says demo. It reads
// OT_HOME and OT_PROFILE, and OT_ZONE for the real profile only, which must
// name an IANA zone: a missing, empty or invalid value, or Local, is an error.
func Load(getenv func(string) string) (Profile, error) {
	home := getenv("OT_HOME")
	name := getenv("OT_PROFILE")
	switch {
	case name != "" && name != "demo" && name != "real":
		return Profile{}, fmt.Errorf("profile: OT_PROFILE %q is neither demo nor real", name)
	case home == "" && name == "real":
		return Profile{}, fmt.Errorf("profile: OT_PROFILE=real needs OT_HOME")
	case home == "" || name == "demo":
		name = "demo"
	default:
		name = "real"
	}
	zone := time.UTC
	if name == "real" {
		key := getenv("OT_ZONE")
		if key == "" || key == "Local" {
			return Profile{}, fmt.Errorf("profile: OT_ZONE must name an IANA zone")
		}
		loc, err := time.LoadLocation(key)
		if err != nil {
			return Profile{}, fmt.Errorf("profile: OT_ZONE is not a known zone")
		}
		zone = loc
	}
	return Profile{Name: name, Zone: zone, LogFields: append([]string(nil), logFields...)}, nil
}
