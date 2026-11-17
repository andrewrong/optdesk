package valuationpg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// The reads go through the views rd.valuation_*_r1 and nothing else: a page role has no grant on a table. The tables
// are small (one owner), so the statements return every row and the rules of Decision 8 (the newest row per key, the
// current valuation) are chosen in Go, where they are tested without a database.
const (
	profilesSQL = `SELECT id, company_key, name, symbol, currency, r_market, pool, pool_date::text, note, set_at ` +
		`FROM rd.valuation_company_profile_r1 ORDER BY id`

	reviewColumns = `id, review_id, company_key, type, version_date::text, fiscal_period_end::text, report_date::text, ` +
		`status, source, v, p, trigger_type, earnings_base, base_basis, forward_factor, locked_multiple, ` +
		`method_results::text, adjustments::text, scenarios::text, change_memo, reaction, moat, understand, ` +
		`management, financial, reasons::text, report_text, void_reason, entered_at, effective_at`
	closingReviewsSQL = `SELECT ` + reviewColumns + ` FROM rd.valuation_review_r1 ` +
		`WHERE status IN ('effective', 'void') ORDER BY id`
	companyReviewsSQL = `SELECT ` + reviewColumns + ` FROM rd.valuation_review_r1 WHERE company_key = $1 ORDER BY id DESC`

	rVersionsSQL = `SELECT id, effective_date::text, yield_used, r_us, r_jp, r_hkcn, ai_switch, ai_expiry, note, set_at ` +
		`FROM rd.valuation_r_version_r1 ORDER BY id`
	rTableSQL = `SELECT effective_date::text, set_at, yield_up_to, us, jp, hkcn FROM rd.valuation_r_table_r1 ORDER BY id`
	marginSQL = `SELECT effective_date::text, set_at, p_from, m, w, h, s FROM rd.valuation_margin_table_r1 ORDER BY id`
)

// scanner is the row of database/sql.
type scanner interface{ Scan(dest ...any) error }

// query runs a statement and calls scan once per row, in the order the database returns them.
func (s *Store) query(ctx context.Context, q string, args []any, scan func(scanner) error) error {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Current returns the State: the newest profile row per company_key (ascending key), each with its current
// valuation, and the newest R version, R table version and margin table version.
func (s *Store) Current(ctx context.Context) (valuation.State, error) {
	st := valuation.State{Entries: []valuation.Entry{}}
	profiles, err := s.profiles(ctx)
	if err != nil {
		return st, err
	}
	var closing []valuation.Review
	err = s.query(ctx, closingReviewsSQL, nil, func(sc scanner) error {
		r, err := scanReview(sc)
		closing = append(closing, r)
		return err
	})
	if err != nil {
		return st, fmt.Errorf("valuationpg: current: %w", err)
	}
	current := currentByCompany(closing)
	for _, p := range newestProfiles(profiles) {
		st.Entries = append(st.Entries, valuation.Entry{Profile: p, Current: current[p.CompanyKey]})
	}
	versions, err := s.RVersions(ctx)
	if err != nil {
		return st, err
	}
	if len(versions) > 0 {
		st.RVersion = &versions[0]
	}
	rt, err := s.rTable(ctx)
	if err != nil {
		return st, err
	}
	st.RTable = rt
	mt, err := s.marginTable(ctx)
	if err != nil {
		return st, err
	}
	st.Margin = mt
	return st, nil
}

func (s *Store) profiles(ctx context.Context) ([]valuation.Profile, error) {
	var out []valuation.Profile
	err := s.query(ctx, profilesSQL, nil, func(sc scanner) error {
		var p valuation.Profile
		var poolDate sql.NullString
		if err := sc.Scan(&p.ID, &p.CompanyKey, &p.Name, &p.Symbol, &p.Currency, &p.RMarket, &p.Pool, &poolDate, &p.Note, &p.SetAt); err != nil {
			return err
		}
		p.PoolDate = contracts.Day(poolDate.String)
		out = append(out, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("valuationpg: profiles: %w", err)
	}
	return out, nil
}

// newestProfiles keeps the newest row of each company_key (by SetAt, then ID), in ascending key order.
func newestProfiles(rows []valuation.Profile) []valuation.Profile {
	best := map[string]valuation.Profile{}
	for _, p := range rows {
		b, ok := best[p.CompanyKey]
		if !ok || p.SetAt.After(b.SetAt) || (p.SetAt.Equal(b.SetAt) && p.ID > b.ID) {
			best[p.CompanyKey] = p
		}
	}
	out := make([]valuation.Profile, 0, len(best))
	for _, p := range best {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CompanyKey < out[j].CompanyKey })
	return out
}

// currentByCompany applies the rule of Decision 8 to the effective and void rows: the effective row with no void row
// of its ReviewID, whose Type is not backfill, with the latest EffectiveAt, ties by ID. A draft is never in rows.
func currentByCompany(rows []valuation.Review) map[string]*valuation.Review {
	voided := map[string]bool{}
	for _, r := range rows {
		if r.Status == "void" {
			voided[r.ReviewID] = true
		}
	}
	best := map[string]*valuation.Review{}
	for i := range rows {
		r := rows[i]
		if r.Status != "effective" || voided[r.ReviewID] || r.Type == "backfill" {
			continue
		}
		if b, ok := best[r.CompanyKey]; ok && !laterEffective(r, *b) {
			continue
		}
		best[r.CompanyKey] = &r
	}
	return best
}

func laterEffective(a, b valuation.Review) bool {
	at, bt := timeOf(a.EffectiveAt), timeOf(b.EffectiveAt)
	if !at.Equal(bt) {
		return at.After(bt)
	}
	return a.ID > b.ID
}

func timeOf(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// Reviews returns every row of a company, newest first (by ID); none gives an empty, non-nil slice.
func (s *Store) Reviews(ctx context.Context, companyKey string) ([]valuation.Review, error) {
	out := []valuation.Review{}
	err := s.query(ctx, companyReviewsSQL, []any{companyKey}, func(sc scanner) error {
		r, err := scanReview(sc)
		out = append(out, r)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("valuationpg: reviews: %w", err)
	}
	return out, nil
}

func scanReview(sc scanner) (valuation.Review, error) {
	var r valuation.Review
	var typ, vd, fpe, rd, v, p, trig, eb, bb, ff, lm, methods, adj, scen, memo, react sql.NullString
	var moat, und, mgmt, fin, reasons, text, voidReason sql.NullString
	if err := sc.Scan(&r.ID, &r.ReviewID, &r.CompanyKey, &typ, &vd, &fpe, &rd, &r.Status, &r.Source, &v, &p, &trig,
		&eb, &bb, &ff, &lm, &methods, &adj, &scen, &memo, &react, &moat, &und, &mgmt, &fin, &reasons, &text,
		&voidReason, &r.EnteredAt, &r.EffectiveAt); err != nil {
		return r, err
	}
	r.Type, r.V, r.P, r.Trigger = typ.String, v.String, p.String, trig.String
	r.VersionDate, r.FiscalPeriodEnd, r.ReportDate = contracts.Day(vd.String), contracts.Day(fpe.String), contracts.Day(rd.String)
	r.EarningsBase, r.BaseBasis, r.ForwardFactor, r.LockedMultiple = eb.String, bb.String, ff.String, lm.String
	r.ChangeMemo, r.Reaction = memo.String, react.String
	r.Moat, r.Understand, r.Management, r.Financial = moat.String, und.String, mgmt.String, fin.String
	r.ReportText, r.VoidReason = text.String, voidReason.String
	if methods.Valid {
		if err := json.Unmarshal([]byte(methods.String), &r.MethodResults); err != nil {
			return r, err
		}
	}
	if adj.Valid {
		var a []adjustmentJSON
		if err := json.Unmarshal([]byte(adj.String), &a); err != nil {
			return r, err
		}
		for _, x := range a {
			r.Adjustments = append(r.Adjustments, valuation.Adjustment(x))
		}
	}
	if scen.Valid {
		var a []scenarioJSON
		if err := json.Unmarshal([]byte(scen.String), &a); err != nil {
			return r, err
		}
		for _, x := range a {
			r.Scenarios = append(r.Scenarios, valuation.Scenario(x))
		}
	}
	if reasons.Valid {
		if err := json.Unmarshal([]byte(reasons.String), &r.Reasons); err != nil {
			return r, err
		}
	}
	return r, nil
}

// RVersions returns every R version, newest first by EffectiveDate, then SetAt, then ID; never nil.
func (s *Store) RVersions(ctx context.Context) ([]valuation.RVersion, error) {
	out := []valuation.RVersion{}
	err := s.query(ctx, rVersionsSQL, nil, func(sc scanner) error {
		var v valuation.RVersion
		var day string
		if err := sc.Scan(&v.ID, &day, &v.YieldUsed, &v.RUS, &v.RJP, &v.RHKCN, &v.AISwitch, &v.AIExpiry, &v.Note, &v.SetAt); err != nil {
			return err
		}
		v.EffectiveDate = contracts.Day(day)
		out = append(out, v)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("valuationpg: r versions: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.EffectiveDate != b.EffectiveDate {
			return a.EffectiveDate > b.EffectiveDate
		}
		if !a.SetAt.Equal(b.SetAt) {
			return a.SetAt.After(b.SetAt)
		}
		return a.ID > b.ID
	})
	return out, nil
}

// versionKey identifies the rows of one table version.
type versionKey struct {
	date contracts.Day
	at   int64 // set_at in microseconds, the precision of timestamptz
}

func (a versionKey) newerThan(b versionKey) bool {
	if a.date != b.date {
		return a.date > b.date
	}
	return a.at > b.at
}

func (s *Store) rTable(ctx context.Context) (*valuation.RTableVersion, error) {
	rows := map[versionKey][]valuation.RTableRow{}
	at := map[versionKey]time.Time{}
	err := s.query(ctx, rTableSQL, nil, func(sc scanner) error {
		var day string
		var set time.Time
		var r valuation.RTableRow
		if err := sc.Scan(&day, &set, &r.YieldUpTo, &r.US, &r.JP, &r.HKCN); err != nil {
			return err
		}
		k := versionKey{contracts.Day(day), set.UnixMicro()}
		rows[k] = append(rows[k], r)
		at[k] = set
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("valuationpg: r table: %w", err)
	}
	k, ok := newestKey(rows)
	if !ok {
		return nil, nil
	}
	return &valuation.RTableVersion{EffectiveDate: k.date, SetAt: at[k], Rows: rows[k]}, nil
}

func (s *Store) marginTable(ctx context.Context) (*valuation.MarginTableVersion, error) {
	rows := map[versionKey][]valuation.MarginTableRow{}
	at := map[versionKey]time.Time{}
	err := s.query(ctx, marginSQL, nil, func(sc scanner) error {
		var day string
		var set time.Time
		var r valuation.MarginTableRow
		if err := sc.Scan(&day, &set, &r.PFrom, &r.M, &r.W, &r.H, &r.S); err != nil {
			return err
		}
		k := versionKey{contracts.Day(day), set.UnixMicro()}
		rows[k] = append(rows[k], r)
		at[k] = set
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("valuationpg: margin table: %w", err)
	}
	k, ok := newestKey(rows)
	if !ok {
		return nil, nil
	}
	return &valuation.MarginTableVersion{EffectiveDate: k.date, SetAt: at[k], Rows: rows[k]}, nil
}

// newestKey is the version with the latest EffectiveDate, then SetAt.
func newestKey[T any](m map[versionKey][]T) (versionKey, bool) {
	var best versionKey
	found := false
	for k := range m {
		if !found || k.newerThan(best) {
			best, found = k, true
		}
	}
	return best, found
}
