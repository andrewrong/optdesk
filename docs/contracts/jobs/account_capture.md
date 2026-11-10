# account_capture

## account_capture

- `Run` reads the live account, which has no history: a day other than the date of the clock in `America/New_York` returns NoValue "wrong day" and reads and writes nothing, so a backfill of a past day counts the key Absent and never stores today's account under a past `session_day`. Test: `internal/jobs/accountcapture/job_test.go` `TestWrongDay`.
