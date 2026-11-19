package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfo(t *testing.T) {
	t.Parallel()
	info := func(kv ...string) *debug.BuildInfo {
		bi := &debug.BuildInfo{}
		for i := 0; i+1 < len(kv); i += 2 {
			bi.Settings = append(bi.Settings, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
		}
		return bi
	}
	cases := []struct {
		name    string
		version string
		bi      *debug.BuildInfo
		want    string
	}{
		{"ldflags version kept", "v1.2.3", info("vcs.revision", "abc123"), "v1.2.3"},
		{"clean revision", "dev", info("vcs.revision", "abc123", "vcs.modified", "false"), "abc123"},
		{"modified revision", "dev", info("vcs.revision", "abc123", "vcs.modified", "true"), "abc123-dirty"},
		{"no revision", "dev", info("vcs.modified", "true"), "dev"},
		{"no build info", "dev", nil, "dev"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := versionFromBuildInfo(c.version, c.bi); got != c.want {
				t.Fatalf("versionFromBuildInfo(%q) = %q, want %q", c.version, got, c.want)
			}
		})
	}
}
