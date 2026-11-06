# macro_summary fixtures

Invented data only: a Monday (2026-10-26), an invented run id and times. No real reading, position
or account. The files are written by `scripts/gen_viewmodel_schema.py` (`pnpm gen:viewmodels`); CI
fails on any difference. This README is hand-written.

## Hand-worked figures of `ok.json`

- Ease 56 against 56.2 seven days earlier: 56 − 56.2 = −0.2 index points, text `−0.2` (U+2212). The
  marker sits at 56 % of the 0 to 100 track, the neutral band at 34 % to 66 %.
- Say: `松紧度中性，较上周 −0.2。本周的事件是 CPI。`
- Events: 美国 CPI on 2026-10-28 and 美国非农就业 on 2026-11-06. No 前值 is stored, so each `previous`
  is a named gap (`no_data`), not a number.

## Other files

- `stale.json`: same figures, generated two days later, after `stale_after`.
- `gap.json`: no stored ease, so `card.ease` is a named gap (`no_data`), never a zero.
- `bad-version.json` (`schema_version: 2`), `bad-nan.json` (a `value` written as the string `"NaN"`),
  `bad-extra-field.json` (an unknown field in `card`), `bad-missing-unit.json` (no `unit` in a figure),
  `bad-marker-range.json` (the marker beyond 100 % of the track): each must fail validation.
