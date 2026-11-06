// Package fred is the FRED source client: it fetches series observations over net/http and maps
// them to contracts.SeriesPoint. It opens no database and knows neither runner nor pages.
package fred

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const maxBody = 16 << 20

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
	key    string
	clk    clock.Clock
	pacer  *pacer
	client *http.Client
}

var _ sources.NewFREDFunc = New

// New builds the FRED source. BaseURL, APIKey and Clock are required; no environment is read.
func New(cfg sources.FREDConfig) (sources.Source, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Clock == nil {
		return nil, fail(contracts.Permanent, "fred: BaseURL, APIKey and Clock are required")
	}
	return &client{
		base:   strings.TrimRight(cfg.BaseURL, "/"),
		key:    cfg.APIKey,
		clk:    cfg.Clock,
		pacer:  newPacer(cfg.Clock),
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Fetch returns the points of every series of the plan, one paced request per series.
func (c *client) Fetch(ctx context.Context, plan sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, id := range plan.SeriesIDs {
		pts, err := c.fetchOne(ctx, id, plan)
		if err != nil {
			return nil, err
		}
		out = append(out, pts...)
	}
	return out, nil
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

func (c *client) fetchOne(ctx context.Context, id string, plan sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	if err := c.wait(ctx); err != nil {
		return nil, &typedError{kind: contracts.Retryable, msg: "fred: request cancelled", cause: err}
	}
	q := url.Values{}
	q.Set("series_id", id)
	q.Set("file_type", "json")
	q.Set("api_key", c.key)
	if plan.From != "" {
		q.Set("observation_start", string(plan.From))
	}
	if plan.To != "" {
		q.Set("observation_end", string(plan.To))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/fred/series/observations?"+q.Encode(), nil)
	if err != nil {
		return nil, fail(contracts.Permanent, "fred: bad request")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		// The error of net/http carries the URL, and with it the key: report no text of it.
		te := &typedError{kind: contracts.Retryable, msg: "fred: request failed"}
		if ctx.Err() != nil {
			te.msg, te.cause = "fred: request cancelled", ctx.Err()
		}
		return nil, te
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, fail(contracts.Retryable, "fred: status %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fail(contracts.Permanent, "fred: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fail(contracts.Retryable, "fred: reading response failed")
	}
	pts, err := mapObservations(id, body, c.clk.Now())
	if err != nil {
		return nil, &typedError{kind: contracts.Permanent, msg: err.Error()}
	}
	return pts, nil
}
