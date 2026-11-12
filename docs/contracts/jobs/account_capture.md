# account_capture

## account_capture

- `Run` reads the live account, which has no history. In development mode (owner decision 2026-11-12) a day other than the date of the clock in `America/New_York` runs with the clock's date as its day, so every row carries today's `session_day`. Test: `internal/jobs/accountcapture/job_test.go` `TestPastDayStoresClockDay`.
