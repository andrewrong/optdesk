package underlyingresolve

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
	Calendar string
	Anchor   string
	Zone     string
	Offset   time.Duration
	RunBound time.Duration
}

// Manifest is the job.toml of underlying_resolve.
type Manifest struct {
	Version   int
	ID        string
	RetryDays int // an unresolved name is asked again after this many days
	MaxNames  int // names asked in one run
	Schedule  Schedule
}

type rawManifest struct {
	Version   int    `toml:"version"`
	ID        string `toml:"id"`
	RetryDays int    `toml:"retry_days"`
	MaxNames  int    `toml:"max_names"`
	Schedule  struct {
		Calendar string `toml:"calendar"`
		Anchor   string `toml:"anchor"`
		Zone     string `toml:"zone"`
		Offset   string `toml:"offset"`
		RunBound string `toml:"run_bound"`
	} `toml:"schedule"`
}

func refuse(key, why string) error {
	return fmt.Errorf("underlyingresolve: manifest: %s %s", key, why)
}

// DefaultManifest loads the embedded job.toml.
func DefaultManifest() (Manifest, error) { return LoadManifest(manifestBytes) }

// LoadManifest reads a job.toml. A malformed value is refused with an error that names the key, never the value.
func LoadManifest(data []byte) (Manifest, error) {
	var r rawManifest
	if err := toml.Unmarshal(data, &r); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return Manifest{}, fmt.Errorf("underlyingresolve: manifest: not valid TOML at line %d column %d", row, col)
		}
		return Manifest{}, errors.New("underlyingresolve: manifest: not valid TOML")
	}
	if r.ID == "" {
		return Manifest{}, refuse("id", "is missing")
	}
	if r.RetryDays < 1 {
		return Manifest{}, refuse("retry_days", "must be at least 1")
	}
	if r.MaxNames < 1 {
		return Manifest{}, refuse("max_names", "must be at least 1")
	}
	if r.Schedule.Calendar == "" {
		return Manifest{}, refuse("schedule.calendar", "is missing")
	}
	if _, err := time.LoadLocation(r.Schedule.Zone); err != nil || r.Schedule.Zone == "" {
		return Manifest{}, refuse("schedule.zone", "is not a zone")
	}
	off, err := time.ParseDuration(r.Schedule.Offset)
	if err != nil {
		return Manifest{}, refuse("schedule.offset", "is not a duration")
	}
	bound, err := time.ParseDuration(r.Schedule.RunBound)
	if err != nil {
		return Manifest{}, refuse("schedule.run_bound", "is not a duration")
	}
	return Manifest{
		Version: r.Version, ID: r.ID, RetryDays: r.RetryDays, MaxNames: r.MaxNames,
		Schedule: Schedule{
			Calendar: r.Schedule.Calendar, Anchor: r.Schedule.Anchor, Zone: r.Schedule.Zone, Offset: off, RunBound: bound,
		},
	}, nil
}
