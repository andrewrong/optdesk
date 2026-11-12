// Package flex_import builds page.flex_import.v1, the Flex XML import page. Every sentence is written here from a
// closed table or from counts; the view computes and formats nothing.
package flex_import

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	postPath    = "/owner-flex-import"
	gapKind     = "gap"
	gapNoReader = "no_reader"
	nameRunes   = 80
	titleText   = "Flex import"

	// scopeNote is the one fixed line that says what the import reads and what it leaves out.
	scopeNote = "Only trades of level EXECUTION are read, from the same Flex query that option_ledger uses for trades " +
		"(the TradeQueries entry of its login in flex.toml); cash, positions, NAV and the other sections of the file " +
		"are not imported."
	readOnlyGapText = "Import is off: start ot serve with --owner-edit and the option ledger line to upload files."
	offSentence     = "The import is off."
)

// File statuses, in the order in which the first that applies wins.
const (
	StatusFailed       = "failed"
	StatusStored       = "stored"
	StatusPresent      = "present"
	StatusUnconfigured = "unconfigured"
	StatusNothingFound = "nothing_found"
)

// statusText is the closed table of the sentence of a status that needs no count; stored is written by storedText.
var statusText = map[string]string{
	StatusPresent:      "Already stored: no new rows",
	StatusNothingFound: "No trades found in the file",
	StatusUnconfigured: "No alias of the file is configured",
}

// reasonText is the closed table of the sentence of a failure reason.
var reasonText = map[string]string{
	"not_report":     "Failed: the file is not a Flex report",
	"too_large":      "Failed: the file is too large, at most 50 MiB after decompression",
	"too_many_files": "Failed: too many files, at most 8 per upload",
	"bad_gzip":       "Failed: the gzip file cannot be read",
	"store_error":    "Failed: the rows could not be written",
}

// summaryWords is the closed table of the words of one status in the sentence of a result, in sentence order.
var summaryWords = []struct{ status, words string }{
	{StatusStored, "stored"},
	{StatusPresent, "already present"},
	{StatusNothingFound, "without trades"},
	{StatusUnconfigured, "unmapped"},
	{StatusFailed, "failed"},
}

// Form is the upload form: the token fields and the limits, as plain values.
type Form struct {
	Expires     string
	MAC         string
	MaxFiles    int
	MaxFileText string
}

// Gap is one gap line, as the common Gap block.
type Gap struct{ Kind, Reason, Text string }

// Alias is the counts of one alias of a file.
type Alias struct {
	Alias                                        string
	Statements, RowsParsed, RowsNew, RowsPresent int
}

// File is the result of one uploaded file. A non-empty Reason is a failure; otherwise the status is derived from
// the counts: stored, present, unconfigured or nothing_found.
type File struct {
	Name        string
	Reason      string
	PeriodFirst string
	PeriodLast  string
	Aliases     []Alias
	Unmapped    int
}

// Recent is one earlier import record; Status and Reason are given by the caller (derived from the stored record).
type Recent struct {
	AtText                                     string
	Alias                                      string
	Status                                     string
	Reason                                     string
	RowsParsed, RowsNew, RowsPresent, Unmapped int
}

// Input is everything the page shows. A nil Form means the import is off.
type Input struct {
	Form   *Form
	Gaps   []Gap
	Files  []File
	Recent []Recent
}

type pageJSON struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	Title         string       `json:"title"`
	Sentence      string       `json:"sentence"`
	ScopeNote     string       `json:"scope_note"`
	Form          *formJSON    `json:"form"`
	Gaps          []gapJSON    `json:"gaps"`
	Files         []fileJSON   `json:"files"`
	Recent        []recentJSON `json:"recent"`
}

type formJSON struct {
	PostPath    string `json:"post_path"`
	Expires     string `json:"expires"`
	MAC         string `json:"mac"`
	MaxFiles    int    `json:"max_files"`
	MaxFileText string `json:"max_file_text"`
}

type gapJSON struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type fileJSON struct {
	Name        string      `json:"name"`
	Status      string      `json:"status"`
	Reason      string      `json:"reason"`
	StatusText  string      `json:"status_text"`
	PeriodFirst string      `json:"period_first"`
	PeriodLast  string      `json:"period_last"`
	Aliases     []aliasJSON `json:"aliases"`
	Unmapped    int         `json:"unmapped_statements"`
}

type aliasJSON struct {
	Alias       string `json:"alias"`
	Statements  int    `json:"statements"`
	RowsParsed  int    `json:"rows_parsed"`
	RowsNew     int    `json:"rows_new"`
	RowsPresent int    `json:"rows_present"`
}

type recentJSON struct {
	AtText      string `json:"at_text"`
	Alias       string `json:"alias"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	RowsParsed  int    `json:"rows_parsed"`
	RowsNew     int    `json:"rows_new"`
	RowsPresent int    `json:"rows_present"`
	Unmapped    int    `json:"unmapped_statements"`
}

// Build returns the JSON of page.flex_import.v1 for in. A nil Form gives form null. The status of a file is the
// first that applies of failed, stored, present, unconfigured and nothing_found. An unknown reason of a file or
// record, an unknown status of a record, a failed record with no reason or a reason on any other status is an error.
func Build(in Input) ([]byte, error) {
	p := pageJSON{
		Schema: "flex_import", SchemaVersion: 1, Title: titleText, ScopeNote: scopeNote,
		Gaps: []gapJSON{}, Files: []fileJSON{}, Recent: []recentJSON{},
	}
	if f := in.Form; f != nil {
		p.Form = &formJSON{PostPath: postPath, Expires: f.Expires, MAC: f.MAC, MaxFiles: f.MaxFiles, MaxFileText: f.MaxFileText}
	}
	for _, g := range in.Gaps {
		p.Gaps = append(p.Gaps, gapJSON(g))
	}
	counts := map[string]int{}
	for _, f := range in.Files {
		fj, err := buildFile(f)
		if err != nil {
			return nil, err
		}
		counts[fj.Status]++
		p.Files = append(p.Files, fj)
	}
	for _, r := range in.Recent {
		if err := checkStatusReason(r.Status, r.Reason); err != nil {
			return nil, err
		}
		p.Recent = append(p.Recent, recentJSON(r))
	}
	switch {
	case in.Form == nil:
		p.Sentence = offSentence
	case len(in.Files) > 0:
		p.Sentence = resultSentence(len(in.Files), counts)
	default:
		p.Sentence = chooseSentence(in.Form.MaxFiles, len(in.Recent))
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("flex_import: %w", err)
	}
	return out, nil
}

func buildFile(f File) (fileJSON, error) {
	status := deriveStatus(f)
	if err := checkStatusReason(status, f.Reason); err != nil {
		return fileJSON{}, err
	}
	fj := fileJSON{
		Name: cutName(f.Name), Status: status, Reason: f.Reason, PeriodFirst: f.PeriodFirst, PeriodLast: f.PeriodLast,
		Aliases: []aliasJSON{}, Unmapped: f.Unmapped,
	}
	newRows := 0
	for _, a := range f.Aliases {
		newRows += a.RowsNew
		fj.Aliases = append(fj.Aliases, aliasJSON(a))
	}
	switch status {
	case StatusFailed:
		fj.StatusText = reasonText[f.Reason]
	case StatusStored:
		fj.StatusText = fmt.Sprintf("Stored: %d new %s", newRows, plural(newRows, "row", "rows"))
	default:
		fj.StatusText = statusText[status]
	}
	return fj, nil
}

// deriveStatus is the first status that applies to the file.
func deriveStatus(f File) string {
	var parsed, added int
	for _, a := range f.Aliases {
		parsed += a.RowsParsed
		added += a.RowsNew
	}
	switch {
	case f.Reason != "":
		return StatusFailed
	case added > 0:
		return StatusStored
	case parsed > 0:
		return StatusPresent
	case f.Unmapped > 0:
		return StatusUnconfigured
	}
	return StatusNothingFound
}

// checkStatusReason accepts a status of the closed list, with a reason of the closed table exactly when failed.
func checkStatusReason(status, reason string) error {
	switch status {
	case StatusStored, StatusPresent, StatusNothingFound, StatusUnconfigured:
		if reason != "" {
			return fmt.Errorf("flex_import: reason %q on status %q", reason, status)
		}
	case StatusFailed:
		if _, ok := reasonText[reason]; !ok {
			return fmt.Errorf("flex_import: unknown reason %q", reason)
		}
	default:
		return fmt.Errorf("flex_import: unknown status %q", status)
	}
	return nil
}

// cutName is the base name of an uploaded file name, at most nameRunes runes.
func cutName(name string) string {
	if name == "" {
		return ""
	}
	base := filepath.Base(name)
	if r := []rune(base); len(r) > nameRunes {
		base = string(r[:nameRunes])
	}
	return base
}

func resultSentence(n int, counts map[string]int) string {
	var parts []string
	for _, w := range summaryWords {
		if c := counts[w.status]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, w.words))
		}
	}
	return fmt.Sprintf("%d %s read: %s.", n, plural(n, "file", "files"), strings.Join(parts, ", "))
}

func chooseSentence(maxFiles, recent int) string {
	head := fmt.Sprintf("Choose up to %d Flex XML files; ", maxFiles)
	switch recent {
	case 0:
		return head + "no earlier import is listed."
	case 1:
		return head + "1 earlier import is listed below."
	}
	return head + fmt.Sprintf("%d earlier imports are listed below.", recent)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
