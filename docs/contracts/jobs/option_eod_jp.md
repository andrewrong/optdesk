# option_eod_jp

## Version history

- v1: `option_eod_jp`, `trading_class`, `ledger_dataset` (additive; `optioneod.NewJP`).
- v1: `optioneod.NewJPWithWatch` and the `[stock]` table of `job_jp.toml` (additive).
- v1: `option_eod_jp` is in session: window rule `in_session` with `open_after_minutes` 90, slot `session_open` 90m, a `Run` of a day that is not the current day in `Asia/Tokyo` is `Permanent` ("option_eod: day D is not the current day; a past option chain cannot be collected"), every run has its own `capture_id`, `snapshot_kind` is `session` for `MarketDataType` 1 and `off_session` for 2, 3 or 4; the stop-time key is gone (see `option_eod.md`).

A run before the slot of its day (`1030` Tokyo for `option_eod_jp`, `1100` New York for `option_eod_index`) is not refused: it stores its rows under its own capture id (`captureID` with the slot argument `early` and the run's start in UTC) and ends `Retryable` with the text `option_eod: before_slot: ...`, `Returned` and `Usable` as counted; the key stays open and the scheduled run of the day still runs. Readers get the scheduled rows because they pick the newest session row (`optionEodBetter`); limit, accepted: an early row taken with a live quote beats a scheduled row taken with a frozen or delayed quote. The behaviour is that of `option_eod` (`TestSlotInstantIsNotEarly`).
