// Package jobrequestpg implements the three interfaces of internal/store/jobrequest on PostgreSQL: the request queue
// of the job console, two tables in schema settings read through the view rd.job_request_r1
// (docs/design/job-console.md Decisions 2, 3 and 9). The role of the DSN decides what the opened store may do:
// the pages role reads, the web role posts and withdraws, the runner role claims and finishes.
package jobrequestpg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

var (
	_ jobrequest.OpenReaderFunc   = OpenReader
	_ jobrequest.OpenPosterFunc   = OpenPoster
	_ jobrequest.OpenConsumerFunc = OpenConsumer
	_ jobrequest.Poster           = (*Store)(nil)
	_ jobrequest.Consumer         = (*Store)(nil)
)

// OpenReader connects with the DSN of the pages role and returns a Reader.
func OpenReader(ctx context.Context, dsn string) (jobrequest.Reader, error) { return open(ctx, dsn) }

// OpenPoster connects with the DSN of the web role and returns a Poster.
func OpenPoster(ctx context.Context, dsn string) (jobrequest.Poster, error) { return open(ctx, dsn) }

// OpenConsumer connects with the DSN of the runner role and returns a Consumer.
func OpenConsumer(ctx context.Context, dsn string) (jobrequest.Consumer, error) {
	return open(ctx, dsn)
}

// open connects and pings; it applies no migration, the schema is made by goose with the role ot_owner.
func open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("jobrequestpg: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("jobrequestpg: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}
