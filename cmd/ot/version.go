package main

import (
	"fmt"
	"runtime/debug"
)

// version is set at build time with -ldflags "-X main.version=...". A build
// without it takes the commit it was built at (see versionFromBuildInfo).
var version = "dev"

// versionFromBuildInfo returns the version to use: v unless it is "dev", else
// the vcs.revision of bi, with "-dirty" appended when vcs.modified is "true".
// Without build information or a vcs.revision it returns "dev".
func versionFromBuildInfo(v string, bi *debug.BuildInfo) string {
	if v != "dev" || bi == nil {
		return v
	}
	var rev, modified string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev == "" {
		return v
	}
	if modified == "true" {
		rev += "-dirty"
	}
	return rev
}

func init() {
	bi, _ := debug.ReadBuildInfo()
	version = versionFromBuildInfo(version, bi)
}

func init() {
	register("version", func(a *app, _ []string) error {
		_, err := fmt.Fprintf(a.stdout, "ot %s (profile %s)\n", version, a.profile.Name)
		return err
	})
}
