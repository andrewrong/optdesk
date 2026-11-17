package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/valuation_page"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// Design: docs/design/valuation-structure.md#7 valuation-wire-posts (Decisions 1, 8, 10, 12, 15).

const (
	statusDraft     = "draft"
	statusEffective = "effective"
	sourceOwner     = "owner"
)

var (
	reviewTypes     = []string{"first", "post_earnings", "event", "backfill"}
	rMarkets        = []string{valuelinesapi.MarketUS, valuelinesapi.MarketJP, valuelinesapi.MarketHKCN}
	pools           = []string{"in_pool", "watch", "removed"}
	valuationIDForm = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	valuationKeyPat = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]{0,63}$`)
)

// reviewPost is one post of the entry form: the typed fields and what the store says of them.
type reviewPost struct {
	a       *app
	clk     clock.Clock
	rd      siteReaders
	h       pages.NewHandlerFunc
	w       http.ResponseWriter
	r       *http.Request
	st      valuationStore
	v       url.Values
	state   valuation.State
	rows    []underlying.Row
	key     string
	typed   valuation_page.Typed
	company string
}

// postOwnerValuation serves POST /valuation-post: the guard, then the action preview (writes nothing), draft,
// effective, void or profile. A refusal re-renders the form with 422 and the text in error; a write answers 303.
// Nothing is written before the guard passes.
func postOwnerValuation(a *app, clk clock.Clock, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	st, ok := valuationPostStart(a, rd, w, r)
	if !ok {
		return
	}
	defer st.close()
	ctx := r.Context()
	state, err := st.r.Current(ctx)
	if err != nil {
		valuationFail(a, w, "valuation post: read failed")
		return
	}
	p := &reviewPost{a: a, clk: clk, rd: rd, h: newHandler, w: w, r: r, st: st, v: r.PostForm, state: state}
	if rd.underlying != nil {
		if p.rows, err = rd.underlying.Current(ctx); err != nil {
			valuationFail(a, w, "valuation post: read failed")
			return
		}
	}
	t := func(k string) string { return strings.TrimSpace(p.v.Get(k)) }
	p.company = t("company")
	p.typed = valuation_page.Typed{
		Type: t("type"), VersionDate: t("version_date"), V: t("v"), P: t("p"),
		Key: t("key"), Name: t("name"), Symbol: t("symbol"), Currency: t("currency"), RMarket: t("r_market"),
	}
	switch action := p.v.Get("action"); action {
	case "preview":
		p.reply(ctx, http.StatusOK, "")
	case "draft", "effective", "void", "profile":
		p.write(ctx, action)
	default:
		p.reply(ctx, http.StatusUnprocessableEntity, "action: unknown")
	}
}

// reply answers the form page: its document as JSON when the request asks for JSON, else the page.
func (p *reviewPost) reply(ctx context.Context, code int, refusal string) {
	in, err := valuationPageInput(ctx, p.a, p.clk, p.rd, p.st.r)
	if err != nil {
		valuationFail(p.a, p.w, "valuation form build failed")
		return
	}
	in.Company, in.Typed, in.Error = p.company, p.typed, refusal
	vm, err := valuation_page.BuildForm(ctx, in)
	if err != nil {
		valuationFail(p.a, p.w, "valuation form build failed")
		return
	}
	if strings.Contains(p.r.Header.Get("Accept"), "application/json") {
		p.w.Header().Set("Content-Type", "application/json")
		p.w.WriteHeader(code)
		_, _ = p.w.Write(vm)
		return
	}
	p.h(vm).ServeHTTP(&statusWriter{ResponseWriter: p.w, code: code}, p.r)
}

// write runs one of the actions that insert.
func (p *reviewPost) write(ctx context.Context, action string) {
	if text := p.resolveKey(); text != "" {
		p.reply(ctx, http.StatusUnprocessableEntity, text)
		return
	}
	if action == "profile" {
		p.saveProfile(ctx)
		return
	}
	id := strings.TrimSpace(p.v.Get("review_id"))
	if id != "" && !valuationIDForm.MatchString(id) {
		p.reply(ctx, http.StatusUnprocessableEntity, "review_id: not an id")
		return
	}
	if action == "void" {
		p.void(ctx, id)
		return
	}
	if id == "" {
		id = newReviewID()
	}
	status := statusDraft
	var problems []valuelinesapi.Problem
	if action == "effective" {
		if problems = p.check(); len(problems) == 0 {
			status = statusEffective
		}
	}
	rev, text := reviewFromForm(p.v, p.key, id, status)
	if text != "" {
		p.reply(ctx, http.StatusUnprocessableEntity, text)
		return
	}
	if text := p.putProfileOnFirstSave(ctx); text != "" {
		p.reply(ctx, http.StatusUnprocessableEntity, text)
		return
	}
	if p.putReview(ctx, rev) {
		return
	}
	if len(problems) > 0 {
		p.reply(ctx, http.StatusUnprocessableEntity, "Saved as a draft: it is not effective until the checks pass.")
		return
	}
	valuationSeeOther(p.w, p.r, valuationCompanyRoute, url.Values{"company": {p.key}, "saved": {"1"}, "review": {id}})
}

// putReview inserts rev. It answers the refusal (422) or the failure (500) itself and returns true then.
func (p *reviewPost) putReview(ctx context.Context, rev valuation.Review) bool {
	err := p.st.w.PutReview(ctx, rev)
	switch {
	case err == nil:
		return false
	case errors.Is(err, valuation.ErrClosed):
		p.reply(ctx, http.StatusUnprocessableEntity, "review: this review is already effective; enter a new review or void it")
	case errors.Is(err, valuation.ErrNotEffective):
		p.reply(ctx, http.StatusUnprocessableEntity, "review: only an effective review can be voided")
	default:
		valuationFail(p.a, p.w, "valuation review write failed")
	}
	return true
}

// check is Calculator.Check of the typed record against the newest margin table.
func (p *reviewPost) check() []valuelinesapi.Problem {
	var margin []valuelinesapi.MarginRow
	if m := p.state.Margin; m != nil {
		for _, r := range m.Rows {
			margin = append(margin, valuelinesapi.MarginRow{PFrom: r.PFrom, M: r.M, W: r.W, H: r.H, S: r.S})
		}
	}
	t := p.typed
	return valuelines.New().Check(valuelinesapi.CheckInput{Type: t.Type, VersionDate: t.VersionDate, V: t.V, P: t.P, Margin: margin})
}

// resolveKey sets the company key: the company chosen, else the key typed. A typed key equal to a Key on
// settings.underlying is refused, so one company has one key (Decision 10).
func (p *reviewPost) resolveKey() string {
	p.key = p.company
	if p.key == "" {
		p.key = p.typed.Key
		for _, r := range p.rows {
			if r.Key == p.key {
				return "key: " + p.key + " is on the underlying list; choose it there"
			}
		}
	}
	if p.key == "" {
		return "company: required"
	}
	if !valuationKeyPat.MatchString(p.key) {
		return "key: use letters, digits and : . _ -"
	}
	p.company = p.key
	return ""
}

// storedProfile is the profile of the key, if it has one.
func (p *reviewPost) storedProfile() (valuation.Profile, bool) {
	for _, e := range p.state.Entries {
		if e.Profile.CompanyKey == p.key {
			return e.Profile, true
		}
	}
	return valuation.Profile{}, false
}

// putProfileOnFirstSave inserts the profile of a key that has none, from the defaults of Decision 10, before the
// review (and returns the text of a refusal). A second save finds the profile and writes none.
func (p *reviewPost) putProfileOnFirstSave(ctx context.Context) string {
	if _, ok := p.storedProfile(); ok {
		return ""
	}
	prof, text := defaultProfile(p.key, p.rows, p.typed)
	if text != "" {
		return text
	}
	if err := p.st.w.PutProfile(ctx, prof); err != nil {
		valuationFail(p.a, p.w, "valuation profile write failed")
		return "profile: not saved"
	}
	return ""
}

// defaultProfile is the profile of a key with none: from its row on settings.underlying, else from the typed fields
// of a free key. The R market comes from the market of the key (US, JP); any other key must have it chosen.
func defaultProfile(key string, rows []underlying.Row, t valuation_page.Typed) (valuation.Profile, string) {
	if i := slices.IndexFunc(rows, func(r underlying.Row) bool { return r.Key == key }); i >= 0 {
		r := rows[i]
		market := valuelinesapi.MarketUS
		if m, ok := valuation_page.MarketDefault(key); ok {
			market = m
		}
		name := r.Name
		if name == "" {
			name = r.Symbol
		}
		return valuation.Profile{CompanyKey: key, Name: name, Symbol: r.Symbol, Currency: r.Currency, RMarket: market, Pool: "in_pool"}, ""
	}
	market, ok := valuation_page.MarketDefault(key)
	if !ok {
		market = t.RMarket
	}
	if !slices.Contains(rMarkets, market) {
		return valuation.Profile{}, "r_market: choose US, JP or HKCN"
	}
	name := t.Name
	if name == "" {
		name = key
	}
	cur := t.Currency
	if cur == "" {
		cur = "USD"
	}
	return valuation.Profile{CompanyKey: key, Name: name, Symbol: t.Symbol, Currency: cur, RMarket: market, Pool: "in_pool"}, ""
}

// saveProfile inserts a profile row from the typed fields (a newer row of the key replaces the older one).
func (p *reviewPost) saveProfile(ctx context.Context) {
	prof, text := defaultProfile(p.key, p.rows, p.typed)
	if text == "" {
		if old, ok := p.storedProfile(); ok {
			prof = old
		}
		text = p.profileFields(&prof)
	}
	if text != "" {
		p.reply(ctx, http.StatusUnprocessableEntity, text)
		return
	}
	if err := p.st.w.PutProfile(ctx, prof); err != nil {
		valuationFail(p.a, p.w, "valuation profile write failed")
		return
	}
	valuationSeeOther(p.w, p.r, valuationFormRoute, url.Values{"company": {p.key}, "saved": {"1"}})
}

// profileFields lays the typed fields of the profile form over prof; a field left empty keeps its value.
func (p *reviewPost) profileFields(prof *valuation.Profile) string {
	t := func(k string) string { return strings.TrimSpace(p.v.Get(k)) }
	prof.Name = orText(p.typed.Name, prof.Name)
	prof.Symbol = orText(p.typed.Symbol, prof.Symbol)
	prof.Currency = orText(p.typed.Currency, prof.Currency)
	prof.RMarket = orText(p.typed.RMarket, prof.RMarket)
	prof.Pool = orText(t("pool"), prof.Pool)
	prof.Note = orText(t("note"), prof.Note)
	if !slices.Contains(rMarkets, prof.RMarket) {
		return "r_market: choose US, JP or HKCN"
	}
	if !slices.Contains(pools, prof.Pool) {
		return "pool: choose in_pool, watch or removed"
	}
	if d := t("pool_date"); d != "" {
		day, text := postedDay(d, "pool_date")
		if text != "" {
			return text
		}
		prof.PoolDate = day
	}
	prof.ID, prof.SetAt = 0, time.Time{}
	return ""
}

func orText(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// void inserts the void row of an effective review: a copy of its row with the status void and the reason, which is
// required.
func (p *reviewPost) void(ctx context.Context, id string) {
	reason := strings.TrimSpace(p.v.Get("void_reason"))
	switch {
	case id == "":
		p.reply(ctx, http.StatusUnprocessableEntity, "review_id: required")
		return
	case reason == "":
		p.reply(ctx, http.StatusUnprocessableEntity, "void_reason: required")
		return
	}
	rows, err := p.st.r.Reviews(ctx, p.key)
	if err != nil {
		valuationFail(p.a, p.w, "valuation post: read failed")
		return
	}
	i := slices.IndexFunc(rows, func(r valuation.Review) bool { return r.ReviewID == id && r.Status == statusEffective })
	if i < 0 {
		p.reply(ctx, http.StatusUnprocessableEntity, "review: only an effective review can be voided")
		return
	}
	row := rows[i]
	row.ID, row.EnteredAt, row.EffectiveAt = 0, time.Time{}, nil
	row.Status, row.Source, row.VoidReason = "void", sourceOwner, reason
	if p.putReview(ctx, row) {
		return
	}
	valuationSeeOther(p.w, p.r, valuationCompanyRoute, url.Values{"company": {p.key}, "saved": {"1"}, "review": {id}})
}

// newReviewID is the id of a record the form did not name: the form keeps it across the drafts of one record.
func newReviewID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "r-" + hex.EncodeToString(b[:])
}

// reviewFromForm reads the review fields of the form. The four required values may be empty on a draft; a value
// that is typed must have its form (a type of the list, a day), and the effective check is Calculator.Check.
func reviewFromForm(v url.Values, key, id, status string) (valuation.Review, string) {
	t := func(k string) string { return strings.TrimSpace(v.Get(k)) }
	rev := valuation.Review{
		ReviewID: id, CompanyKey: key, Type: t("type"), Status: status, Source: sourceOwner, V: t("v"), P: t("p"),
		Trigger: t("trigger"), EarningsBase: t("earnings_base"), BaseBasis: t("base_basis"),
		ForwardFactor: t("forward_factor"), LockedMultiple: t("locked_multiple"),
		ChangeMemo: t("change_memo"), Reaction: t("reaction"), Moat: t("moat"), Understand: t("understand"),
		Management: t("management"), Financial: t("financial"), ReportText: t("report_text"),
	}
	if rev.Type != "" && !slices.Contains(reviewTypes, rev.Type) {
		return rev, "type: choose first, post_earnings, event or backfill"
	}
	if rev.Trigger != "" && !slices.Contains([]string{"a", "b", "c", "d"}, rev.Trigger) {
		return rev, "trigger: choose a, b, c or d"
	}
	for _, d := range []struct {
		name string
		to   *contracts.Day
	}{{"version_date", &rev.VersionDate}, {"fiscal_period_end", &rev.FiscalPeriodEnd}, {"report_date", &rev.ReportDate}} {
		if t(d.name) == "" {
			continue
		}
		day, text := postedDay(t(d.name), d.name)
		if text != "" {
			return rev, text
		}
		*d.to = day
	}
	for _, k := range []string{"method_1", "method_2", "method_3"} {
		if s := t(k); s != "" {
			rev.MethodResults = append(rev.MethodResults, s)
		}
	}
	for _, dim := range []string{"p", "moat", "understand", "management", "financial"} {
		if s := t("reason_" + dim); s != "" {
			if rev.Reasons == nil {
				rev.Reasons = map[string]string{}
			}
			rev.Reasons[dim] = s
		}
	}
	return rev, ""
}
