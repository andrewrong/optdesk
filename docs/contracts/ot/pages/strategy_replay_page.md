# strategy_replay_page

Design: `docs/design/strategy-replay-page.md#3`. Schema `page.strategy_replay_page.v1`.

- Route: `/strategy-replay` (page family `strategy_replay_page`, order 136, between `base_data_page` and the valuation
  pages). A GET answers the view `form` with the defaults. When the owner token or its settings line is missing the
  page carries the notice `form_off` and the post route is not served.
- Post path: `/strategy-replay-post`, an owner post. The guard is `guardOwnerPost` (urlencoded body under 64 KiB, the
  owner cookie, `ParseForm`); a method other than POST is 405. The fields are `lots`, `alias`, `as_of`, `early_keep`,
  `early_dte` and `min_lots`. The answer is the whole page (`pages.NewHandlerFunc`), the form with the values sent and
  the view `report`. Nothing is written or stored.
- One function: the post runs `replayRun`, the function `ot strategy-replay` runs, and copies the `replayapi.Report`
  into the page input field by field.
- Cap: one run has `replayPostTimeout` (60 s [T]) on the request context; a run over it answers the form with the
  notice `timed_out`. Only one run at a time: a second post gets the notice `busy`; the lock is released when the
  goroutine that runs `replayRun` returns, not when `timed_out` was answered.
- Bad field: 422 with the notice of the field (`bad_lots`, `bad_as_of`, `bad_early_keep`, `bad_early_dte`,
  `bad_min_lots`); every other answer is 200. A failed read (a missing line, an unwired opener, a failed open) is the
  notice `read_failed`; the log names the line, never a DSN.
- The four lines read through the env file of `ot serve`: `OT_PG_DSN_OPTION_LEDGER`, `OT_PG_DSN_PAGES`,
  `OT_PG_DSN_RUNNER`, `OT_PG_DSN_BAR`.
- Restart note: no report is stored; a run cut by a restart of `ot serve` leaves nothing behind, and the owner presses
  the button again.
