# option_eod_jp

## Version history

- v1: `option_eod_jp`, `trading_class`, `ledger_dataset` (additive; `optioneod.NewJP`).
- v1: `optioneod.NewJPWithWatch` and the `[stock]` table of `job_jp.toml` (additive).
- v1: `option_eod_jp` is in session: window rule `in_session` with `open_after_minutes` 90, slot `session_open` 90m, a `Run` of a day that is not the current day in `Asia/Tokyo` is `Permanent` ("option_eod: day D is not the current day; a past option chain cannot be collected"), every run has its own `capture_id`, `snapshot_kind` is `session` for `MarketDataType` 1 and `off_session` for 2, 3 or 4; the stop-time key is gone (see `option_eod.md`).
