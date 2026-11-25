package main

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/valuation_import_page"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// Design: docs/design/valuation-import.md#5 (Decisions 1, 2, 11, 12).

const (
	// importFormMargin is what a post may hold beyond the file: the fields of the form (ticks and picks).
	importFormMargin = 256 << 10
	// importFieldMax is the bytes of one field kept; a field is a ticker, a key, a market, a hash or an action.
	importFieldMax = 1 << 10
	// importPartsMax is the parts read before the file part.
	importPartsMax = 2000

	importTooLargeText = "upload too large: at most 1 MiB"
	importNoFileText   = "No file was sent: pick the file and post again."
	importNoStoreText  = "The valuation store could not be opened: nothing was written."
	importOffText      = "Import is off: it is not set up on this site."
)

// importForm is a post as read: the fields that came before the file part and the file itself.
type importForm struct {
	fields map[string][]string
	file   []byte
	name   string
}

// postOwnerValuationImport serves POST /valuation-import-post. A method other than POST is 405; a declared length
// over the file cap and the form margin is 413; the owner cookie is checked before any byte of the body is read.
// The parts are read with MultipartReader, the fields first and the file last, into memory: no
// form parse, no file, no temporary file. The action preview answers the page with the plan and writes nothing;
// the action confirm plans again on the state of that moment with the posted picks and writes what the plan says.
func postOwnerValuationImport(a *app, clk clock.Clock, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.ContentLength < 0 || r.ContentLength > valueimportapi.MaxFileBytes+importFormMargin {
		http.Error(w, importTooLargeText, http.StatusRequestEntityTooLarge)
		return
	}
	if !ownerCookieOK(r, rd.edit) {
		refuseOwner(w, rd.edit)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, valueimportapi.MaxFileBytes+importFormMargin)
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, refusedText, http.StatusForbidden)
		return
	}
	form, err := readImportForm(mr)
	if err != nil {
		refuseImport(w, err)
		return
	}
	p := importPost{a: a, clk: clk, rd: rd, newHandler: newHandler, w: w, r: r}
	switch {
	case form.file == nil:
		p.answer(http.StatusUnprocessableEntity, valuation_import_page.Input{View: valuation_import_page.ViewEmpty, Error: importNoFileText})
	case a.deps.planValueImport == nil:
		p.answer(http.StatusOK, valuation_import_page.Input{View: valuation_import_page.ViewEmpty, Gaps: []string{importOffText}})
	default:
		p.run(form)
	}
}

// importPost is one post past the guard: the page is answered with the document built from an Input.
type importPost struct {
	a          *app
	clk        clock.Clock
	rd         siteReaders
	newHandler pages.NewHandlerFunc
	w          http.ResponseWriter
	r          *http.Request
}

func (p importPost) answer(status int, in valuation_import_page.Input) {
	vm, err := valuation_import_page.Build(p.r.Context(), in)
	if err != nil {
		p.a.log.Error("valuation import build failed", "component", "ot", "status", "failed")
		http.Error(p.w, "internal error", http.StatusInternalServerError)
		return
	}
	p.newHandler(vm).ServeHTTP(&statusWriter{ResponseWriter: p.w, code: status}, p.r)
}

// gap answers the empty page with one plain sentence.
func (p importPost) gap(text string) {
	p.answer(http.StatusOK, valuation_import_page.Input{View: valuation_import_page.ViewEmpty, Gaps: []string{text}})
}

// badFile answers the empty page with the sentence for a file the importer cannot read.
func (p importPost) badFile(err error) {
	p.answer(http.StatusUnprocessableEntity, valuation_import_page.Input{View: valuation_import_page.ViewEmpty, Error: badFileText(err)})
}

// run opens the store, plans, and for a confirm of the file that was previewed writes the plan.
func (p importPost) run(form importForm) {
	st, err := openValuationStore(p.a)
	if err != nil {
		p.a.log.Error("valuation import: no store", "component", "ot", "status", "failed")
		p.gap(importNoStoreText)
		return
	}
	defer st.close()
	ctx := p.r.Context()
	state, err := st.r.Current(ctx)
	if err != nil {
		p.a.log.Error("valuation import: read failed", "component", "ot", "status", "failed")
		p.gap(importNoStoreText)
		return
	}
	day, err := dayOrDefault(p.clk, "")
	if err != nil {
		p.a.log.Error("valuation import: no day", "component", "ot", "status", "failed")
		http.Error(p.w, "internal error", http.StatusInternalServerError)
		return
	}
	latest := ""
	if y := latestYield(ctx, p.rd.series, day); y != nil {
		latest = y.Value
	}
	in := valueimportapi.Input{File: form.file, Stored: importStored(ctx, p.rd, state, latest), Calc: valuelines.New()}
	confirm := form.get("action") == "confirm"
	if confirm {
		in.Picks = importPicks(form.fields)
	}
	plan, err := p.a.deps.planValueImport(in)
	if err != nil {
		p.badFile(err)
		return
	}
	page := valuation_import_page.Input{View: valuation_import_page.ViewPreview, Plan: plan, LatestYield: latest, FileName: displayName(form.name)}
	if confirm && form.get("file_sha256") != plan.FileSHA256 {
		// another file than the one previewed: nothing is written and the page shows the preview of this file
		in.Picks = nil
		if page.Plan, err = p.a.deps.planValueImport(in); err != nil {
			p.badFile(err)
			return
		}
		confirm = false
	}
	if confirm {
		page.View = valuation_import_page.ViewResult
		page.Applied = applyImport(ctx, st.w, plan, tickedOf(form.fields))
		if slices.ContainsFunc(page.Applied, func(o valuation_import_page.Outcome) bool {
			return o.Outcome == valuation_import_page.OutcomeInserted
		}) {
			page.Applied = append(page.Applied, p.refreshGate(ctx, plan, page.Applied, day)...)
		}
	}
	p.answer(http.StatusOK, page)
}

// refuseImport answers a post that failed before the guard passed or while it was read: 413 over the cap, else 403.
func refuseImport(w http.ResponseWriter, err error) {
	if errors.Is(err, errImportTooLarge) || isCapErr(err) {
		http.Error(w, importTooLargeText, http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, refusedText, http.StatusForbidden)
}

var errImportTooLarge = errors.New("valuation import: file too large")

// readImportForm reads the parts one at a time: a part that is not the file is a field, kept up to a small size; the
// file part is the last and is read into memory up to MaxFileBytes. Parts after the file are not read.
func readImportForm(mr *multipart.Reader) (importForm, error) {
	f := importForm{fields: map[string][]string{}}
	for range importPartsMax {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return f, nil
		}
		if err != nil {
			return f, err
		}
		if p.FormName() == "file" {
			b, err := io.ReadAll(io.LimitReader(p, valueimportapi.MaxFileBytes+1))
			if err != nil {
				return f, err
			}
			if len(b) > valueimportapi.MaxFileBytes {
				return f, errImportTooLarge
			}
			f.file, f.name = b, p.FileName()
			return f, nil
		}
		b, err := io.ReadAll(io.LimitReader(p, importFieldMax))
		if err != nil {
			return f, err
		}
		f.fields[p.FormName()] = append(f.fields[p.FormName()], string(b))
	}
	return f, errors.New("valuation import: too many parts")
}

func (f importForm) get(k string) string {
	if v := f.fields[k]; len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	return ""
}

// importPicks reads key.<ticker> and r_market.<ticker> into the picks of the owner by ticker.
func importPicks(fields map[string][]string) map[string]valueimportapi.Pick {
	picks := map[string]valueimportapi.Pick{}
	for k, v := range fields {
		t, ok := strings.CutPrefix(k, "key.")
		if !ok || t == "" || len(v) == 0 {
			continue
		}
		p := valueimportapi.Pick{Key: strings.TrimSpace(v[0])}
		if m := fields["r_market."+t]; len(m) > 0 {
			p.RMarket = strings.TrimSpace(m[0])
		}
		picks[t] = p
	}
	return picks
}

// tickedOf is the set of tickers ticked: the values of the field tick.
func tickedOf(fields map[string][]string) map[string]bool {
	out := map[string]bool{}
	for _, t := range fields["tick"] {
		out[strings.TrimSpace(t)] = true
	}
	return out
}

// importStored turns the valuation state, the list of settings.underlying and the latest yield into the state the
// importer compares with.
func importStored(ctx context.Context, rd siteReaders, st valuation.State, latest string) valueimportapi.Stored {
	s := valueimportapi.Stored{Latest: latest}
	if rd.underlying != nil {
		if rows, err := rd.underlying.Current(ctx); err == nil {
			s.Underlying = underlyingStored(rows)
		}
	}
	for _, e := range st.Entries {
		p := e.Profile
		s.Profiles = append(s.Profiles, valueimportapi.StoredProfile{Key: p.CompanyKey, Name: p.Name, Symbol: p.Symbol,
			Currency: p.Currency, RMarket: p.RMarket, Pool: p.Pool})
		if c := e.Current; c != nil {
			s.Current = append(s.Current, valueimportapi.StoredCurrent{Key: p.CompanyKey, V: c.V, P: c.P, VersionDate: c.VersionDate})
		}
	}
	if v := st.RVersion; v != nil {
		s.R = &valueimportapi.StoredR{US: v.RUS, JP: v.RJP, HKCN: v.RHKCN, EffectiveDate: v.EffectiveDate,
			AISwitch: v.AISwitch, AIExpiry: v.AIExpiry}
	}
	if m := st.Margin; m != nil {
		s.MarginDate = m.EffectiveDate
		for _, r := range m.Rows {
			s.Margin = append(s.Margin, valuelinesapi.MarginRow(r))
		}
	}
	return s
}

func underlyingStored(rows []underlying.Row) []valueimportapi.StoredUnderlying {
	out := make([]valueimportapi.StoredUnderlying, 0, len(rows))
	for _, r := range rows {
		out = append(out, valueimportapi.StoredUnderlying{Key: r.Key, Symbol: r.Symbol, Market: r.Market})
	}
	return out
}

// badFileText is the sentence for a file the importer cannot read: the path of the key, never a value.
func badFileText(err error) string {
	if fe, ok := errors.AsType[*valueimportapi.FileError](err); ok {
		return "The file cannot be read: " + fe.Path
	}
	return "The file cannot be read."
}
