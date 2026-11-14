package macroevents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const maxBody = 16 << 20

type typedError struct {
	kind contracts.ErrorKind
	msg  string
}

func (e *typedError) Error() string             { return e.msg }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func fail(kind contracts.ErrorKind, format string, a ...any) error {
	return &typedError{kind: kind, msg: fmt.Sprintf(format, a...)}
}

type releaseClient struct {
	base, key string
	client    *http.Client
}

// NewRelease builds the ReleaseSource over net/http: own implementation, because the repository has no FRED
// release-dates client and nothing else uses one. baseURL and key are required.
func NewRelease(baseURL, key string) (ReleaseSource, error) {
	if baseURL == "" || key == "" {
		return nil, fail(contracts.Permanent, "macroevents: base URL and key are required")
	}
	return &releaseClient{base: strings.TrimRight(baseURL, "/"), key: key, client: &http.Client{Timeout: 30 * time.Second}}, nil
}

// Dates implements ReleaseSource: one GET of fred/release/dates.
func (c *releaseClient) Dates(ctx context.Context, releaseID string, from contracts.Day) ([]contracts.Day, error) {
	q := url.Values{}
	q.Set("release_id", releaseID)
	q.Set("realtime_start", string(from))
	q.Set("include_release_dates_with_no_data", "true")
	q.Set("sort_order", "asc")
	q.Set("file_type", "json")
	q.Set("api_key", c.key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/fred/release/dates?"+q.Encode(), nil)
	if err != nil {
		return nil, fail(contracts.Permanent, "macroevents: bad request")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		// The error of net/http carries the URL, and with it the key: report no text of it.
		return nil, fail(contracts.Retryable, "macroevents: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, fail(contracts.Retryable, "macroevents: status %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fail(contracts.Permanent, "macroevents: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fail(contracts.Retryable, "macroevents: reading response failed")
	}
	var doc struct {
		ReleaseDates []struct {
			Date string `json:"date"`
		} `json:"release_dates"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fail(contracts.Permanent, "macroevents: response is not the release dates")
	}
	var out []contracts.Day
	for _, r := range doc.ReleaseDates {
		if _, err := time.Parse("2006-01-02", r.Date); err != nil {
			return nil, fail(contracts.Permanent, "macroevents: a release date is not YYYY-MM-DD")
		}
		if r.Date < string(from) {
			continue
		}
		out = append(out, contracts.Day(r.Date))
	}
	return out, nil
}
