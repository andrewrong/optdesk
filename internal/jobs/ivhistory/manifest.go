package ivhistory

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
	Offset   time.Duration
	RunBound time.Duration
}

// Manifest is the job.toml of iv_history.
type Manifest struct {
	Version        int
	ID             string
	Reading        string // register id of the stored figure
	ReadingVersion int
	Convention     string
	BackfillDays   int // asked when the name has no ok reading in the last LookbackDays
	DailyDays      int // asked otherwise
	LookbackDays   int
	Schedule       Schedule
}

type rawManifest struct {
	Version        int    `toml:"version"`
	ID             string `toml:"id"`
	Reading        string `toml:"reading"`
	ReadingVersion int    `toml:"reading_version"`
	Convention     string `toml:"convention"`
	BackfillDays   int    `toml:"backfill_days"`
	DailyDays      int    `toml:"daily_days"`
	LookbackDays   int    `toml:"lookback_days"`
	Schedule       struct {
		Calendar string `toml:"calendar"`
		Anchor   string `toml:"anchor"`
		Offset   string `toml:"offset"`
		RunBound string `toml:"run_bound"`
	} `toml:"schedule"`
}

func refuse(key, why string) error {
	return fmt.Errorf("ivhistory: manifest: %s %s", key, why)
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
			return Manifest{}, fmt.Errorf("ivhistory: manifest: not valid TOML at line %d column %d", row, col)
		}
		return Manifest{}, errors.New("ivhistory: manifest: not valid TOML")
	}
	for _, p := range []struct{ key, v string }{
		{"id", r.ID}, {"reading", r.Reading}, {"convention", r.Convention}, {"schedule.calendar", r.Schedule.Calendar},
	} {
		if p.v == "" {
			return Manifest{}, refuse(p.key, "is missing")
		}
	}
	for _, p := range []struct {
		key string
		v   int
	}{
		{"reading_version", r.ReadingVersion}, {"backfill_days", r.BackfillDays},
		{"daily_days", r.DailyDays}, {"lookback_days", r.LookbackDays},
	} {
		if p.v <= 0 {
			return Manifest{}, refuse(p.key, "must be positive")
		}
	}
	m := Manifest{
		Version: r.Version, ID: r.ID, Reading: r.Reading, ReadingVersion: r.ReadingVersion, Convention: r.Convention,
		BackfillDays: r.BackfillDays, DailyDays: r.DailyDays, LookbackDays: r.LookbackDays,
		Schedule: Schedule{Calendar: r.Schedule.Calendar, Anchor: r.Schedule.Anchor},
	}
	var err error
	if r.Schedule.Offset != "" {
		if m.Schedule.Offset, err = time.ParseDuration(r.Schedule.Offset); err != nil {
			return Manifest{}, refuse("schedule.offset", "is not a duration")
		}
	}
	if r.Schedule.RunBound != "" {
		if m.Schedule.RunBound, err = time.ParseDuration(r.Schedule.RunBound); err != nil {
			return Manifest{}, refuse("schedule.run_bound", "is not a duration")
		}
	}
	return m, nil
}
