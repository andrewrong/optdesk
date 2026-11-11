package optionopen

import (
	"errors"
	"fmt"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"
)

type windowTable struct {
	Rule          string `toml:"rule"`
	LatenessHours int    `toml:"lateness_hours"`
	Calendar      string `toml:"calendar"`
}

type scheduleTable struct {
	Calendar         string `toml:"calendar"`
	Anchor           string `toml:"anchor"`
	Offset           string `toml:"offset"`
	RunBound         string `toml:"run_bound"`
	RestartSensitive bool   `toml:"restart_sensitive"`
}

type paramsTable struct {
	PutK           *float64 `toml:"put_k"`
	CallK          *float64 `toml:"call_k"`
	StoreK         *float64 `toml:"store_k"`
	StoreDrop      *float64 `toml:"store_drop"`
	Calls          string   `toml:"calls"`
	MinDTE         *int     `toml:"min_dte"`
	MaxDTE         *int     `toml:"max_dte"`
	MinPuts        *int     `toml:"min_puts"`
	MaxPerExpiry   *int     `toml:"max_per_expiry"`
	FarOTM         *float64 `toml:"far_otm"`
	FiveDollar     []string `toml:"five_dollar_names"`
	DeepPutsDrop   *float64 `toml:"deep_puts_drop"`
	CeilAroundSpot *int     `toml:"ceil_around_spot"`
	CeilPuts       *int     `toml:"ceil_puts"`
	CeilCalls      *int     `toml:"ceil_calls"`
	CeilPerStock   *int     `toml:"ceil_per_stock"`
}

// manifest is the common shape of job_plan.toml, job_resolve.toml and job_gap.toml.
type manifest struct {
	Version       int           `toml:"version"`
	ID            string        `toml:"id"`
	Zone          string        `toml:"zone"`
	Slot          string        `toml:"slot"`
	BudgetMinutes int           `toml:"budget_minutes"`
	MaxNew        int           `toml:"max_new"`
	RetryDays     int           `toml:"retry_days"`
	JitterDays    int           `toml:"jitter_days"`
	SpotMove      float64       `toml:"spot_move"`
	LookaheadDays int           `toml:"lookahead_days"`
	GapOnly       bool          `toml:"gap_only"`
	Window        windowTable   `toml:"window"`
	Schedule      scheduleTable `toml:"schedule"`
	Params        paramsTable   `toml:"params"`

	loc      *time.Location
	offset   time.Duration
	runBound time.Duration
	params   PlanParams
}

func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("optionopen: manifest: %w", err)
	}
	if m.ID == "" || m.BudgetMinutes <= 0 {
		return manifest{}, errors.New("optionopen: manifest needs an id and a budget")
	}
	loc, err := time.LoadLocation(m.Zone)
	if err != nil {
		return manifest{}, errors.New("optionopen: manifest: zone is unknown")
	}
	m.loc = loc
	if m.Window.Rule != "after_close" || m.Window.LatenessHours < 0 {
		return manifest{}, errors.New("optionopen: manifest: window must be after_close with lateness_hours of at least 0")
	}
	if m.Schedule.Anchor != "session_open" && m.Schedule.Anchor != "session_close" {
		return manifest{}, errors.New("optionopen: manifest: schedule.anchor must be session_open or session_close")
	}
	if m.Schedule.Calendar == "" {
		return manifest{}, errors.New("optionopen: manifest: schedule.calendar is required")
	}
	if m.offset, err = time.ParseDuration(m.Schedule.Offset); err != nil {
		return manifest{}, errors.New("optionopen: manifest: schedule.offset is not a duration")
	}
	if m.Schedule.RunBound != "" {
		if m.runBound, err = time.ParseDuration(m.Schedule.RunBound); err != nil || m.runBound < 0 {
			return manifest{}, errors.New("optionopen: manifest: schedule.run_bound is not a duration")
		}
	}
	if m.params, err = m.Params.build(); err != nil {
		return manifest{}, err
	}
	if m.MaxNew < 0 || m.RetryDays < 0 || m.JitterDays < 0 || m.JitterDays > m.RetryDays && m.RetryDays > 0 {
		return manifest{}, errors.New("optionopen: manifest: max_new, retry_days and jitter_days are out of range")
	}
	return m, nil
}

// build lays the set over DefaultPlanParams: a key the table does not give keeps the default.
func (t paramsTable) build() (PlanParams, error) {
	p := DefaultPlanParams()
	setF := func(dst *float64, v *float64) {
		if v != nil {
			*dst = *v
		}
	}
	setI := func(dst *int, v *int) {
		if v != nil {
			*dst = *v
		}
	}
	setF(&p.PutK, t.PutK)
	setF(&p.CallK, t.CallK)
	setF(&p.StoreK, t.StoreK)
	setF(&p.StoreDrop, t.StoreDrop)
	setF(&p.FarOTM, t.FarOTM)
	setF(&p.DeepPutsDrop, t.DeepPutsDrop)
	setI(&p.MinDTE, t.MinDTE)
	setI(&p.MaxDTE, t.MaxDTE)
	setI(&p.MinPuts, t.MinPuts)
	setI(&p.MaxPerExpiry, t.MaxPerExpiry)
	setI(&p.CeilAroundSpot, t.CeilAroundSpot)
	setI(&p.CeilPuts, t.CeilPuts)
	setI(&p.CeilCalls, t.CeilCalls)
	setI(&p.CeilPerStock, t.CeilPerStock)
	if t.FiveDollar != nil {
		p.FiveDollarNames = t.FiveDollar
	}
	switch CallsScope(t.Calls) {
	case "":
	case CallsHeld, CallsAll:
		p.Calls = CallsScope(t.Calls)
	default:
		return PlanParams{}, errors.New("optionopen: manifest: params.calls must be held or all")
	}
	if p.MinDTE < 0 || p.MaxDTE < p.MinDTE {
		return PlanParams{}, errors.New("optionopen: manifest: params min_dte and max_dte are out of range")
	}
	return p, nil
}

// Schedule is the slot as plain values.
func (m manifest) schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return m.Schedule.Calendar, m.Schedule.Anchor, "", m.offset
}

func (m manifest) window() (string, time.Duration, time.Duration) {
	return m.Window.Calendar, 0, time.Duration(m.Window.LatenessHours) * time.Hour
}
