package datacheck

import (
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/pelletier/go-toml/v2"
)

//go:embed job.toml
var manifestBytes []byte

// Schedule is the [schedule] table.
type Schedule struct {
	Calendar         string
	Anchor           string
	Zone             string
	Offset           time.Duration
	RunBound         time.Duration
	RestartSensitive bool
}

// Manifest is the job.toml of data_check.
type Manifest struct {
	Version  int
	ID       string
	Schedule Schedule
}

type rawManifest struct {
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
}

func refuse(key, why string) error {
	return fmt.Errorf("datacheck: manifest: %s %s", key, why)
}

// DefaultManifest loads the embedded job.toml.
func DefaultManifest() (Manifest, error) { return LoadManifest(manifestBytes) }

// LoadManifest reads a job.toml. A malformed value is refused with an error that names the key, never the value.
func LoadManifest(data []byte) (Manifest, error) {
	var r rawManifest
	if err := toml.Unmarshal(data, &r); err != nil {
		if de, ok := errors.AsType[*toml.DecodeError](err); ok {
			row, col := de.Position()
			return Manifest{}, fmt.Errorf("datacheck: manifest: not valid TOML at line %d column %d", row, col)
		}
		return Manifest{}, errors.New("datacheck: manifest: not valid TOML")
	}
	if r.ID == "" {
		return Manifest{}, refuse("id", "is missing")
	}
	if r.Schedule.Calendar == "" {
		return Manifest{}, refuse("schedule.calendar", "is missing")
	}
	if r.Schedule.Anchor == "" {
		return Manifest{}, refuse("schedule.anchor", "is missing")
	}
	s := Schedule{
		Calendar: r.Schedule.Calendar, Anchor: r.Schedule.Anchor, Zone: r.Schedule.Zone,
		RestartSensitive: r.Schedule.RestartSensitive,
	}
	var err error
	if r.Schedule.Offset != "" {
		if s.Offset, err = time.ParseDuration(r.Schedule.Offset); err != nil {
			return Manifest{}, refuse("schedule.offset", "is not a duration")
		}
	}
	if s.RunBound, err = time.ParseDuration(r.Schedule.RunBound); err != nil || s.RunBound <= 0 {
		return Manifest{}, refuse("schedule.run_bound", "is not a positive duration")
	}
	return Manifest{Version: r.Version, ID: r.ID, Schedule: s}, nil
}
