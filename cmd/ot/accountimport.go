package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/cutover"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("account-import", accountImport) }

// accountFile is the name of the export inside DIR.
const accountFile = "account.jsonl"

// accountImportReport is the file of --report: counts, the hash of the input file and ok; no row.
type accountImportReport struct {
	Dataset    string         `json:"dataset"`
	FileSHA256 string         `json:"file_sha256"`
	Counts     map[string]int `json:"counts"`
	OK         bool           `json:"ok"`
}

// accountImport is `ot account-import DIR [--map from=to ...] --through DAY [--verify] [--report FILE]`.
// It prints counts only and returns exit code 1 when a verify has Missing or Different above 0 or an
// import has Conflict above 0. With --verify it calls VerifyAccount only and writes nothing.
func accountImport(a *app, args []string) error {
	o, err := parseAccountImport(args)
	if err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return errors.New("account-import needs a real profile; the demo profile opens no store")
	}
	book, err := a.loadBook()
	if err != nil {
		return err
	}
	if err := checkAliasMap(book, o.aliasMap); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(o.dir, accountFile))
	if err != nil {
		return fmt.Errorf("cannot read %s in DIR", accountFile)
	}
	sum := sha256.Sum256(data)
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn := env[store.DSNEnvAccount]
	if dsn == "" {
		return fmt.Errorf("%s is not set", store.DSNEnvAccount)
	}
	ctx := context.Background()
	st, err := a.deps.openAccount(ctx, dsn)
	if err != nil {
		// The text of an open error may hold the DSN: it is withheld.
		return fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
	}
	defer func() { _ = st.Close() }()

	counts := map[string]int{}
	bad := false
	if o.verify {
		res, err := cutover.VerifyAccount(ctx, bytes.NewReader(data), st, o.aliasMap, o.day)
		if err != nil {
			return fmt.Errorf("account-import verify: %w", err)
		}
		counts["lines"], counts["missing"], counts["different"] = res.Lines, res.Missing, res.Different
		bad = res.Missing > 0 || res.Different > 0
		_, _ = fmt.Fprintf(a.stdout, "account-import verify lines=%d missing=%d different=%d\n", res.Lines, res.Missing, res.Different)
	} else {
		res, err := cutover.ImportAccount(ctx, bytes.NewReader(data), st, o.aliasMap, o.day)
		if err != nil {
			return fmt.Errorf("account-import: %w", err)
		}
		counts["lines"], counts["inserted"], counts["same"], counts["conflict"] = res.Lines, res.Inserted, res.Same, res.Conflict
		bad = res.Conflict > 0
		_, _ = fmt.Fprintf(a.stdout, "account-import lines=%d inserted=%d same=%d conflict=%d\n", res.Lines, res.Inserted, res.Same, res.Conflict)
	}
	if o.report != "" {
		rep := accountImportReport{Dataset: "account", FileSHA256: hex.EncodeToString(sum[:]), Counts: counts, OK: !bad}
		raw, err := json.Marshal(rep)
		if err != nil {
			return errors.New("cannot encode the report")
		}
		if err := os.WriteFile(o.report, append(raw, '\n'), 0o644); err != nil { //nolint:gosec // a report holds counts only
			return errors.New("cannot write the report file")
		}
	}
	if bad {
		return &exitError{code: 1, err: errors.New("account-import: the file does not match the store (see the counts)")}
	}
	return nil
}

// parseAliasMaps turns the --map values into an alias map; none gives nil (every alias keeps its label).
func parseAliasMaps(maps []string) (map[string]string, error) {
	if len(maps) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(maps))
	for _, s := range maps {
		from, to, ok := strings.Cut(s, "=")
		if !ok || from == "" || to == "" {
			return nil, errors.New("--map wants from=to")
		}
		m[from] = to
	}
	return m, nil
}

// accountImportOpts are the parsed arguments of ot account-import.
type accountImportOpts struct {
	dir      string
	day      contracts.Day
	aliasMap map[string]string
	verify   bool
	report   string
}

// parseAccountImport reads DIR (first or last argument) and the flags.
func parseAccountImport(args []string) (accountImportOpts, error) {
	var o accountImportOpts
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		o.dir, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("account-import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var maps stringList
	fs.Var(&maps, "map", "alias map from=to (repeatable)")
	through := fs.String("through", "", "last complete session day YYYY-MM-DD (required)")
	fs.BoolVar(&o.verify, "verify", false, "compare the file with the store and write nothing")
	fs.StringVar(&o.report, "report", "", "write counts, the file hash and ok to this file")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if o.dir == "" && fs.NArg() == 1 {
		o.dir = fs.Arg(0)
	} else if fs.NArg() != 0 {
		return o, errors.New("usage: ot account-import DIR [--map from=to ...] --through DAY [--verify] [--report FILE]")
	}
	if o.dir == "" {
		return o, errors.New("account-import needs DIR")
	}
	if *through == "" {
		return o, errors.New("--through is required")
	}
	t, err := time.Parse(dayLayout, *through)
	if err != nil {
		return o, fmt.Errorf("--through %q is not YYYY-MM-DD", *through)
	}
	o.day = contracts.Day(t.Format(dayLayout))
	o.aliasMap, err = parseAliasMaps(maps)
	return o, err
}
