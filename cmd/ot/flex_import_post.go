package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	optionledgerjob "github.com/andrewrong/optdesk/internal/jobs/optionledger"
	"github.com/andrewrong/optdesk/internal/pages"
	flexpage "github.com/andrewrong/optdesk/internal/pages/flex_import"
	"github.com/andrewrong/optdesk/internal/sources/flex"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/flex-xml-import.md#8 ot-flex-import-route.

const (
	// flexImportMax is the cap of one post, declared and read (Decision 9).
	flexImportMax = 64 << 20
	// flexFieldMax is the cap of one leading field (expires, mac) of the form.
	flexFieldMax = 512
	// flexNameMax is the bytes of a file name kept for display.
	flexNameMax = 64
	// flexMoreFiles is the entry that stands for the files of a post past the eighth.
	flexMoreFiles = "(more files)"

	refusedText  = "upload refused: the form is not valid or has expired, load the page again"
	tooLargeText = "upload too large: at most 64 MiB"
)

// errBrokenUpload is a stream that ended before its last part: the post stops with the files read so far.
var errBrokenUpload = errors.New("flex import: upload broke")

// errUploadRefused ends a post whose guard failed: 403 and the fixed text.
var errUploadRefused = errors.New("flex import: upload refused")

// postOwnerFlexImport serves POST /owner-flex-import: the declared length against 64 MiB, the leading fields
// expires and mac, CheckUpload, then at most 8 file parts one at a time, each parsed, imported or left as a failed
// record, and the page of the results. Nothing is written before the guard passes.
func postOwnerFlexImport(a *app, clk clock.Clock, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.ContentLength < 0 || r.ContentLength > flexImportMax {
		http.Error(w, tooLargeText, http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, flexImportMax)
	now := clk.Now().UTC()
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, refusedText, http.StatusForbidden)
		return
	}
	tok, first, err := readUploadFields(mr)
	if err != nil {
		refuseUpload(w, err)
		return
	}
	up, ok := rd.edit.form.(settingsapi.UploadChecker)
	req := settingsapi.FormRequest{
		RemoteAddr: r.RemoteAddr, Host: r.Host, Origin: r.Header.Get("Origin"), Listen: rd.edit.listen,
		ContentType: r.Header.Get("Content-Type"), BodySize: r.ContentLength, Token: tok,
	}
	if !ok || up.CheckUpload(req, flexImportMax, now) != nil {
		http.Error(w, refusedText, http.StatusForbidden)
		return
	}
	page := func(ctx context.Context, x flexImportResult) {
		vm, err := buildFlexImportPage(ctx, a, clk, rd, x)
		if err != nil {
			a.log.Error("serve build failed", "component", "ot", "status", "failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		newHandler(vm).ServeHTTP(w, r)
	}
	g, gap := a.flexImportGate()
	if gap != nil {
		page(r.Context(), flexImportResult{})
		return
	}
	run := &flexImportRun{a: a, ctx: r.Context(), dsn: g.env[store.DSNEnvOptionLedger], accounts: a.flexAccounts(g.ff, g.env),
		id: newRequestID(), now: now}
	defer run.close()
	files, err := run.readFiles(mr, first)
	if err != nil {
		http.Error(w, tooLargeText, http.StatusRequestEntityTooLarge)
		return
	}
	page(r.Context(), flexImportResult{files: files, gaps: run.gaps()})
}

// refuseUpload answers a post that failed before the guard passed: 413 for a stream over the cap, else 403.
func refuseUpload(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		http.Error(w, tooLargeText, http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, refusedText, http.StatusForbidden)
}

// readUploadFields reads the leading parts of the form up to the first file part, which it returns unread. The
// fields expires and mac must both come first: a file part before them, or a stream with no file part and
// without them, is refused. A part of another name is dropped.
func readUploadFields(mr *multipart.Reader) (settingsapi.FormToken, *multipart.Part, error) {
	var tok settingsapi.FormToken
	var haveExpires, haveMAC bool
	for {
		p, err := mr.NextPart()
		if err != nil {
			return tok, nil, errUploadOr(err)
		}
		switch p.FormName() {
		case "expires", "mac":
			b, err := io.ReadAll(io.LimitReader(p, flexFieldMax+1))
			if err != nil || len(b) > flexFieldMax {
				return tok, nil, errUploadOr(err)
			}
			if p.FormName() == "expires" {
				tok.Expires, haveExpires = string(b), true
			} else {
				tok.MAC, haveMAC = string(b), true
			}
		case "files":
			if !haveExpires || !haveMAC {
				return tok, nil, errUploadRefused
			}
			return tok, p, nil
		}
		if haveExpires && haveMAC {
			return tok, nil, nil
		}
	}
}

// errUploadOr is the error of a failed read of a leading part: the cap error as it is, else a refusal.
func errUploadOr(err error) error {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return err
	}
	return errUploadRefused
}

// newRequestID is 8 hex characters from crypto/rand, made once per post.
func newRequestID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("flex import: no random source")
	}
	return hex.EncodeToString(b[:])
}

// displayName is the base name of a client file name, `\` read as `/`, cut to 64 bytes. It is for display only: it
// is never joined to a path, opened or written.
func displayName(name string) string {
	if name == "" {
		return ""
	}
	base := path.Base(strings.ReplaceAll(name, `\`, "/"))
	if len(base) > flexNameMax {
		base = strings.ToValidUTF8(base[:flexNameMax], "")
	}
	return base
}

// flexImportRun is one post: the account map, the request id and the ledger writer, opened at the first record to
// write and closed with the post.
type flexImportRun struct {
	a        *app
	ctx      context.Context
	dsn      string
	accounts map[string]string
	id       string
	now      time.Time

	st         store.OptionLedgerStore
	openFailed bool
	lostRecord bool
}

// writer opens the ledger store once; a failed open is not tried again.
func (p *flexImportRun) writer() (store.OptionLedgerStore, bool) {
	if p.st != nil {
		return p.st, true
	}
	if p.openFailed {
		return nil, false
	}
	st, err := p.a.deps.openOptionLedgerStore(p.ctx, p.dsn)
	if err != nil || st == nil {
		p.openFailed = true
		p.a.log.Error("flex import store not opened", "component", "ot", "status", "failed")
		return nil, false
	}
	p.st = st
	return st, true
}

func (p *flexImportRun) close() {
	if p.st != nil {
		_ = p.st.Close()
	}
}

// gaps is the line that says what was not stored, when a write failed.
func (p *flexImportRun) gaps() []flexpage.Gap {
	text := ""
	switch {
	case p.openFailed:
		text = "The ledger store could not be opened: nothing was stored and no record was left."
	case p.lostRecord:
		text = "A record could not be written: it will not show after a reload."
	default:
		return nil
	}
	return []flexpage.Gap{{Kind: "gap", Reason: gapReasonNoData, Text: text}}
}

// readFiles reads the file parts one at a time, the first being first (nil when the stream had none), at most 8,
// and answers one result each. The ninth part is not read; it is one entry `(more files)`. A part of another name
// is dropped. A stream over the cap is an error and the post answers 413; a stream that broke ends the post with
// the files read so far.
func (p *flexImportRun) readFiles(mr *multipart.Reader, part *multipart.Part) ([]flexpage.File, error) {
	var out []flexpage.File
	for n := 1; ; n++ {
		for part == nil || part.FormName() != "files" {
			var err error
			if part, err = mr.NextPart(); err != nil {
				return out, capErr(err)
			}
		}
		ref := optionledgerjob.FlexFileRef{RequestID: p.id, Ordinal: n}
		if n > flexImportMaxFiles {
			return append(out, p.failFile(ref, flexMoreFiles, "too_many_files")), nil
		}
		f, err := p.importFile(ref, part)
		if errors.Is(err, errBrokenUpload) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, f)
		part = nil
	}
}

// capErr is the error of a failed read of the next part: the cap error, else nil (the post ends with the files
// read so far).
func capErr(err error) error {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return err
	}
	return nil
}

// failFile leaves a failed record for the file ref with the reason code and returns its result.
func (p *flexImportRun) failFile(ref optionledgerjob.FlexFileRef, name, code string) flexpage.File {
	if st, ok := p.writer(); ok {
		if err := optionledgerjob.RecordImportFailure(p.ctx, st, ref, code, p.now); err != nil {
			p.lostRecord = true
		}
	} else {
		code = optionledgerjob.ReasonStoreErr
	}
	p.a.log.Warn("flex import file failed", "component", "ot", "status", code, "file", ref.Ordinal)
	return flexpage.File{Name: name, Reason: code}
}

// readTracker remembers the first read error of the body, so a stream over the cap is not taken for a bad file.
type readTracker struct {
	r   io.Reader
	err error
}

func (t *readTracker) Read(b []byte) (int, error) {
	n, err := t.r.Read(b)
	if err != nil && err != io.EOF && t.err == nil {
		t.err = err
	}
	return n, err
}

// importFile parses the part and imports it; a refused or failed file is a failed record and a result, never an
// error. The error is a stream over the cap or one that broke, which ends the post.
func (p *flexImportRun) importFile(ref optionledgerjob.FlexFileRef, part *multipart.Part) (flexpage.File, error) {
	name := displayName(part.FileName())
	tr := &readTracker{r: part}
	rep, err := flex.Parse(tr)
	// the rest of the part is read before anything is written, so a stream over the cap leaves no record
	_, _ = io.Copy(io.Discard, tr)
	if tr.err != nil {
		if capErr(tr.err) != nil {
			return flexpage.File{}, tr.err
		}
		return flexpage.File{}, errBrokenUpload
	}
	if err != nil {
		return p.failFile(ref, name, parseCode(err)), nil
	}
	st, ok := p.writer()
	if !ok {
		return flexpage.File{Name: name, Reason: optionledgerjob.ReasonStoreErr}, nil
	}
	sum, err := optionledgerjob.ImportFlexReport(p.ctx, rep, p.accounts, st, st, ref, p.now)
	if err != nil {
		var ie *optionledgerjob.ImportError
		code := optionledgerjob.ReasonStoreErr
		if errors.As(err, &ie) {
			code = ie.Code
		}
		return p.failFile(ref, name, code), nil
	}
	p.a.log.Info("flex import file", "component", "ot", "status", fileStatus(sum.Records), "file", ref.Ordinal)
	return resultOf(name, sum), nil
}

// parseCode is the reason code of a refusal of flex.Parse: too_large, bad_gzip, else not_report.
func parseCode(err error) string {
	var fe interface {
		FlexKind() string
		Code() string
	}
	if errors.As(err, &fe) {
		switch {
		case fe.FlexKind() == "too_large":
			return "too_large"
		case fe.Code() == "gzip":
			return "bad_gzip"
		}
	}
	return optionledgerjob.ReasonNotReport
}

// fileStatus is the status of a file by the first that applies to its records, each read by DeriveImportStatus.
func fileStatus(recs []store.FlexFetchRecord) string {
	order := []string{optionledgerjob.ImportFailed, optionledgerjob.ImportStored, optionledgerjob.ImportPresent,
		optionledgerjob.ImportUnconfigured, optionledgerjob.ImportNothingFound}
	best := len(order)
	for _, rec := range recs {
		s, _ := optionledgerjob.DeriveImportStatus(rec)
		for i, o := range order {
			if o == s && i < best {
				best = i
			}
		}
	}
	if best == len(order) {
		return optionledgerjob.ImportNothingFound
	}
	return order[best]
}

// resultOf is the result of an imported file: the counts per alias and the period of its records.
func resultOf(name string, sum optionledgerjob.FlexImportSummary) flexpage.File {
	f := flexpage.File{Name: name, Unmapped: sum.Unmapped}
	for _, c := range sum.Aliases {
		if c.Alias != "-" {
			f.Aliases = append(f.Aliases, flexpage.Alias(c))
		}
	}
	for i, rec := range sum.Records {
		if i == 0 || string(rec.FirstDay) < f.PeriodFirst {
			f.PeriodFirst = string(rec.FirstDay)
		}
		if i == 0 || string(rec.LastDay) > f.PeriodLast {
			f.PeriodLast = string(rec.LastDay)
		}
	}
	return f
}
