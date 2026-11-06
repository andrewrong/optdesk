// Package profile owns the profile (demo or real) and the log-field whitelist.
package profile

import "log/slog"

// Profile is the profile name and the whitelist of log fields.
type Profile struct {
	// Name is "demo" or "real".
	Name string
	// LogFields are the only log attribute keys that pass the log handler.
	LogFields []string
}

// LoadFunc loads the profile: demo when OT_HOME is unset; reads only OT_HOME and OT_PROFILE (internal/profile.Load).
type LoadFunc func(getenv func(string) string) (Profile, error)

// NewLogHandlerFunc wraps a handler so that only whitelisted fields pass (internal/profile.NewLogHandler).
type NewLogHandlerFunc func(Profile, slog.Handler) slog.Handler
