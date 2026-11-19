package optioncandidates

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/candidatecur"
)

// jpEnv is the stores of the JP job over testdata/jp_open_rows.json.
type jpEnv struct {
	mu       sync.Mutex
	day      contracts.Day
	sessions []string
	names    []store.InstrumentName
	terms    map[int64]store.OptionTerms
	open     []contracts.OptionOpen
	eod      []contracts.OptionEod
	calendar string
	written  []candidatecur.Row
	inputs   []optioncandapi.CurrencyInput
	openErr  error
}

func loadJP(t *testing.T) *jpEnv {
	t.Helper()
	b, err := os.ReadFile("testdata/jp_open_rows.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Day      string   `json:"day"`
		Sessions []string `json:"sessions"`
		Names    []struct {
			ID     int64  `json:"id"`
			Symbol string `json:"symbol"`
		} `json:"names"`
		Terms []struct {
			ID     int64  `json:"id"`
			Strike string `json:"strike"`
			Expiry string `json:"expiry"`
		} `json:"terms"`
		Open []struct {
			UnderlyingID int      `json:"underlying_id"`
			InstrumentID int      `json:"instrument_id"`
			Status       int      `json:"status"`
			Currency     string   `json:"currency"`
			Bid          *float64 `json:"bid"`
			Ask          *float64 `json:"ask"`
		} `json:"open"`
		Eod []struct {
			UnderlyingID int     `json:"underlying_id"`
			InstrumentID int     `json:"instrument_id"`
			Day          string  `json:"day"`
			OI           float64 `json:"open_interest"`
		} `json:"eod"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	e := &jpEnv{day: contracts.Day(f.Day), sessions: f.Sessions, terms: map[int64]store.OptionTerms{}}
	for _, r := range f.Open {
		e.open = append(e.open, contracts.OptionOpen{UnderlyingID: r.UnderlyingID, InstrumentID: r.InstrumentID, Status: r.Status,
			Currency: r.Currency, Bid: r.Bid, Ask: r.Ask, BidSize: f64(5), UnderlyingPrice: f64(2500), MarketDataType: 1,
			CaptureID: "o" + strconv.Itoa(r.InstrumentID)})
	}
	for _, n := range f.Names {
		e.names = append(e.names, store.InstrumentName{InstrumentID: n.ID, Type: "stock", Name: n.Symbol})
	}
	for _, tm := range f.Terms {
		e.terms[tm.ID] = store.OptionTerms{InstrumentID: tm.ID, Right: "P", Strike: tm.Strike, Multiplier: "100",
			ExpirationDate: contracts.Day(tm.Expiry)}
	}
	for _, r := range f.Eod {
		e.eod = append(e.eod, contracts.OptionEod{UnderlyingID: r.UnderlyingID, InstrumentID: r.InstrumentID, SessionDay: day(r.Day),
			OpenInterest: f64(r.OI), Status: 1, CaptureID: "e1"})
	}
	return e
}

func (e *jpEnv) OptionOpen(context.Context, contracts.Day) ([]contracts.OptionOpen, error) {
	return e.open, e.openErr
}

func (e *jpEnv) OptionEod(_ context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	var out []contracts.OptionEod
	for _, r := range e.eod {
		d := contracts.Day(r.SessionDay.Format("2006-01-02"))
		if int64(r.UnderlyingID) == id && d >= from && d <= to {
			out = append(out, r)
		}
	}
	return out, nil
}

func (e *jpEnv) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	var out []store.OptionTerms
	for _, id := range ids {
		if t, ok := e.terms[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func (e *jpEnv) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, n := range e.names {
		for _, id := range ids {
			if id == n.InstrumentID {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

func (e *jpEnv) SessionDays(_ context.Context, cal string, _, _ contracts.Day) ([]contracts.SessionDay, error) {
	e.mu.Lock()
	e.calendar = cal
	e.mu.Unlock()
	var out []contracts.SessionDay
	for _, d := range e.sessions {
		out = append(out, contracts.SessionDay{CalendarID: cal, SessionDay: day(d), IsSession: true})
	}
	return out, nil
}

func (e *jpEnv) InsertCandidatesIn(_ context.Context, rows []candidatecur.Row) (int, error) {
	e.mu.Lock()
	e.written = append(e.written, rows...)
	e.mu.Unlock()
	return len(rows), nil
}

// EvaluateIn returns one ok row per quote with a quote of the 2026-12-21 expiry.
func (e *jpEnv) EvaluateIn(in optioncandapi.CurrencyInput, _ compute.CandidateParams) []compute.Candidate {
	e.mu.Lock()
	e.inputs = append(e.inputs, in)
	e.mu.Unlock()
	var out []compute.Candidate
	for _, q := range in.Quotes {
		if !q.QuoteOK || q.Expiry != "2026-12-21" {
			continue
		}
		out = append(out, compute.Candidate{Kind: compute.KindShortPut, Underlying: in.Underlying, InstrumentID: q.InstrumentID,
			Expiry: q.Expiry, DTE: 32, Right: "P", Strike: q.Strike, Multiplier: q.Multiplier, Bid: q.Bid,
			AnnualizedYield: "0.1", Status: "ok", Gates: []compute.GateResult{{Gate: "buy_price", Status: compute.GatePass}}})
	}
	return out
}

func (e *jpEnv) stores() JPStores {
	return JPStores{CandidateStores: CandidateStores{OptionEod: e, Terms: e, Names: e, SessionDays: e, Open: e}, Rows: e}
}

func jpEntry(symbol, issuer string) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: issuer, Market: "JP", Contract: broker.Contract{Symbol: symbol, SecType: "STK", Currency: "JPY"}}
}

func (e *jpEnv) build(t *testing.T, entries ...jobs.WatchEntry) contracts.Job {
	t.Helper()
	j, err := NewJP(e.stores(), e, entries, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func (e *jpEnv) rowOf(id int) (candidatecur.Row, bool) {
	for _, r := range e.written {
		if r.InstrumentID == id {
			return r, true
		}
	}
	return candidatecur.Row{}, false
}

func skippedHas(j contracts.Job, parts ...string) bool {
	for _, s := range j.(interface{ Skipped() []string }).Skipped() {
		ok := true
		for _, p := range parts {
			ok = ok && strings.Contains(s, p)
		}
		if ok {
			return true
		}
	}
	return false
}

func TestNewJPPlanKeys(t *testing.T) {
	t.Parallel()
	e := loadJP(t)
	var entries []jobs.WatchEntry
	for i := range 8 {
		entries = append(entries, jpEntry(strconv.Itoa(1000+i), "jp:"+strconv.Itoa(i)))
	}
	entries = append(entries, jpEntry("9999", ""), usEntry())
	j := e.build(t, entries...)
	if j.ID() != "option_candidates_jp" {
		t.Fatalf("id %s", j.ID())
	}
	sj, ok := j.(interface {
		Schedule() (string, string, string, time.Duration)
		After() []string
		RunBound() time.Duration
	})
	if !ok {
		t.Fatal("the job has no schedule methods")
	}
	cal, anchor, zone, off := sj.Schedule()
	if cal != "XTKS" || anchor != "session_open" || zone != "Asia/Tokyo" || off.Minutes() != 5 {
		t.Fatalf("schedule %s %s %s %s", cal, anchor, zone, off)
	}
	if a := sj.After(); len(a) != 1 || a[0] != "option_open_jp" {
		t.Fatalf("after %v", a)
	}
	if sj.RunBound().Minutes() != 10 {
		t.Fatalf("run bound %s", sj.RunBound())
	}
	keys, hash, err := j.Plan(context.Background(), e.day)
	if err != nil || len(keys) != 8 || hash == "" {
		t.Fatalf("keys %d hash %q err %v", len(keys), hash, err)
	}
	for _, k := range keys {
		if !strings.HasPrefix(k.SubjectKey, "jp:") {
			t.Fatalf("key %+v", k)
		}
	}
	if _, err := NewJP(e.stores(), e, entries, fixedClock{}); err != nil {
		t.Fatal(err)
	}
	st := e.stores()
	st.Open = nil
	if _, err := NewJP(st, e, entries, fixedClock{}); err == nil {
		t.Fatal("an open reader is required")
	}
}

func TestJPManifestKeys(t *testing.T) {
	t.Parallel()
	m, _, p, err := parseManifest(manifestJPBytes)
	if err != nil {
		t.Fatal(err)
	}
	if m.Source.Currency != "JPY" || m.Source.EodFallback || m.Schedule.Calendar != "XTKS" {
		t.Fatalf("source %+v", m.Source)
	}
	if p.MinDTE != 21 || p.MaxDTE != 45 || p.MaxSpread != "0.10" || p.MinBidSize != 1 || p.MinOpenInterest != 10 {
		t.Fatalf("params %+v", p)
	}
	us, _, _, err := parseManifest(manifestBytes)
	if err != nil || us.Source.Currency != "USD" || !us.Source.EodFallback {
		t.Fatalf("us source %+v err %v", us.Source, err)
	}
}

func TestJPSessionDaysUseXTKS(t *testing.T) {
	t.Parallel()
	e := loadJP(t)
	res := e.build(t, jpEntry("7203", "jp:7203")).Run(context.Background(), e.day, nil)
	if res.Err != nil || e.calendar != "XTKS" {
		t.Fatalf("res %+v calendar %s", res, e.calendar)
	}
}

func TestJPSkippedNoChain(t *testing.T) {
	t.Parallel()
	e := loadJP(t)
	j := e.build(t, jpEntry("7203", "jp:7203"), jpEntry("8306", "jp:8306"))
	res := j.Run(context.Background(), e.day, nil)
	if res.Err != nil || res.Returned != 2 {
		t.Fatalf("res %+v", res)
	}
	if !skippedHas(j, "jp:8306", "no option chain") {
		t.Fatalf("skipped %v", j.(interface{ Skipped() []string }).Skipped())
	}
	if _, ok := e.rowOf(101); !ok {
		t.Fatal("the name with a chain has no row")
	}
	for _, r := range e.written {
		if r.Issuer == "jp:8306" {
			t.Fatal("a name with no chain has a row")
		}
	}
	if r, _ := e.rowOf(101); r.Currency != "JPY" {
		t.Fatalf("currency %q", r.Currency)
	}
	if in := e.inputs[0]; in.Currency != "JPY" || in.ChainDay != e.day {
		t.Fatalf("input %s %s", in.Currency, in.ChainDay)
	}
}

func TestJPNoUsableQuote(t *testing.T) {
	t.Parallel()
	e := loadJP(t)
	j := e.build(t, jpEntry("7203", "jp:7203"), jpEntry("6758", "jp:6758"))
	res := j.Run(context.Background(), e.day, nil)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if !skippedHas(j, "jp:6758", "no usable open quote") {
		t.Fatalf("skipped %v", j.(interface{ Skipped() []string }).Skipped())
	}
	for _, r := range e.written {
		if r.Issuer == "jp:6758" {
			t.Fatal("a name with no usable quote has a row")
		}
		for _, m := range []string{"eod_fallback"} {
			if strings.Contains(r.Marks, m) {
				t.Fatalf("mark %s written", m)
			}
		}
	}
}

func TestJPCurrencySeparation(t *testing.T) {
	t.Parallel()
	e := loadJP(t)
	j := e.build(t, jpEntry("9984", "jp:9984"), jpEntry("7203", "jp:7203"))
	if res := j.Run(context.Background(), e.day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	if _, ok := e.rowOf(301); ok {
		t.Fatal("a USD open row was read by the JP job")
	}
	if !skippedHas(j, "jp:9984", "no option chain") {
		t.Fatalf("skipped %v", j.(interface{ Skipped() []string }).Skipped())
	}
	for _, in := range e.inputs {
		if in.Underlying == "9984" {
			t.Fatal("a name with USD rows only was evaluated")
		}
	}
}

func TestJPOpenInterestPrior(t *testing.T) {
	t.Parallel()
	e := loadJP(t)
	j := e.build(t, jpEntry("7203", "jp:7203"))
	if res := j.Run(context.Background(), e.day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	var oi map[string]int
	for _, q := range e.inputs[0].Quotes {
		if oi == nil {
			oi = map[string]int{}
		}
		oi[q.InstrumentID] = q.OpenInterest
	}
	if oi["101"] != 120 {
		t.Fatalf("open interest of 101 is %d, want the newest eod row 120", oi["101"])
	}
	if oi["104"] != 0 {
		t.Fatalf("open interest of 104 is %d, a row older than 3 sessions is not read", oi["104"])
	}
	r101, _ := e.rowOf(101)
	r104, _ := e.rowOf(104)
	if !strings.Contains(r101.Marks, "oi_prior_session") || strings.Contains(r104.Marks, "oi_prior_session") {
		t.Fatalf("marks %s | %s", r101.Marks, r104.Marks)
	}
}

func TestJPFailedKeyOtherNamesWrite(t *testing.T) {
	t.Parallel()
	e := loadJP(t)
	// 8306 is the name whose pull key failed: no open row at all.
	j := e.build(t, jpEntry("8306", "jp:8306"), jpEntry("7203", "jp:7203"))
	res := j.Run(context.Background(), e.day, nil)
	if res.Err != nil || res.Returned == 0 {
		t.Fatalf("res %+v", res)
	}
	if _, ok := e.rowOf(101); !ok {
		t.Fatal("the other name wrote no row")
	}
	e2 := loadJP(t)
	e2.openErr = errors.New("down")
	if res := e2.build(t, jpEntry("7203", "jp:7203")).Run(context.Background(), e2.day, nil); res.Err == nil {
		t.Fatal("an unreadable open pull must fail the run: there is no fallback")
	}
}
