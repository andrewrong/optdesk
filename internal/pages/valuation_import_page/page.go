// Package valuation_import_page builds the page.valuation_import_page.v1 view model: the upload form (view
// empty), the preview of a valueimportapi.Plan (view preview) and the outcome of a confirm (view result). Every
// number and text is a field of the plan or of the input; the builder rounds and computes nothing.
// Design: docs/design/valuation-import.md#4.
package valuation_import_page

import "github.com/andrewrong/optdesk/internal/compute/valueimportapi"

// The views of the page.
const (
	ViewEmpty   = "empty"
	ViewPreview = "preview"
	ViewResult  = "result"
)

// The outcomes of one row of a confirm.
const (
	OutcomeInserted     = "inserted"
	OutcomeSame         = "same"
	OutcomeOlder        = "older"
	OutcomeRefused      = "refused"
	OutcomeNotConfirmed = "not_confirmed"
	OutcomeFailed       = "failed"
)

const (
	postPath    = "valuation-import-post"
	maxFileName = 64
)

// Outcome is what the confirm did with one key.
type Outcome struct{ Key, Outcome, Text string }

// Input is everything Build reads. Plan and FileName are unused in the view empty; Applied is read in the view
// result. Gaps and Problems are plain sentences.
type Input struct {
	View        string
	Plan        valueimportapi.Plan
	Applied     []Outcome
	LatestYield string
	FileName    string
	Gaps        []string
	Problems    []string
	Error       string
}

type fileJSON struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	AsOf   string `json:"as_of"`
}

type rJSON struct {
	Action      string `json:"action"`
	FileUS      string `json:"file_us"`
	FileJP      string `json:"file_jp"`
	FileHKCN    string `json:"file_hkcn"`
	StoredUS    string `json:"stored_us"`
	StoredJP    string `json:"stored_jp"`
	StoredHKCN  string `json:"stored_hkcn"`
	Note        string `json:"note"`
	LatestYield string `json:"latest_yield"`
	Problem     string `json:"problem"`
}

type marginRowJSON struct {
	PFrom string `json:"p_from"`
	M     string `json:"m"`
	W     string `json:"w"`
	H     string `json:"h"`
	S     string `json:"s"`
}

type marginJSON struct {
	Action string          `json:"action"`
	Rows   []marginRowJSON `json:"rows"`
}

type rowJSON struct {
	Key         string `json:"key"`
	Ticker      string `json:"ticker"`
	Name        string `json:"name"`
	Currency    string `json:"currency"`
	RMarket     string `json:"r_market"`
	NewProfile  bool   `json:"new_profile"`
	Unmatched   bool   `json:"unmatched"`
	ProposedKey string `json:"proposed_key"`
	Action      string `json:"action"`
	Reason      string `json:"reason"`
	ReviewType  string `json:"review_type"`
	V           string `json:"v"`
	P           string `json:"p"`
	VersionDate string `json:"version_date"`
	FileBuyTop  string `json:"file_buy_top"`
	SiteBuyTop  string `json:"site_buy_top"`
	NeedsTick   bool   `json:"needs_tick"`
}

type countsJSON struct {
	Insert    int `json:"insert"`
	Same      int `json:"same"`
	Older     int `json:"older"`
	Refused   int `json:"refused"`
	NeedsTick int `json:"needs_tick"`
}

type resultJSON struct {
	Key     string `json:"key"`
	Outcome string `json:"outcome"`
	Text    string `json:"text"`
}

type problemJSON struct {
	Text string `json:"text"`
}

type gapJSON struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type labelsJSON struct {
	R              string `json:"r"`
	BuyZoneTopSite string `json:"buy_zone_top_site"`
	BuyZoneTopFile string `json:"buy_zone_top_file"`
	GeneralTick    string `json:"general_tick"`
}

type pageJSON struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	View          string        `json:"view"`
	PostPath      string        `json:"post_path"`
	MaxFileBytes  int           `json:"max_file_bytes"`
	Labels        labelsJSON    `json:"labels"`
	File          *fileJSON     `json:"file"`
	R             *rJSON        `json:"r"`
	Margin        *marginJSON   `json:"margin"`
	Rows          []rowJSON     `json:"rows"`
	NotInFile     []string      `json:"not_in_file"`
	Unvalued      []string      `json:"unvalued"`
	Removed       []string      `json:"removed"`
	Counts        countsJSON    `json:"counts"`
	FlaggedNote   *string       `json:"flagged_note"`
	Result        []resultJSON  `json:"result"`
	Problems      []problemJSON `json:"problems"`
	Gaps          []gapJSON     `json:"gaps"`
	Error         *string       `json:"error"`
}
