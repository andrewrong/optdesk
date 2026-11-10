// Package accountcapture is the account_capture job: it reads the account of each configured gateway
// through broker.AccountReader and writes it through store.AccountWriter.
package accountcapture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	datasetID = "account.capture"
	runKey    = "daily"

	statusOK      = "ok"
	statusFailed  = "failed"
	statusPending = "pending"
)

// Config is what New needs. Main string is the alias of Decision 8 and must be a key of Readers.
type Config struct {
	Main    string
	Readers map[string]broker.AccountReader
	Writer  store.AccountWriter
	Clock   clock.Clock
}

type job struct {
	m       manifest
	main    string
	aliases []string
	readers map[string]broker.AccountReader
	w       store.AccountWriter
	clk     clock.Clock
	log     *slog.Logger
}

type typedError struct {
	kind contracts.ErrorKind
	text string
}

func (e *typedError) Error() string             { return e.text }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

// New builds the account_capture job from the embedded job.toml.
func New(cfg Config) (contracts.Job, error) {
	return newJob(cfg, slog.Default())
}

func newJob(cfg Config, log *slog.Logger) (*job, error) {
	if cfg.Writer == nil || cfg.Clock == nil {
		return nil, errors.New("accountcapture: writer and clock are required")
	}
	if _, ok := cfg.Readers[cfg.Main]; !ok {
		return nil, errors.New("accountcapture: main is not a key of readers")
	}
	aliases := make([]string, 0, len(cfg.Readers))
	for a, r := range cfg.Readers {
		if r == nil {
			return nil, errors.New("accountcapture: a reader is nil")
		}
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	m, err := parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	return &job{m: m, main: cfg.Main, aliases: aliases, readers: cfg.Readers, w: cfg.Writer, clk: cfg.Clock, log: log}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Plan expects one key per day; the hash covers the job, the day and the aliases.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: runKey}}
	sum := sha256.Sum256([]byte(strings.Join([]string{j.m.ID, string(day), strings.Join(j.aliases, ",")}, "\n")))
	return keys, hex.EncodeToString(sum[:]), nil
}

// Run captures every alias for every key. Returned counts the aliases tried and Usable those that ended ok.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	var res contracts.JobResult
	for _, k := range keys {
		for _, alias := range j.aliases {
			status := j.captureAlias(ctx, alias, day, k.SubjectKey)
			res.Returned++
			if status == statusOK {
				res.Usable++
				continue
			}
			if alias == j.main {
				if res.Err == nil {
					res.Err = &typedError{kind: contracts.Retryable, text: "account_capture: alias " + alias + " status " + status}
				}
				continue
			}
			j.log.Warn("degraded", "alias", alias, "status", status)
		}
	}
	return res
}

// captureAlias reads and writes one alias and returns its status: ok, pending or failed.
func (j *job) captureAlias(ctx context.Context, alias string, day contracts.Day, key string) string {
	r := j.readers[alias]
	status := statusOK
	skipped := 0
	defer func() {
		if skipped > 0 {
			j.log.Warn("skipped rows of an unsupported sec type", "alias", alias, "count", skipped)
		}
	}()
	acc, err := r.Account(ctx)
	if err != nil {
		j.logFailure(alias, "account", err)
		status = statusFailed
	} else {
		n, err := j.writeAccount(ctx, alias, day, key, acc)
		skipped += n
		if err != nil {
			j.logFailure(alias, "write", err)
			status = statusFailed
		}
	}
	ex, err := r.Executions(ctx)
	if err != nil {
		j.logFailure(alias, "executions", err)
		return statusFailed
	}
	if ex.Collected && len(ex.Executions) > 0 {
		rows, n := toStore(day, ex.Executions)
		skipped += n
		if len(rows) > 0 {
			if err := j.w.WriteExecutions(ctx, alias, rows); err != nil {
				return statusFailed
			}
		}
	}
	if status == statusOK && ex.Collected && ex.Pending > 0 {
		return statusPending
	}
	return status
}

// logFailure logs the alias, the step and the error kind of a failed capture step. It never logs the
// error text, which can carry an account id.
func (j *job) logFailure(alias, step string, err error) {
	kind := "unknown"
	var k interface{ Kind() contracts.ErrorKind }
	if errors.As(err, &k) {
		kind = string(k.Kind())
	}
	j.log.Warn("capture failed", "alias", alias, "step", step, "kind", kind)
}

func (j *job) writeAccount(ctx context.Context, alias string, day contracts.Day, key string, acc broker.Account) (int, error) {
	if !acc.ValuesCollected && !acc.PositionsCollected {
		return 0, nil
	}
	c := store.Capture{
		ID:         alias + ":" + string(day) + ":" + key,
		Alias:      alias,
		At:         j.clk.Now(),
		SessionDay: string(day),
		Status:     statusOK,
	}
	if acc.ValuesCollected {
		c.Values = make([]store.AccountValue, 0, len(acc.Values))
		for _, v := range acc.Values {
			c.Values = append(c.Values, store.AccountValue{Tag: v.Tag, Value: v.Value, Currency: v.Currency})
		}
	}
	skipped := 0
	if acc.PositionsCollected {
		c.Positions = make([]store.Position, 0, len(acc.Positions))
		for _, p := range acc.Positions {
			sp, ok := positionOf(p)
			if !ok {
				skipped++
				continue
			}
			c.Positions = append(c.Positions, sp)
		}
	}
	return skipped, j.w.WriteCapture(ctx, c)
}

// storeSecType maps the broker sec type word (STK, IND, OPT) to the store word; false for any other word.
func storeSecType(s string) (string, bool) {
	switch s {
	case "STK":
		return "stock", true
	case "IND":
		return "index", true
	case "OPT":
		return "option", true
	case "":
		return "", true
	}
	return "", false
}

// storeRight maps the broker right word (P, C) to put or call; any other word gives the empty string.
func storeRight(r string) string {
	switch r {
	case "P":
		return "put"
	case "C":
		return "call"
	}
	return ""
}

// positionOf converts a broker position to a store row; false when its sec type has no store word.
func positionOf(p broker.Position) (store.Position, bool) {
	c := p.Contract
	sec, ok := storeSecType(c.SecType)
	if !ok {
		return store.Position{}, false
	}
	return store.Position{
		Symbol: c.Symbol, SecType: sec, Exchange: c.Exchange, Currency: c.Currency,
		Expiry: isoDay(c.Expiry), Strike: c.Strike, Multiplier: c.Multiplier, Right: storeRight(c.Right),
		ConID: c.ConID, Quantity: p.Quantity, AvgCost: p.AvgCost,
	}, true
}

// isoDay turns the broker's YYYYMMDD into YYYY-MM-DD; anything else is returned as it is.
func isoDay(s string) string {
	if len(s) != 8 {
		return s
	}
	return s[:4] + "-" + s[4:6] + "-" + s[6:]
}

// toStore converts broker executions to store rows and counts those left out for an unsupported sec type.
func toStore(day contracts.Day, in []broker.Execution) ([]store.Execution, int) {
	out := make([]store.Execution, 0, len(in))
	skipped := 0
	for _, e := range in {
		c := e.Contract
		sec, ok := storeSecType(c.SecType)
		if !ok {
			skipped++
			continue
		}
		out = append(out, store.Execution{
			ExecID: e.ExecID, PermID: e.PermID, ExecutedAt: e.Time, SessionDay: string(day),
			Symbol: c.Symbol, SecType: sec, Expiry: isoDay(c.Expiry), Strike: c.Strike, Right: storeRight(c.Right),
			Multiplier: c.Multiplier, Currency: c.Currency, Side: e.Side, Quantity: e.Quantity, Price: e.Price,
			Commission: e.Commission, CommissionCurrency: e.CommissionCurrency,
		})
	}
	return out, skipped
}
