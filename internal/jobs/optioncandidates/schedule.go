package optioncandidates

import (
	"errors"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/compute"
)

type manifest struct {
	Version  int    `toml:"version"`
	ID       string `toml:"id"`
	Schedule struct {
		Calendar         string `toml:"calendar"`
		Anchor           string `toml:"anchor"`
		Zone             string `toml:"zone"`
		Offset           string `toml:"offset"`
		RunBound         string `toml:"run_bound"`
		RestartSensitive bool   `toml:"restart_sensitive"`
	} `toml:"schedule"`
	Params struct {
		MinDTE              int    `toml:"min_dte"`
		MaxDTE              int    `toml:"max_dte"`
		MinBidSize          int    `toml:"min_bid_size"`
		MaxSpread           string `toml:"max_spread"`
		MinOpenInterest     int    `toml:"min_open_interest"`
		DivGuardFactor      string `toml:"div_guard_factor"`
		MaxRateAgeDays      int    `toml:"max_rate_age_days"`
		MaxEventAgeSessions int    `toml:"max_event_age_sessions"`
		MinReportGapDays    int    `toml:"min_report_gap_days"`
	} `toml:"params"`
}

// slot is the checked [schedule] table as plain values.
type slot struct {
	calendar, anchor, zone string
	offset, runBound       time.Duration
	restartSensitive       bool
}

// parseManifest reads the embedded job.toml; errors name the key and never a value.
func parseManifest(b []byte) (manifest, slot, compute.CandidateParams, error) {
	var m manifest
	if err := toml.Unmarshal(b, &m); err != nil || m.ID == "" {
		return m, slot{}, compute.CandidateParams{}, errors.New("optioncandidates: manifest is invalid")
	}
	s := m.Schedule
	off, err1 := time.ParseDuration(s.Offset)
	bound, err2 := time.ParseDuration(s.RunBound)
	if err1 != nil || err2 != nil || s.Calendar == "" || s.Anchor == "" {
		return m, slot{}, compute.CandidateParams{}, errors.New("optioncandidates: manifest: schedule is invalid")
	}
	p := m.Params
	if p.MinDTE < 0 || p.MaxDTE < p.MinDTE || p.MaxSpread == "" || p.DivGuardFactor == "" {
		return m, slot{}, compute.CandidateParams{}, errors.New("optioncandidates: manifest: params are invalid")
	}
	return m, slot{s.Calendar, s.Anchor, s.Zone, off, bound, s.RestartSensitive}, compute.CandidateParams{
		MinDTE: p.MinDTE, MaxDTE: p.MaxDTE, MinBidSize: p.MinBidSize, MaxSpread: p.MaxSpread,
		MinOpenInterest: p.MinOpenInterest, DivGuardFactor: p.DivGuardFactor, MaxRateAgeDays: p.MaxRateAgeDays,
		MaxEventAgeSessions: p.MaxEventAgeSessions, MinReportGapDays: p.MinReportGapDays,
	}, nil
}
