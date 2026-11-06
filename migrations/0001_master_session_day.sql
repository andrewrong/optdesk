-- +goose Up
-- master.session_day r1; session_day is the market-zone date, not a UTC date.
CREATE SCHEMA IF NOT EXISTS master;

CREATE TABLE master.session_day (
    calendar_id  text        NOT NULL,
    session_day  date        NOT NULL,
    is_session   boolean     NOT NULL,
    open_at      timestamptz NOT NULL,
    close_at     timestamptz NOT NULL,
    early_close  boolean     NOT NULL,
    PRIMARY KEY (calendar_id, session_day)
);

-- +goose Down
DROP TABLE master.session_day;
