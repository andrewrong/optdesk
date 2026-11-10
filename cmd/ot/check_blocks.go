package main

import (
	_ "embed"
	"encoding/json"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// checkBlocksJSON is the table "Blocks that must be present" of docs/design/local-e2e.md: {"pages": {"<page>": ["<block>", ...]}}.
//
//go:embed check_blocks.json
var checkBlocksJSON []byte

// checkBlockTable is the parsed table; it is read once.
var checkBlockTable = mustBlockTable(checkBlocksJSON)

func mustBlockTable(b []byte) map[string][]string {
	var t struct {
		Pages map[string][]string `json:"pages"`
	}
	if err := json.Unmarshal(b, &t); err != nil {
		panic("check_blocks.json: " + err.Error())
	}
	return t.Pages
}

// pageDayKeys names the field of each page that holds its data day. A page not listed (market_page,
// option_data_page) declares its day only in data_times.
var pageDayKeys = map[string]string{
	"report_page":            "session_date",
	"macro_rates":            "session_day",
	"option_candidates_page": "chain_day",
	"option_notices_page":    "day",
	"option_tracking_page":   "as_of",
	"settings_page":          "as_of",
}

// pageMetaKeys are the top-level keys of a flat page that are not blocks: the fields of its schema besides the
// blocks of the table. Any other top-level key of such a page is an unknown block. report_page keeps its blocks
// under "blocks", so it needs no list.
var pageMetaKeys = map[string][]string{
	"market_page": {"build_id", "data_times", "gaps", "generated_at", "notes", "run_id", "run_status", "schema", "schema_version",
		"scope", "sentence", "stale", "stale_after", "title"},
	"macro_rates": {"gaps", "generated_at", "schema", "schema_version", "sentence", "session_day", "title"},
	"option_candidates_page": {"build_id", "chain_day", "computed_at", "data_times", "estimate_note", "gaps", "generated_at", "notes",
		"rule_version", "run_id", "run_status", "schema", "schema_version", "scope", "stale", "stale_after", "title"},
	"option_notices_page": {"build_id", "data_times", "day", "gaps", "generated_at", "notes", "run_id", "run_status", "schema",
		"schema_version", "scope", "stale", "stale_after", "title"},
	"option_tracking_page": {"as_of", "build_id", "data_times", "gaps", "generated_at", "mark_note", "notes", "run_id", "run_status",
		"schema", "schema_version", "scope", "stale", "stale_after", "title"},
	"option_data_page": {"build_id", "chain_title", "data_times", "gaps", "generated_at", "notes", "notes_title", "run_id", "run_status",
		"schema", "schema_version", "scope", "sentence", "series_title", "stale", "stale_after", "title", "underlying", "underlyings"},
	"settings_page": {"as_of", "build_id", "confirm", "data_times", "form", "gap", "generated_at", "notice", "run_id", "run_status",
		"schema", "schema_version", "scope", "title"},
}

// blockNames are the top-level block names of a page body: the keys of its `blocks` object when it has one
// (report_page), else its top-level keys that are not meta fields of the page.
func blockNames(page string, body map[string]json.RawMessage) map[string]bool {
	names := map[string]bool{}
	if raw, ok := body["blocks"]; ok && page == "report_page" {
		var inner map[string]json.RawMessage
		if json.Unmarshal(raw, &inner) == nil {
			for k := range inner {
				names[k] = true
			}
		}
		return names
	}
	meta := map[string]bool{}
	for _, k := range pageMetaKeys[page] {
		meta[k] = true
	}
	for k := range body {
		if !meta[k] {
			names[k] = true
		}
	}
	return names
}

// compareBlocks returns one FAIL reason per difference between the blocks of the body of page and the table:
// `missing block <name>` for a listed name the page lacks, `unknown block <name>` for a block the table does not
// list. A page the table does not list is `unknown page <name>`. It reads no state below the top level.
func compareBlocks(page string, body map[string]json.RawMessage) []string {
	want, ok := checkBlockTable[page]
	if !ok {
		return []string{"unknown page " + page}
	}
	have := blockNames(page, body)
	var out []string
	listed := map[string]bool{}
	for _, w := range want {
		listed[w] = true
		if !have[w] {
			out = append(out, "missing block "+w)
		}
	}
	var extra []string
	for k := range have {
		if !listed[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		out = append(out, "unknown block "+k)
	}
	return out
}

// pageDataDay is the data day the page declares: its day field, else the latest day of data_times. ok is false
// when the page declares none.
func pageDataDay(page string, body map[string]json.RawMessage) (contracts.Day, bool) {
	if key, ok := pageDayKeys[page]; ok {
		var s string
		if json.Unmarshal(body[key], &s) == nil {
			if d, ok := dayPrefix(s); ok {
				return d, true
			}
		}
		return "", false
	}
	var times []struct {
		DataTime string `json:"data_time"`
	}
	if json.Unmarshal(body["data_times"], &times) != nil {
		return "", false
	}
	var best contracts.Day
	for _, t := range times {
		if d, ok := dayPrefix(t.DataTime); ok && d > best {
			best = d
		}
	}
	return best, best != ""
}

// dayPrefix reads the YYYY-MM-DD at the start of s (a day or a timestamp).
func dayPrefix(s string) (contracts.Day, bool) {
	if len(s) < len(dayLayout) {
		return "", false
	}
	if _, err := time.Parse(dayLayout, s[:len(dayLayout)]); err != nil {
		return "", false
	}
	return contracts.Day(s[:len(dayLayout)]), true
}
