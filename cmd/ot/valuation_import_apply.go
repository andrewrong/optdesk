package main

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/pages/valuation_import_page"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// Design: docs/design/valuation-import.md#5 (Decisions 8, 10, 11, 12).

// The keys of the two table lines of a result; a company key never has this form.
const (
	importMarginKey = "margin_table"
	importRKey      = "r_version"
)

// The fixed texts of a result line. None of them holds a value of the file or of the store.
const (
	textWritten      = "written"
	textAlready      = "already written"
	textWriteFailed  = "the write failed; nothing of this line is shown"
	textNotConfirmed = "not ticked, not written"
	textNoKey        = "no key was picked for this name"
)

// applyImport writes the plan through w in this order, each as one Writer call: the margin table, the R version,
// then per row in file order the profile and the review. A row that is same, older, refused or needs a tick that
// was not given is not written. ErrClosed is read as already written; any other error is the outcome failed with a
// fixed text, and the next rows go on. The result has one entry per line, in the order written.
func applyImport(ctx context.Context, w valuation.Writer, plan valueimportapi.Plan, ticked map[string]bool) []valuation_import_page.Outcome {
	out := []valuation_import_page.Outcome{
		applyMargin(ctx, w, plan.Margin),
		applyR(ctx, w, plan),
	}
	for _, row := range plan.Rows {
		out = append(out, applyRow(ctx, w, plan, row, ticked))
	}
	return out
}

func applyMargin(ctx context.Context, w valuation.Writer, m valueimportapi.MarginPlan) valuation_import_page.Outcome {
	if m.Action != valueimportapi.ActionInsert {
		return notWritten(importMarginKey, m.Action)
	}
	rows := make([]valuation.MarginTableRow, 0, len(m.Rows))
	for _, r := range m.Rows {
		rows = append(rows, valuation.MarginTableRow{PFrom: r.PFrom, M: r.M, W: r.W, H: r.H, S: r.S})
	}
	err := w.PutMarginTable(ctx, valuation.MarginTableVersion{EffectiveDate: m.EffectiveDate, Rows: rows})
	return written(importMarginKey, err)
}

func applyR(ctx context.Context, w valuation.Writer, plan valueimportapi.Plan) valuation_import_page.Outcome {
	r := plan.R
	if r.Action != valueimportapi.ActionInsert {
		return notWritten(importRKey, r.Action)
	}
	err := w.PutRVersion(ctx, valuation.RVersion{
		EffectiveDate: plan.AsOf, RUS: r.US, RJP: r.JP, RHKCN: r.HKCN, Note: r.Note,
		AISwitch: r.AISwitch, AIExpiry: r.AIExpiry,
	})
	return written(importRKey, err)
}

func applyRow(ctx context.Context, w valuation.Writer, plan valueimportapi.Plan, row valueimportapi.RowPlan,
	ticked map[string]bool) valuation_import_page.Outcome {
	key := rowLabel(row)
	switch row.Action {
	case valueimportapi.ActionSame:
		return notWritten(key, row.Action)
	case valueimportapi.ActionOlder:
		return notWritten(key, row.Action)
	case valueimportapi.ActionInsert, valueimportapi.ActionProfileOnly:
	default:
		return valuation_import_page.Outcome{Key: key, Outcome: valuation_import_page.OutcomeRefused, Text: row.Reason}
	}
	if row.NeedsTick && !ticked[row.Ticker] {
		return valuation_import_page.Outcome{Key: key, Outcome: valuation_import_page.OutcomeNotConfirmed, Text: textNotConfirmed}
	}
	if row.Key == "" {
		return valuation_import_page.Outcome{Key: key, Outcome: valuation_import_page.OutcomeRefused, Text: textNoKey}
	}
	closed := false
	if row.NewProfile || row.Action == valueimportapi.ActionProfileOnly {
		p := valuation.Profile{CompanyKey: row.Key, Name: row.Name, Symbol: row.Symbol, Currency: row.Currency,
			RMarket: row.RMarket, Pool: row.Pool}
		if row.Pool == "removed" {
			p.PoolDate = plan.AsOf
		}
		if err := w.PutProfile(ctx, p); err != nil {
			return failed(key)
		}
	}
	if row.Action == valueimportapi.ActionInsert && row.ReviewType != "" {
		err := w.PutReview(ctx, importReview(row))
		switch {
		case errors.Is(err, valuation.ErrClosed):
			closed = true
		case err != nil:
			return failed(key)
		}
	}
	if closed {
		return valuation_import_page.Outcome{Key: key, Outcome: valuation_import_page.OutcomeSame, Text: textAlready}
	}
	return valuation_import_page.Outcome{Key: key, Outcome: valuation_import_page.OutcomeInserted, Text: textWritten}
}

// importReview is the effective review of a row: the type, version date and review id of the plan, source owner.
func importReview(row valueimportapi.RowPlan) valuation.Review {
	return valuation.Review{
		ReviewID: row.ReviewID, CompanyKey: row.Key, Type: row.ReviewType, VersionDate: row.VersionDate,
		Status: "effective", Source: "owner", V: row.V, P: row.P, ChangeMemo: row.ChangeMemo, ReportText: row.ReportText,
	}
}

// rowLabel is the key of a row, else its ticker (an unmatched name has no key yet).
func rowLabel(row valueimportapi.RowPlan) string {
	if row.Key != "" {
		return row.Key
	}
	return row.Ticker
}

// notWritten is the line of a table or row the plan leaves alone: same, older or refused.
func notWritten(key, action string) valuation_import_page.Outcome {
	o := valuation_import_page.OutcomeRefused
	switch action {
	case valueimportapi.ActionSame:
		o = valuation_import_page.OutcomeSame
	case valueimportapi.ActionOlder:
		o = valuation_import_page.OutcomeOlder
	}
	return valuation_import_page.Outcome{Key: key, Outcome: o, Text: action}
}

// written is the line of one table write: inserted, already written for ErrClosed, else failed.
func written(key string, err error) valuation_import_page.Outcome {
	switch {
	case err == nil:
		return valuation_import_page.Outcome{Key: key, Outcome: valuation_import_page.OutcomeInserted, Text: textWritten}
	case errors.Is(err, valuation.ErrClosed):
		return valuation_import_page.Outcome{Key: key, Outcome: valuation_import_page.OutcomeSame, Text: textAlready}
	}
	return failed(key)
}

func failed(key string) valuation_import_page.Outcome {
	return valuation_import_page.Outcome{Key: key, Outcome: valuation_import_page.OutcomeFailed, Text: textWriteFailed}
}
