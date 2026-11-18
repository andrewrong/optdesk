# Valuation import (`internal/compute/valueimportapi`)

Source: `docs/design/valuation-import.md`, entry 1.

**Pure, decimal text.** `Importer.Plan` takes the bytes of the file and the stored state and returns a plan; it has no clock, no I/O and no store type. Numbers are read from the JSON as decimal text and never through a float, so `0.20` stays `0.20`; the importer computes no price itself and asks the `Calculator` in `Input`. Unknown keys of the file are ignored.

**The key.** The key of a company is found by matching its ticker to a current `settings.underlying` row (upper-cased, exchange suffix cut), never from the file's `market`. A ticker with no match is unmatched: the plan proposes a key and the owner's pick (`Input.Picks`, by ticker) decides the key and the R market; an unmatched name is written only when the owner ticks it.

**Older never overwrites newer.** A file never overwrites a newer stored valuation, R or margin table: a row that differs and whose version date is not after the stored one, or an R or margin table whose stored `effective_date` is not before the file's `as_of`, has action `older`, is listed and is not written. Equal rows are `same`.

**Tolerance of the buy-zone check.** The plan computes the site's buy-zone top from the file's V and P and compares it with the file's own. A difference of at most `RoundTolerance` (relative, `0.005`) or `RoundToleranceAbs` (absolute, `0.5`), whichever is larger, is rounding; more refuses the row, with both numbers in `FileBuyTop` and `SiteBuyTop`.

**Bad file.** `ErrBadFile` is returned as a `*FileError` that names the path of the key that is missing, of a wrong type or out of range (more than `MaxCompanies` companies, a file over `MaxFileBytes`), and never a value of the file; `errors.Is(err, ErrBadFile)` is true.
