# options_summary v2 fixtures

Invented data only: a Monday (2026-10-26), invented run and build ids. No real position or account.
Written by hand. Validated against `contracts/schemas/page.options_summary.v2.json` by
`internal/contracts/fixtures_test.go`.

## Hand-worked figures of `ok.json`

- Premium series (USD): 5月 +120, 6月 −30, 7月 +200.5, 8月 +80, 9月 +150, 10月 +310. Closed is half
  of each month's premium: +60, −15, +100.25, +40, +75, +155.
- Text is whole dollars, half to even: 200.5 gives `+$200`, 100.25 gives `+$100`, 1250.5 gives `$1,250`.
- Tiles: 3 structures sold in the month; month premium +$310 (equals the 10月 point); open premium
  $1,250.50 shown `$1,250`; max loss $4,000.

## Other files

- `stale.json`: same figures, generated 2026-10-28 after `stale_after`.
- `gap.json`: `open_premium_usd` is a Gap `missing_quote`, `max_loss_usd` is a Gap `unsupported_currency`.
- `bad-version.json`: `schema_version` 1; must fail at `/schema_version` with keyword `const`.
