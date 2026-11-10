// Package sec is the SEC source client: it fetches the companyfacts and submissions documents of a
// CIK over net/http and maps them to contracts.FundamentalFact. It opens no database and reads no
// environment.
package sec

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const maxBody = 64 << 20

var subjectRE = regexp.MustCompile(`^cik:[0-9]{10}$`)

type typedError struct {
	kind  contracts.ErrorKind
	msg   string
	cause error
}

func (e *typedError) Error() string             { return e.msg }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }
func (e *typedError) Unwrap() error             { return e.cause }

func fail(kind contracts.ErrorKind, format string, a ...any) error {
	return &typedError{kind: kind, msg: fmt.Sprintf(format, a...)}
}

type client struct {
	base   string
	agent  string
	clk    clock.Clock
	pacer  *pacer
	client *http.Client
}

var _ sources.NewSECFunc = New

// New builds the SEC source. BaseURL, UserAgent and Clock are required.
func New(cfg sources.SECConfig) (sources.FactSource, error) {
	if cfg.BaseURL == "" || cfg.UserAgent == "" || cfg.Clock == nil {
		return nil, fail(contracts.Permanent, "sec: BaseURL, UserAgent and Clock are required")
	}
	return &client{
		base:   strings.TrimRight(cfg.BaseURL, "/"),
		agent:  cfg.UserAgent,
		clk:    cfg.Clock,
		pacer:  newPacer(cfg.Clock),
		client: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Fetch returns the facts of every subject of the plan: the companyfacts concepts filed within the
// plan's range and the SIC code. Two paced requests per subject.
func (c *client) Fetch(ctx context.Context, plan sources.FactPlan) ([]contracts.FundamentalFact, error) {
	var out []contracts.FundamentalFact
	for _, subject := range plan.Subjects {
		if !subjectRE.MatchString(subject) {
			return nil, fail(contracts.Permanent, "sec: subject %q is not cik:<10 digits>", subject)
		}
		cik := strings.TrimPrefix(subject, "cik:")
		facts, err := c.get(ctx, "/api/xbrl/companyfacts/CIK"+cik+".json")
		if err != nil {
			return nil, err
		}
		mapped, err := mapCompanyFacts(subject, facts.body, facts.at, plan.From, plan.To)
		if err != nil {
			return nil, &typedError{kind: contracts.Permanent, msg: err.Error()}
		}
		out = append(out, mapped...)
		subs, err := c.get(ctx, "/submissions/CIK"+cik+".json")
		if err != nil {
			return nil, err
		}
		sic, err := mapSIC(subject, subs.body, subs.at)
		if err != nil {
			return nil, &typedError{kind: contracts.Permanent, msg: err.Error()}
		}
		out = append(out, sic...)
	}
	return out, nil
}

type document struct {
	body []byte
	at   time.Time
}

func (c *client) wait(ctx context.Context) error {
	d := c.pacer.reserve()
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *client) get(ctx context.Context, path string) (document, error) {
	if err := c.wait(ctx); err != nil {
		return document{}, &typedError{kind: contracts.Retryable, msg: "sec: request cancelled", cause: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return document{}, fail(contracts.Permanent, "sec: bad request")
	}
	req.Header.Set("User-Agent", c.agent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		// Report no text of the net/http error.
		te := &typedError{kind: contracts.Retryable, msg: "sec: request failed"}
		if ctx.Err() != nil {
			te.msg, te.cause = "sec: request cancelled", ctx.Err()
		}
		return document{}, te
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return document{}, fail(contracts.Retryable, "sec: status %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return document{}, fail(contracts.Permanent, "sec: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return document{}, fail(contracts.Retryable, "sec: reading response failed")
	}
	return document{body: body, at: c.clk.Now().UTC()}, nil
}
