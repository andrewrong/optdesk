package reconbars

import (
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/pelletier/go-toml/v2"
)

//go:embed job.toml
var manifestBytes []byte

// deadlineMargin is the part of run_bound kept for the compare and the write: the one source call stops at run_bound
// minus this.
const deadlineMargin = 2 * time.Minute

// Schedule is the [schedule] table.
type Schedule struct {
	Calendar string
	Anchor   string
	Offset   time.Duration
	RunBound time.Duration
}

// Manifest is the job.toml of recon_bars. SourceID is the value of [source] id, returned as it is written.
type Manifest struct {
	Version       int
	ID            string
	SourceID      string
	CloseRel      string // decimal text, the relative room of a close
	ToleranceText string
	Schedule      Schedule
}

type rawManifest struct {
	Version  int    `toml:"version"`
	ID       string `toml:"id"`
	Schedule struct {
		Calendar string `toml:"calendar"`
		Anchor   string `toml:"anchor"`
		Offset   string `toml:"offset"`
		RunBound string `toml:"run_bound"`
	} `toml:"schedule"`
	Source struct {
		ID string `toml:"id"`
	} `toml:"source"`
	Compare struct {
		CloseRel      string `toml:"close_rel"`
		ToleranceText string `toml:"tolerance_text"`
	} `toml:"compare"`
}

func refuse(key, why string) error {
	return fmt.Errorf("reconbars: manifest: %s %s", key, why)
}

// DefaultManifest loads the embedded job.toml.
func DefaultManifest() (Manifest, error) { return LoadManifest(manifestBytes) }

// LoadManifest reads a job.toml. A malformed value is refused with an error that names the key, never the value.
// The source id is data: any value is returned unchanged.
func LoadManifest(data []byte) (Manifest, error) {
	var r rawManifest
	if err := toml.Unmarshal(data, &r); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return Manifest{}, fmt.Errorf("reconbars: manifest: not valid TOML at line %d column %d", row, col)
		}
		return Manifest{}, errors.New("reconbars: manifest: not valid TOML")
	}
	if r.ID == "" {
		return Manifest{}, refuse("id", "is missing")
	}
	if r.Schedule.Calendar == "" {
		return Manifest{}, refuse("schedule.calendar", "is missing")
	}
	if r.Source.ID == "" {
		return Manifest{}, refuse("source.id", "is missing")
	}
	s := Schedule{Calendar: r.Schedule.Calendar, Anchor: r.Schedule.Anchor}
	var err error
	if r.Schedule.Offset != "" {
		if s.Offset, err = time.ParseDuration(r.Schedule.Offset); err != nil {
			return Manifest{}, refuse("schedule.offset", "is not a duration")
		}
	}
	if s.RunBound, err = time.ParseDuration(r.Schedule.RunBound); err != nil {
		return Manifest{}, refuse("schedule.run_bound", "is not a duration")
	}
	if s.RunBound <= deadlineMargin {
		return Manifest{}, refuse("schedule.run_bound", "must be longer than 2m")
	}
	return Manifest{
		Version: r.Version, ID: r.ID, SourceID: r.Source.ID, CloseRel: r.Compare.CloseRel,
		ToleranceText: r.Compare.ToleranceText, Schedule: s,
	}, nil
}
