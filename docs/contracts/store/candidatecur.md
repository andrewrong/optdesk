# store/candidatecur

Package `internal/store/candidatecur` declares the per-currency read and write of the candidate rows
(`docs/design/candidates-jp.md`, Decisions 19 to 21). It holds declarations only; `internal/store/candidate`
implements it on its `Store`, through `rd.option_candidate_r1` and `candidates.option_candidate`.

## Currency

`candidates.option_candidate` has a nullable `currency` (migration `0061_candidate_currency.sql`), appended last to
the table and the view. A null reads as USD: no existing row changes. A currency is three upper-case letters.

## Reads

`store.CandidateReader.Candidates` is USD-only: it returns the newest run of the day among rows whose currency is
null or USD. Its signature and meaning are unchanged.

`Reader.CandidatesIn(day, kind, currency)` returns the newest run of that currency: the rows of one currency are
chosen first and the latest `ComputedAt` among them picks the run. A day holding a US run at 22:58 and a JP run at
08:10 gives each from its own call, so the rank sequences of two currencies never mix in one result. Currency USD
includes the null rows, so `CandidatesIn(..., "USD")` equals `Candidates`.

## Writes

`Writer.InsertCandidatesIn(rows)` inserts like `InsertCandidates` (insert-only, a key that exists is skipped) and
stores each `Row.Currency`. `InsertCandidates` still writes null for `currency`.
