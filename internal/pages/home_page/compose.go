// Package home_page composes the home_page payload from five summary documents.
//
// Compose is a pure function: it reads no clock, no store and no file. The five
// members are written through as they came; Compose only checks their identity
// and folds their run status, data times and stale_after into the envelope, and
// copies the overview from them.
package home_page

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	schemaName    = "home_page"
	schemaVersion = 3
	maxDataTimes  = 20
	staleDays     = 4 // the freshness rule of macro_page: data day + 4 days at 09:30
	timeLayout    = "2006-01-02T15:04:05-07:00"
)

// Link is a link of the home page: the path it goes to and its words.
type Link struct {
	Href string `json:"href"`
	Text string `json:"text"`
}

// Input is what Compose needs: the identity and texts of the run and the five
// member documents, each the whole JSON payload of one summary page.
type Input struct {
	RunID       string
	BuildID     string
	Scope       string
	Zone        *time.Location
	Title       string
	Say         string
	CardLinks   CardLinks
	ReportLinks []Link

	Household []byte
	Companies []byte
	Macro     []byte
	Market    []byte
	Options   []byte
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

// member holds the fields of a summary document that Compose reads.
type member struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	RunStatus     string       `json:"run_status"`
	DataTimes     []sourceTime `json:"data_times"`
	StaleAfter    *string      `json:"stale_after"`
}

type slot struct {
	name    string
	schema  string
	version int
	doc     []byte
}

type payload struct {
	Schema        string          `json:"schema"`
	SchemaVersion int             `json:"schema_version"`
	RunID         string          `json:"run_id"`
	BuildID       string          `json:"build_id"`
	Scope         string          `json:"scope"`
	GeneratedAt   string          `json:"generated_at"`
	StaleAfter    string          `json:"stale_after"`
	DataTimes     []sourceTime    `json:"data_times"`
	RunStatus     string          `json:"run_status"`
	Title         string          `json:"title"`
	Say           string          `json:"say"`
	GapLine       string          `json:"gap_line"`
	Overview      overview        `json:"overview"`
	ReportLinks   []Link          `json:"report_links"`
	Household     json.RawMessage `json:"household"`
	Companies     json.RawMessage `json:"companies"`
	Macro         json.RawMessage `json:"macro"`
	Market        json.RawMessage `json:"market"`
	Options       json.RawMessage `json:"options"`
}

// Compose returns the home_page payload of in, with generated_at set to
// generatedAt written in in.Zone. The overview and gap_line are built from the
// members (see buildOverview). stale_after is the earliest of the members that
// carry one, else the earliest data_time day plus 4 days at 09:30 in in.Zone. It
// returns an error when Zone is nil, a card link does not start with "/", a card
// has neither say nor subtitle, a member does not carry the schema, schema_version and run identity
// of its slot (the error names the slot), or the members hold more than 20
// distinct data times.
func Compose(in Input, generatedAt time.Time) ([]byte, error) {
	if in.Zone == nil {
		return nil, errors.New("home_page: nil Zone")
	}
	ov, gaps, err := buildOverview(in)
	if err != nil {
		return nil, err
	}
	out := payload{
		Schema:        schemaName,
		SchemaVersion: schemaVersion,
		RunID:         in.RunID,
		BuildID:       in.BuildID,
		Scope:         in.Scope,
		GeneratedAt:   generatedAt.In(in.Zone).Format(timeLayout),
		DataTimes:     []sourceTime{},
		RunStatus:     "ok",
		Title:         in.Title,
		Say:           in.Say,
		GapLine:       gapLine(gaps),
		Overview:      ov,
		ReportLinks:   nonNil(in.ReportLinks),
		Household:     in.Household,
		Companies:     in.Companies,
		Macro:         in.Macro,
		Market:        in.Market,
		Options:       in.Options,
	}
	var earliest, earliestData time.Time
	seen := map[sourceTime]bool{}
	for _, s := range slotsOf(in) {
		m, err := readMember(s, in)
		if err != nil {
			return nil, fmt.Errorf("home_page: %s: %w", s.name, err)
		}
		if rank(m.RunStatus) > rank(out.RunStatus) {
			out.RunStatus = m.RunStatus
		}
		for _, dt := range m.DataTimes {
			if !seen[dt] {
				seen[dt] = true
				t, err := time.Parse(time.RFC3339, dt.DataTime)
				if err != nil {
					return nil, fmt.Errorf("home_page: %s: data_time: %w", s.name, err)
				}
				if earliestData.IsZero() || t.Before(earliestData) {
					earliestData = t
				}
				out.DataTimes = append(out.DataTimes, dt)
			}
		}
		if m.StaleAfter != nil {
			t, err := time.Parse(time.RFC3339, *m.StaleAfter)
			if err != nil {
				return nil, fmt.Errorf("home_page: %s: stale_after: %w", s.name, err)
			}
			if earliest.IsZero() || t.Before(earliest) {
				earliest = t
				out.StaleAfter = *m.StaleAfter
			}
		}
	}
	if out.StaleAfter == "" {
		if out.StaleAfter, err = fallbackStale(earliestData, in.Zone); err != nil {
			return nil, err
		}
	}
	if len(out.DataTimes) > maxDataTimes {
		return nil, fmt.Errorf("home_page: %d data times, at most %d", len(out.DataTimes), maxDataTimes)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("home_page: encode: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// slotsOf lists the five members with the schema and version each must carry.
func slotsOf(in Input) []slot {
	return []slot{
		{"household", "household_summary", 2, in.Household},
		{"companies", "held_companies_summary", 3, in.Companies},
		{"macro", "macro_summary", 2, in.Macro},
		{"market", "market_summary", 3, in.Market},
		{"options", "options_summary", 4, in.Options},
	}
}

// fallbackStale is the day of the earliest data time plus staleDays at 09:30 in zone.
func fallbackStale(earliestData time.Time, zone *time.Location) (string, error) {
	if earliestData.IsZero() {
		return "", errors.New("home_page: no stale_after and no data_times in the members")
	}
	d := earliestData.In(zone)
	return time.Date(d.Year(), d.Month(), d.Day()+staleDays, 9, 30, 0, 0, zone).Format(timeLayout), nil
}

func readMember(s slot, in Input) (member, error) {
	var m member
	if err := json.Unmarshal(s.doc, &m); err != nil {
		return m, fmt.Errorf("not a JSON object: %w", err)
	}
	switch {
	case m.Schema != s.schema:
		return m, fmt.Errorf("schema %q, want %q", m.Schema, s.schema)
	case m.SchemaVersion != s.version:
		return m, fmt.Errorf("schema_version %d, want %d", m.SchemaVersion, s.version)
	case m.RunID != in.RunID:
		return m, fmt.Errorf("run_id %q, want %q", m.RunID, in.RunID)
	case m.BuildID != in.BuildID:
		return m, fmt.Errorf("build_id %q, want %q", m.BuildID, in.BuildID)
	case m.Scope != in.Scope:
		return m, fmt.Errorf("scope %q, want %q", m.Scope, in.Scope)
	case rank(m.RunStatus) < 0:
		return m, fmt.Errorf("run_status %q", m.RunStatus)
	}
	return m, nil
}

// rank orders the run statuses by badness; an unknown status is -1.
func rank(status string) int {
	switch status {
	case "ok":
		return 0
	case "partial":
		return 1
	case "failed":
		return 2
	}
	return -1
}

func nonNil(l []Link) []Link {
	if l == nil {
		return []Link{}
	}
	return l
}
