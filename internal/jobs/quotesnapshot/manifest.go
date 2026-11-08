package quotesnapshot

import (
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/pelletier/go-toml/v2"
)

//go:embed job.toml
var manifestBytes []byte

// The code defaults of the optional keys of job.toml.
const (
	defaultZone     = "America/New_York"
	defaultGrace    = 10
	defaultChunk    = 50
	defaultBudget   = 8
	defaultLateness = 4
	defaultRule     = "after_close"
	defaultCalendar = "XNYS"
)

var defaultSlots = []string{"0945", "1230", "1530"}

type window struct {
	Rule          string `toml:"rule"`
	LatenessHours *int   `toml:"lateness_hours"`
	Calendar      string `toml:"calendar"`
}

// instrument is one [[instrument]] entry.
type instrument struct {
	Symbol         string `toml:"symbol"`
	SecType        string `toml:"sec_type"`
	Exchange       string `toml:"exchange"`
	Currency       string `toml:"currency"`
	ConID          int64  `toml:"con_id"`
	MIC            string `toml:"mic"`
	CalendarID     string `toml:"calendar_id"`
	PriceIncrement string `toml:"price_increment"`
	LotSize        string `toml:"lot_size"`
}

type manifest struct {
	Version       int           `toml:"version"`
	ID            string        `toml:"id"`
	Zone          string        `toml:"zone"`
	Slots         []string      `toml:"slots"`
	GraceMinutes  *int          `toml:"grace_minutes"`
	Chunk         *int          `toml:"chunk"`
	BudgetMinutes *int          `toml:"budget_minutes"`
	Window        window        `toml:"window"`
	Schedule      scheduleTable `toml:"schedule"`
	Instruments   []instrument  `toml:"instrument"`

	loc   *time.Location
	sched schedule
}

func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

func (m manifest) grace() time.Duration {
	return time.Duration(intOr(m.GraceMinutes, defaultGrace)) * time.Minute
}
func (m manifest) budget() time.Duration {
	return time.Duration(intOr(m.BudgetMinutes, defaultBudget)) * time.Minute
}
func (m manifest) chunk() int { return intOr(m.Chunk, defaultChunk) }

// parseManifest reads job.toml, takes the defaults of the missing optional keys and checks the values.
func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("quotesnapshot: manifest: %w", err)
	}
	if m.ID == "" || len(m.Instruments) == 0 {
		return manifest{}, errors.New("quotesnapshot: manifest needs an id and an instrument list")
	}
	if m.Zone == "" {
		m.Zone = defaultZone
	}
	loc, err := time.LoadLocation(m.Zone)
	if err != nil {
		return manifest{}, fmt.Errorf("quotesnapshot: manifest: zone: %w", err)
	}
	m.loc = loc
	if len(m.Slots) == 0 {
		m.Slots = append([]string(nil), defaultSlots...)
	}
	m.Slots = append([]string(nil), m.Slots...)
	sort.Strings(m.Slots)
	for _, s := range m.Slots {
		if _, err := time.Parse("1504", s); err != nil || len(s) != 4 {
			return manifest{}, fmt.Errorf("quotesnapshot: manifest: slot %q is not HHMM", s)
		}
	}
	if m.Window.Rule == "" {
		m.Window.Rule = defaultRule
	}
	if m.Window.Calendar == "" {
		m.Window.Calendar = defaultCalendar
	}
	if m.Window.LatenessHours == nil {
		h := defaultLateness
		m.Window.LatenessHours = &h
	}
	if intOr(m.GraceMinutes, defaultGrace) < 0 || intOr(m.BudgetMinutes, defaultBudget) <= 0 || m.chunk() <= 0 {
		return manifest{}, errors.New("quotesnapshot: manifest: grace_minutes, chunk and budget_minutes are out of range")
	}
	for _, in := range m.Instruments {
		if in.Symbol == "" || in.ConID == 0 || in.Currency == "" {
			return manifest{}, errors.New("quotesnapshot: manifest: an instrument needs symbol, con_id and currency")
		}
	}
	if m.sched, err = scheduleOf(m.Schedule); err != nil {
		return manifest{}, err
	}
	return m, nil
}
