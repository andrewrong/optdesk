// Package valueimport implements valueimportapi.Importer: it reads the owner's valuation export as
// decimal text, maps it to the stored state and gives the plan. Pure: no clock, no I/O, no store.
// Source: docs/design/valuation-import.md, entry 2.
package valueimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
)

// file is the part of the export the import needs; every number is its JSON text.
type file struct {
	asOf               string
	rUS, rJP, rHK, rCN string
	rNote              string
	params             []paramRow
	removed            []removedEntry
	unvalued           []string
	companies          []company
}

type paramRow struct {
	key  string
	nums [4]string
}

type removedEntry struct{ name, ticker, currency string }

type company struct {
	ticker, name, market, currency, vVersion string
	v, p, r, bHi                             string
	note, qNote, sourceReport, flag          string
}

func bad(path string) error { return &valueimportapi.FileError{Path: path} }

type reader struct{ err error }

func (r *reader) fail(path string) {
	if r.err == nil {
		r.err = bad(path)
	}
}

func (r *reader) object(v any, path string) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		r.fail(path)
	}
	return m
}

func (r *reader) child(m map[string]any, path, key string) (any, string, bool) {
	p := key
	if path != "" {
		p = path + "." + key
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil, p, false
	}
	return v, p, true
}

func (r *reader) str(m map[string]any, path, key string, need bool) string {
	v, p, ok := r.child(m, path, key)
	if !ok {
		if need {
			r.fail(p)
		}
		return ""
	}
	s, isStr := v.(string)
	if !isStr || (need && strings.TrimSpace(s) == "") {
		r.fail(p)
		return ""
	}
	return s
}

func (r *reader) day(m map[string]any, path, key string) string {
	s := r.str(m, path, key, true)
	if s == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		r.fail(joinPath(path, key))
	}
	return s
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func (r *reader) numValue(v any, p string) string {
	n, ok := v.(json.Number)
	if !ok {
		r.fail(p)
		return ""
	}
	if _, err := decimal.NewFromString(n.String()); err != nil {
		r.fail(p)
		return ""
	}
	return n.String()
}

func (r *reader) num(m map[string]any, path, key string) string {
	v, p, ok := r.child(m, path, key)
	if !ok {
		r.fail(p)
		return ""
	}
	return r.numValue(v, p)
}

func (r *reader) list(m map[string]any, path, key string, need bool) ([]any, string) {
	v, p, ok := r.child(m, path, key)
	if !ok {
		if need {
			r.fail(p)
		}
		return nil, p
	}
	l, isList := v.([]any)
	if !isList {
		r.fail(p)
	}
	return l, p
}

// parseFile reads the export. A bad file is a *FileError that names the path and no value.
func parseFile(b []byte) (*file, error) {
	if len(b) > valueimportapi.MaxFileBytes {
		return nil, bad("file")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, bad("file")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, bad("file")
	}
	r := &reader{}
	top := r.object(root, "file")
	if r.err != nil {
		return nil, r.err
	}
	f := &file{}
	metaV, mp, ok := r.child(top, "", "meta")
	if !ok {
		return nil, bad(mp)
	}
	meta := r.object(metaV, mp)
	if r.err != nil {
		return nil, r.err
	}
	f.asOf = r.day(meta, mp, "as_of")
	f.rNote = r.str(meta, mp, "R_note", false)
	if tv, tp, ok := r.child(meta, mp, "R_table_current"); !ok {
		r.fail(tp)
	} else if t := r.object(tv, tp); t != nil {
		f.rUS, f.rJP = r.num(t, tp, "US"), r.num(t, tp, "JP")
		f.rHK, f.rCN = r.num(t, tp, "HK"), r.num(t, tp, "CN")
	}
	if pv, pp, ok := r.child(meta, mp, "params_by_P"); !ok {
		r.fail(pp)
	} else if pm := r.object(pv, pp); pm != nil {
		for k, rowV := range pm {
			rp := pp + "." + k
			row, isList := rowV.([]any)
			if !isList || len(row) != 4 {
				r.fail(rp)
				continue
			}
			pr := paramRow{key: k}
			for i, nv := range row {
				pr.nums[i] = r.numValue(nv, fmt.Sprintf("%s[%d]", rp, i))
			}
			f.params = append(f.params, pr)
		}
	}
	r.parseRemoved(meta, mp, f)
	r.parseUnvalued(meta, mp, f)
	r.parseCompanies(top, f)
	if r.err != nil {
		return nil, r.err
	}
	return f, nil
}

func (r *reader) parseRemoved(meta map[string]any, mp string, f *file) {
	l, p := r.list(meta, mp, "removed", false)
	for i, e := range l {
		ep := fmt.Sprintf("%s[%d]", p, i)
		m := r.object(e, ep)
		if m == nil {
			return
		}
		ent := removedEntry{
			name:     strings.TrimSpace(r.str(m, ep, "name", false)),
			ticker:   strings.TrimSpace(r.str(m, ep, "ticker", false)),
			currency: strings.TrimSpace(r.str(m, ep, "currency", false)),
		}
		if ent.name == "" && ent.ticker == "" {
			r.fail(ep + ".name")
			return
		}
		f.removed = append(f.removed, ent)
	}
}

func (r *reader) parseUnvalued(meta map[string]any, mp string, f *file) {
	l, p := r.list(meta, mp, "unvalued", false)
	for i, e := range l {
		s, ok := e.(string)
		if !ok {
			r.fail(fmt.Sprintf("%s[%d]", p, i))
			return
		}
		f.unvalued = append(f.unvalued, s)
	}
}

func (r *reader) parseCompanies(top map[string]any, f *file) {
	l, p := r.list(top, "", "companies", true)
	if r.err != nil {
		return
	}
	if len(l) > valueimportapi.MaxCompanies {
		r.fail(p)
		return
	}
	for i, e := range l {
		cp := fmt.Sprintf("%s[%d]", p, i)
		m := r.object(e, cp)
		if m == nil {
			return
		}
		c := company{
			ticker:   strings.TrimSpace(r.str(m, cp, "ticker", true)),
			name:     strings.TrimSpace(r.str(m, cp, "name", true)),
			market:   strings.ToUpper(strings.TrimSpace(r.str(m, cp, "market", true))),
			currency: strings.ToUpper(strings.TrimSpace(r.str(m, cp, "currency", true))),
			vVersion: r.day(m, cp, "v_version"),
			v:        r.num(m, cp, "V"), p: r.num(m, cp, "P"), r: r.num(m, cp, "R"), bHi: r.num(m, cp, "B_hi"),
			note: r.str(m, cp, "note", false), qNote: r.str(m, cp, "q_note", false),
			sourceReport: r.str(m, cp, "source_report", false), flag: r.str(m, cp, "breakpoint_flag", false),
		}
		switch c.market {
		case "US", "JP", "HK", "CN":
		default:
			r.fail(cp + ".market")
		}
		if r.err != nil {
			return
		}
		f.companies = append(f.companies, c)
	}
}
