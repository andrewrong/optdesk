// Package profile owns the profile (demo or real) and the log-field whitelist.
package profile

import (
	"log/slog"
	"time"
)

// Profile is the profile name, the owner's zone and the whitelist of log fields.
type Profile struct {
	// Name is "demo" or "real".
	Name string
	// Zone is the owner's zone: time.UTC in the demo profile, the zone named by OT_ZONE in the real one. Never nil.
	Zone *time.Location
	// LogFields are the only log attribute keys that pass the log handler.
	LogFields []string
}

// LoadFunc loads the profile: demo when OT_HOME is unset; reads OT_HOME, OT_PROFILE and, for the real profile only, OT_ZONE (internal/profile.Load).
type LoadFunc func(getenv func(string) string) (Profile, error)

// NewLogHandlerFunc wraps a handler so that only whitelisted fields pass (internal/profile.NewLogHandler).
type NewLogHandlerFunc func(Profile, slog.Handler) slog.Handler
