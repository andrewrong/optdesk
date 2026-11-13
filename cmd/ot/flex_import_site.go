package main

import (
	"context"
	"errors"
	"github.com/andrewrong/optdesk/internal/accounts"
	"path/filepath"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	optionledgerjob "github.com/andrewrong/optdesk/internal/jobs/optionledger"
	flexpage "github.com/andrewrong/optdesk/internal/pages/flex_import"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	optionledgerstore "github.com/andrewrong/optdesk/internal/store/optionledger"
)

// Design: docs/design/flex-xml-import.md#8 ot-flex-import-route.

// siteFlexImportRoute is the address of the page flex_import under the site root. With --owner-edit and the
// option ledger line it carries the upload form; otherwise it is read-only.
const siteFlexImportRoute = "flex-import"

// flexImportPostRoute is the address of the upload post, under the site root; the page builder holds the same
// path as the post_path of the form.
const flexImportPostRoute = "owner-flex-import"

const (
	// flexImportLogin is the login of the records of the import (Decision 2).
	flexImportLogin = "import"
	// flexImportRecent is how many of the newest records the page lists.
	flexImportRecent = 50
	// flexImportMaxFiles is the files read of one post.
	flexImportMaxFiles = 8
	// flexImportFileText is what the form says of the size of one file.
	flexImportFileText = "50 MiB after decompression"

	gapReasonOff      = "no_reader"
	gapReasonInput    = "missing_input"
	gapReasonNoData   = "no_data"
	gapPastUnreadable = "past imports cannot be read"
)

func init() {
	registerPage(pageFamily{route: siteFlexImportRoute,
		build: func(ctx context.Context, a *app, clk clock.Clock, _ string, rd siteReaders) ([]byte, error) {
			return buildFlexImportPage(ctx, a, clk, rd, flexImportResult{recent: true})
		}})
}

// flexImportResult is what a build adds to the page: the results of a post, or the recent list of a GET.
type flexImportResult struct {
	files  []flexpage.File
	recent bool
	// gaps are lines of a post that say what could not be stored.
	gaps []flexpage.Gap
}

// flexImportGate is the env file and flex file of an import that may run.
type flexImportGate struct {
	env map[string]string
	ff  sources.FlexFile
	// book is the account book of accounts.toml: it maps the account number of a statement to its alias.
	book accounts.Book
}

// flexImportGate reads the env file and the Flex file. A line that is unset, a file that is missing or not valid
// is the one gap line that says why the import is off; the texts name a key or a file, never a value.
func (a *app) flexImportGate() (flexImportGate, *flexpage.Gap) {
	off := func(reason, why string) (flexImportGate, *flexpage.Gap) {
		return flexImportGate{}, &flexpage.Gap{Kind: "gap", Reason: reason, Text: "Import is off: " + why + "."}
	}
	env, err := a.envFile()
	if err != nil {
		return off(gapReasonInput, "the env file cannot be read")
	}
	if env[store.DSNEnvOptionLedger] == "" {
		return off(gapReasonInput, reasonNoLedgerDSN)
	}
	if a.deps.loadFlexFile == nil {
		return off(gapReasonInput, reasonNoFlexFile)
	}
	ff, err := a.deps.loadFlexFile(filepath.Join(a.getenv("OT_HOME"), flexFileName))
	switch {
	case errors.Is(err, sources.ErrFlexNotConfigured):
		return off(gapReasonInput, reasonNoFlexFile)
	case err != nil:
		return off(gapReasonInput, reasonBadFlexFile)
	}
	book, err := a.loadBook()
	if err != nil {
		return off(gapReasonInput, "accounts.toml cannot be loaded")
	}
	return flexImportGate{env: env, ff: ff, book: book}, nil
}

// buildFlexImportPage builds page.flex_import.v1 per request. Without --owner-edit, or with the ledger line unset
// or the Flex file not loading, the page is read-only: form null and one gap line. Otherwise it carries a fresh
// token and, for a GET, the newest records read with the pages line; a record list that cannot be read is an
// empty list and one gap line.
func buildFlexImportPage(ctx context.Context, a *app, clk clock.Clock, rd siteReaders, x flexImportResult) ([]byte, error) {
	if rd.edit == nil {
		return flexpage.Build(flexpage.Input{Gaps: []flexpage.Gap{
			{Kind: "gap", Reason: gapReasonOff, Text: "Import is off: start ot serve with --owner-edit to upload files."}}})
	}
	g, gap := a.flexImportGate()
	if gap != nil {
		return flexpage.Build(flexpage.Input{Gaps: []flexpage.Gap{*gap}})
	}
	tok := rd.edit.form.Token(clk.Now().UTC())
	in := flexpage.Input{
		Form:  &flexpage.Form{Expires: tok.Expires, MAC: tok.MAC, MaxFiles: flexImportMaxFiles, MaxFileText: flexImportFileText},
		Files: x.files, Gaps: x.gaps,
	}
	if x.recent {
		recs, err := a.flexRecentFetches(ctx, g.env)
		if err != nil {
			in.Gaps = append(in.Gaps, flexpage.Gap{Kind: "gap", Reason: gapReasonNoData, Text: gapPastUnreadable})
		}
		in.Recent = recentOf(recs, a.profile.Zone)
	}
	return flexpage.Build(in)
}

// flexRecentFetches reads the newest import records with the read-only pages line. The error is fixed: no
// driver text leaves this function.
func (a *app) flexRecentFetches(ctx context.Context, env map[string]string) ([]store.FlexFetchRecord, error) {
	fail := func() ([]store.FlexFetchRecord, error) {
		a.log.Warn("flex import recent not read", "component", "ot", "status", "failed")
		return nil, errors.New("flex import: recent records cannot be read")
	}
	dsn := env[store.DSNEnvPages]
	if a.deps.openOptionLedgerRO == nil || dsn == "" {
		return fail()
	}
	st, err := a.deps.openOptionLedgerRO(ctx, dsn)
	if err != nil {
		return fail()
	}
	defer func() { _ = st.Close() }()
	il, ok := st.(optionledgerstore.ImportLog)
	if !ok {
		return fail()
	}
	recs, err := il.RecentFetches(ctx, flexImportLogin, flexImportRecent)
	if err != nil {
		return fail()
	}
	return recs, nil
}

// recentOf turns stored records into the rows of the list; the status of each is the one derivation of the
// import (DeriveImportStatus).
func recentOf(recs []store.FlexFetchRecord, zone *time.Location) []flexpage.Recent {
	if zone == nil {
		zone = time.UTC
	}
	out := make([]flexpage.Recent, 0, len(recs))
	for _, rec := range recs {
		status, reason := optionledgerjob.DeriveImportStatus(rec)
		out = append(out, flexpage.Recent{
			AtText: rec.FinishedAt.In(zone).Format("2006-01-02 15:04 MST"), Alias: optionledgerjob.ImportAlias(rec.QueryID),
			Status: status, Reason: reason, RowsParsed: rec.RowsParsed, RowsNew: rec.RowsNew,
			RowsPresent: max(rec.RowsParsed-rec.RowsNew-rec.RowsRevised, 0), Unmapped: rec.UnmappedStatement,
		})
	}
	return out
}
