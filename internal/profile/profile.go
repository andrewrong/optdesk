package profile

import "fmt"

// logFields is the whitelist of log attribute keys, the same for both profiles.
var logFields = []string{
	"time", "level", "msg", "component", "job", "dataset", "run_id", "attempt",
	"status", "rows", "duration_ms", "err", "profile", "version",
}

// Load implements LoadFunc. OT_HOME unset selects the demo profile and no real value
// is read; with OT_HOME set the profile is real unless OT_PROFILE says demo. It reads
// only OT_HOME and OT_PROFILE.
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
	return Profile{Name: name, LogFields: append([]string(nil), logFields...)}, nil
}
