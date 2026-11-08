# Engineering principles

How code gets into this repository. `AGENTS.md` holds the short version for agents; this is the reasoning behind it.

## 1. Libraries do the maths

- Numerics come from gonum (statistics, linear algebra) and go-talib (indicators), each behind an interface of `internal/compute`; own glue code, even thin, sits behind an interface too, so a library can replace it. Nothing outside `internal/compute` imports a library directly.
- Implied volatility comes only from IBKR model values (`iv_decimal`), with one exception: the own IV30 of `GLOSSARY.md` (`market.iv30_own`, convention `iv30_v1`), which joins IBKR model IVs by the glossary's rule. We do not solve IV ourselves and do not price options to replace a broker value.
- Figures are stored as readings (`readings.reading`), each with the hash of the rule that made it (`reading_input.code_version`) and the hash of its input rows (`rows_sha256`), insert-only. A value is the decimal text of `Units.RoundHalfEven`; a missing figure is stored as missing with a reason code, never as a zero.
- Hand-written formulas are limited to simple arithmetic defined in `GLOSSARY.md` and in the figures register (yields, cost basis, HV, ratios, percentile ranks). Each cites its definition.
- Greeks and a forecast of volatility are not computed here until a design adds them; a Greek is never returned as zero or as a European stand-in.
- A new dependency needs a reason in the issue. `uv.lock` pins every version.

## 2. One vocabulary

`GLOSSARY.md` fixes units and definitions. Convert library or broker units at the boundary and document it. Unit mistakes are the most likely silent bug in this project.

## 3. Tests prove behaviour against independent truth

- Expected values come from an independent source: a published worked example, a second library, or a hand-checked literal. Never recompute the expected value the way the code does (tautological test).
- Pricing code also gets property tests (`hypothesis`). Each invariant states its conditions:
  - put-call parity: European, same inputs;
  - no-arbitrage price bounds for the exercise style and dividends used;
  - price increases with volatility;
  - American prices do not decrease with time to expiry (European puts may);
  - delta within [0, 1] for calls and [−1, 0] for puts.
- Numerical methods check convergence (for example, finite-difference grid refinement).
- Tests never touch the network or a broker. Broker responses are recorded fixtures. Live read-only smoke checks run only when the owner triggers them.
- Coverage of `src/option_trader` must stay at or above 90%. The nightly CI run enforces it.
- Test files that drive a whole page build or a whole command opt in to `database_kept_open` (`pytestmark = pytest.mark.usefixtures("database_kept_open")`) so that the database file is loaded once per test; a file that opens the same database read-only, and `tests/macro/`, do not.

## 4. Architecture

- **Core**: pure functions on typed values, no I/O. Pricing, volatility, yields, stress tests.
- **Adapters**: IBKR (read-only), SQLite storage, file input. Validate everything that enters, reject bad quotes.
- **Reports**: assemble core results for display.

Core never imports adapters. A new source follows an adding-a-source checklist (not carried over yet).

## 5. Small, specified changes

- One issue = one PR = one behaviour, ideally under about 300 changed lines.
- Every issue states acceptance criteria, the seams to test, and the reference values with their sources. Workers follow them and do not stop to ask about test seams.
- Significant design decisions get a short ADR in `docs/adr/`.

## 6. Gates before merge

1. Local: `ruff check`, `ruff format --check`, `mypy` (strict), the tests of the touched files and the quick selection of step 2. The whole suite is not run locally: it is step 3.
2. CI on every push to a PR (job `quick`): `ruff check`, `ruff format --check`, `mypy` and the tests of the changed parts, chosen by `scripts/ci_quick_tests.py` from the changed paths: a static scan for the tests that import each changed module, which may miss transitive effects. It is a first signal, not the gate.
3. CI before merge (job `full`): all checks and every test without coverage, in two test processes. It runs when the label `full-tests` is added to the PR, on the commit that is to merge, and on every push to `main`. The commit status `full-tests` stays pending until this run has passed on the same commit.
4. AO review (Claude) against the checklist in `CLAUDE.md`.
5. The owner merges.

At the end of each milestone the advisor reviews the whole milestone, with a second model's opinion on design questions.

## 7. Trading safety

The tool advises; it never trades. See the "Trading safety" section of `AGENTS.md`. The IB Gateway connection used by this tool must have "Read-Only API" enabled.
