-- +goose Up
-- candidates.option_candidate r1: one row of one candidate run, insert-only (docs/design/option-candidates.md
-- Decisions 1, 9, 13). A rerun is a new run_id and adds rows. The table holds account-derived columns (fits, alias,
-- shares, guard rows): the role ot_w_candidate (made by internal/store/setup) is the only role with a grant on the
-- table (INSERT and SELECT); it also reads the view. ot_ro_pages reads the view and nothing else. ot_runner gets
-- nothing: no job reads candidates through it. No role but the owner may UPDATE, DELETE or TRUNCATE.
CREATE SCHEMA IF NOT EXISTS candidates;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE candidates.option_candidate (
    session_day        date        NOT NULL,
    run_id             text        NOT NULL,
    kind               text        NOT NULL,
    instrument_id      bigint      NOT NULL,
    alias              text        NOT NULL,
    rule_version       text        NOT NULL,
    issuer             text        NOT NULL,
    underlying_id      bigint      NOT NULL,
    expiry             date        NOT NULL,
    dte                integer     NOT NULL,
    option_right       text        NOT NULL,
    strike             text        NOT NULL,
    multiplier         integer     NOT NULL,
    bid_eod            text        NOT NULL,
    ask                text,
    spot               text,
    buy_price          text,
    sale_price         text,
    break_even         text        NOT NULL,
    annualized_yield   text,
    delta              text,
    iv                 text,
    status             text        NOT NULL,
    gates              text        NOT NULL,
    marks              text        NOT NULL,
    fits               text        NOT NULL,
    next_earnings      date,
    earnings_state     text        NOT NULL,
    next_ex_date       date,
    ex_amount          text,
    extrinsic          text,
    rank               integer,
    rank_in_underlying integer,
    computed_at        timestamptz NOT NULL,
    PRIMARY KEY (session_day, run_id, kind, instrument_id, alias),
    CONSTRAINT option_candidate_kind_ck CHECK (kind IN ('short_put', 'covered_call', 'held_call_guard')),
    CONSTRAINT option_candidate_status_ck CHECK (status IN ('ok', 'blocked', 'unknown')),
    CONSTRAINT option_candidate_right_ck CHECK (option_right IN ('put', 'call'))
);

CREATE INDEX option_candidate_day_kind ON candidates.option_candidate (session_day, kind, computed_at DESC);

ALTER SCHEMA candidates OWNER TO ot_owner;
ALTER TABLE candidates.option_candidate OWNER TO ot_owner;

-- Every row passes through, every column named.
CREATE VIEW rd.option_candidate_r1 AS
SELECT session_day, run_id, kind, instrument_id, alias, rule_version, issuer, underlying_id, expiry, dte,
       option_right, strike, multiplier, bid_eod, ask, spot, buy_price, sale_price, break_even, annualized_yield,
       delta, iv, status, gates, marks, fits, next_earnings, earnings_state, next_ex_date, ex_amount, extrinsic,
       rank, rank_in_underlying, computed_at
FROM candidates.option_candidate;

ALTER VIEW rd.option_candidate_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA candidates TO ot_w_candidate;
GRANT INSERT, SELECT ON candidates.option_candidate TO ot_w_candidate;

-- The reader (CandidateReader, ot candidates) and the job read the view only.
GRANT USAGE ON SCHEMA rd TO ot_w_candidate;
GRANT SELECT ON rd.option_candidate_r1 TO ot_w_candidate;

-- Read-only: the pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.option_candidate_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.option_candidate_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.option_candidate_r1 FROM ot_w_candidate;
REVOKE USAGE ON SCHEMA rd FROM ot_w_candidate;
REVOKE INSERT, SELECT ON candidates.option_candidate FROM ot_w_candidate;
REVOKE USAGE ON SCHEMA candidates FROM ot_w_candidate;
DROP VIEW rd.option_candidate_r1;
DROP TABLE candidates.option_candidate;
DROP SCHEMA candidates;
