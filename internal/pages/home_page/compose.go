// Package home_page composes the home_page payload from five summary documents.
//
// Compose is a pure function: it reads no clock, no store and no file. The five
// members are written through as they came; Compose only checks their identity
// and folds their run status, data times and stale_after into the envelope.
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
	schemaVersion = 2
	maxDataTimes  = 20
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
	GapLine     string
	BlockLinks  []Link
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
	StaleAfter    string          `json:"stale_after,omitempty"`
	DataTimes     []sourceTime    `json:"data_times"`
	RunStatus     string          `json:"run_status"`
	Title         string          `json:"title"`
	Say           string          `json:"say"`
	GapLine       string          `json:"gap_line"`
	BlockLinks    []Link          `json:"block_links"`
	ReportLinks   []Link          `json:"report_links"`
	Household     json.RawMessage `json:"household"`
	Companies     json.RawMessage `json:"companies"`
	Macro         json.RawMessage `json:"macro"`
	Market        json.RawMessage `json:"market"`
	Options       json.RawMessage `json:"options"`
}

// Compose returns the home_page payload of in, with generated_at set to
// generatedAt written in in.Zone. It returns an error when Zone is nil, GapLine
// is empty, a member does not carry the schema, schema_version and run identity
// of its slot (the error names the slot), or the members hold more than 20
// distinct data times.
func Compose(in Input, generatedAt time.Time) ([]byte, error) {
	if in.Zone == nil {
		return nil, errors.New("home_page: nil Zone")
	}
	if in.GapLine == "" {
		return nil, errors.New("home_page: empty GapLine")
	}
	slots := []slot{
		{"household", "household_summary", 2, in.Household},
		{"companies", "held_companies_summary", 2, in.Companies},
		{"macro", "macro_summary", 2, in.Macro},
		{"market", "market_summary", 3, in.Market},
		{"options", "options_summary", 3, in.Options},
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
		GapLine:       in.GapLine,
		BlockLinks:    nonNil(in.BlockLinks),
		ReportLinks:   nonNil(in.ReportLinks),
		Household:     in.Household,
		Companies:     in.Companies,
		Macro:         in.Macro,
		Market:        in.Market,
		Options:       in.Options,
	}
	var earliest time.Time
	seen := map[sourceTime]bool{}
	for _, s := range slots {
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
