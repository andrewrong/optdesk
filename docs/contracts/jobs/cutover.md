# cutover

## Cut-over

- `cutover.ImportAccountFunc`, `cutover.VerifyAccountFunc`, `cutover.CompareFiguresFunc` (implemented by `internal/jobs/cutover.ImportAccount`, `VerifyAccount`, `CompareFigures`; design `docs/design/cut-over.md`), with `cutover.VerifyResult`, `cutover.FigureStatus` (`checked`, `explained`, `unexplained`, `new`, `unchecked`), `cutover.FigureResult`, `cutover.FigureReport`; an import reuses `jobs.ImportResult`.
- `--verify` writes nothing: `VerifyAccountFunc` takes a `store.AccountReader`, which has no write method, and reports `Missing` and `Different` only.
- `internal/jobs/cutover` shares `jsonl.go` (JSON Lines reader whose errors carry the line number and never the content; canonical SHA-256 of a line with keys sorted, no space) between the import, the verify and the comparison. It holds no float type.
- Contract tests: `internal/jobs/cutover/types_test.go` `TestImportAccountFuncContract`, `TestVerifyAccountFuncContract`, `TestCompareFiguresFuncContract`; `internal/jobs/cutover/jsonl_test.go` `TestCanonicalHashSortsKeys`.
