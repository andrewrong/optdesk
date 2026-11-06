# Carry-over notes

Source: option-trader at commit `b00b4ed`. Copy allow-list (path globs copied): `GLOSSARY.md`, `docs/engineering.md`,
`docs/adr/0003-*.md`, `docs/adr/0004-*.md`, `docs/adr/0010-*.md`, `src/option_trader/conventions/register.toml`.
The manifest `docs/carryover-manifest.txt` records the source hash of each file. The destinations are not byte copies.
Every file went through the same mechanical pass, then a read for personal sentences; each change is listed by section below,
not quoted.

The mechanical pass, applied to all six files: every reference to an issue or pull request number of the private repository was
removed or restated without the number (the sentence keeps its rule and its date); no link to the private repository remains.

## docs/GLOSSARY.md

- Issue references removed from the entries of the market-weather, IV, benchmark, advice, offer, JP-rate, sizing, spread, pool,
  follow-list, valuation, calendar, freshness, reading and sector sections; "since issue N" phrases restated as "since then".
- The pool-file and follow-list entries named the owner's pronoun and a personal export habit; restated neutrally
  (the owner's local file, a text file exported from the owner's IBKR watchlist or written by hand).
- Command names and the `~/.config/option-trader/` file location are kept as rules of the tool; the new name is a later decision.

## docs/engineering.md

- One issue reference (why the whole suite is not run locally) removed.
- The link to `adding-a-source.md` pointed at a file that is not carried over; restated as a checklist not carried over yet.
- The Python-specific gates, `CLAUDE.md` and the `AO review` step describe the old repository's workflow; kept as the
  reasoning of that repository and to be restated for optdesk by a later issue.

## docs/adr/0003-market-data-architecture.md

- Issue references removed from every dated note (pacer, contract-detail refresh, answer cache, snapshot plans v2 to v4);
  cross-references between notes restated by date or as "the earlier" note.

## docs/adr/0004-markets-calendars-fx.md

- Issue references removed from the dated notes on the after-close slot, the US standard series, the unified serve decision,
  the event-loop change and the Read-Only check notes; references between these notes restated by date.
- The link to a tracking issue of the private repository replaced by "a follow-up".

## docs/adr/0010-reports-and-delivery.md

- Issue references removed from the dated notes; the reference to an earlier note restated as "the earlier note".

## docs/conventions/register.toml

- `agreed_in` held an issue number of the private repository in every entry; all values are now
  "carried over from option-trader".
- History lines and descriptions that cited an issue number were restated without it.
- `source_hash` held a hash of old source code that does not exist here, and some hashes contained long digit runs that the
  de-personalisation scan reads as ids; every value is now `pending-recompute`, to be set when the source is rewritten.
- The file is 1509 lines, over the 600 line limit for a new file; it is a data file copied whole and is not split here.

**Second pass, all copied files**

- Statements of where the owner lives, the named city and its zone name, and two stock codes the owner trades were removed or
  restated as "the owner's configured local zone" and a generic example code, in the glossary entries on FX and live
  observation, ADR 0004 (context and watchlist sections), ADR 0010 (ledger page note) and four register entries.
