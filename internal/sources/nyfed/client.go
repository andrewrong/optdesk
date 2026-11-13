// Package nyfed is the NY Fed source client: a backup for the series SOFR that only fills gaps.
// It needs no key, opens no database and imports no other source package.
package nyfed

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

// DefaultBaseURL is the public host of the NY Fed markets API (read-only, no key).
const DefaultBaseURL = "https://markets.newyorkfed.org"

const (
	maxBody  = 4 << 20
	seriesID = "SOFR"
)

// Config holds the arguments of New. BaseURL empty means DefaultBaseURL; HTTP nil means a client with
// a 30 s timeout; Clock is required (it stamps PublishedAt).
type Config struct {
	BaseURL string
	HTTP    *http.Client
	Clock   clock.Clock
}

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
	base string
	http *http.Client
	clk  clock.Clock
}

// New builds the NY Fed source. No environment is read.
func New(cfg Config) (sources.Source, error) {
	if cfg.Clock == nil {
		return nil, fail(contracts.Permanent, "nyfed: Clock is required")
	}
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	hc := cfg.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &client{base: strings.TrimRight(base, "/"), http: hc, clk: cfg.Clock}, nil
}

// Fetch answers the series SOFR only; any other id is a Permanent error. One request is made when the
// plan names SOFR, and one bad record fails the whole answer with no point.
func (c *client) Fetch(ctx context.Context, plan sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	for _, id := range plan.SeriesIDs {
		if id != seriesID {
			return nil, fail(contracts.Permanent, "nyfed: series %q is not served", id)
		}
	}
	if len(plan.SeriesIDs) == 0 {
		return nil, nil
	}
	q := url.Values{}
	if plan.From != "" {
		q.Set("startDate", string(plan.From))
	}
	if plan.To != "" {
		q.Set("endDate", string(plan.To))
	}
	u := c.base + "/api/rates/secured/sofr/search.json"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fail(contracts.Permanent, "nyfed: bad request")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		te := &typedError{kind: contracts.Retryable, msg: "nyfed: request failed"}
		if ctx.Err() != nil {
			te.msg, te.cause = "nyfed: request cancelled", ctx.Err()
		}
		return nil, te
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, fail(contracts.Retryable, "nyfed: status %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fail(contracts.Permanent, "nyfed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fail(contracts.Retryable, "nyfed: reading response failed")
	}
	pts, err := parse(body, seriesID, plan.From, plan.To, c.clk.Now())
	if err != nil {
		return nil, &typedError{kind: contracts.Permanent, msg: err.Error()}
	}
	return pts, nil
}
